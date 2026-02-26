/*
Copyright 2026 Shane Utt.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GatewayProxy manages a port-forward to a Gateway and provides HTTP
// assertion helpers for testing WAF behavior.
type GatewayProxy struct {
	s         *Scenario
	namespace string
	gateway   string
	localPort string
	baseURL   string
	httpc     *http.Client

	mu     sync.Mutex
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// ProxyToGateway sets up a kubectl port-forward to the named Gateway's pod
// and returns a GatewayProxy for making HTTP requests. The port-forward is
// automatically cleaned up when the scenario ends.
func (s *Scenario) ProxyToGateway(namespace, gatewayName string) *GatewayProxy {
	s.T.Helper()
	port := AllocatePort()
	ctx, cancel := context.WithCancel(context.Background())

	proxy := &GatewayProxy{
		s:         s,
		namespace: namespace,
		gateway:   gatewayName,
		localPort: port,
		baseURL:   fmt.Sprintf("http://localhost:%s", port),
		httpc:     &http.Client{Timeout: 10 * time.Second},
		cancel:    cancel,
	}

	go proxy.maintain(ctx)

	// Wait for the port-forward to accept connections.
	require.Eventually(s.T, func() bool {
		resp, err := proxy.httpc.Get(proxy.baseURL)
		if err != nil {
			return false
		}
		defer func() {
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
		}()
		return true
	}, DefaultTimeout, time.Second,
		"port-forward to %s/%s (localhost:%s) not ready", namespace, gatewayName, port,
	)

	s.OnCleanup(func() {
		cancel()
		proxy.mu.Lock()
		defer proxy.mu.Unlock()
		if proxy.cmd != nil && proxy.cmd.Process != nil {
			_ = proxy.cmd.Process.Kill()
			_ = proxy.cmd.Wait()
		}
	})

	s.T.Logf("Port-forwarding %s/%s -> localhost:%s", namespace, gatewayName, port)
	return proxy
}

// URL returns the full URL for a given path through the proxy.
func (g *GatewayProxy) URL(path string) string {
	return g.baseURL + path
}

// Get makes a GET request through the proxy and returns the result.
func (g *GatewayProxy) Get(path string) *HTTPResult {
	resp, err := g.httpc.Get(g.URL(path))
	if err != nil {
		return &HTTPResult{Err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return &HTTPResult{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       body,
	}
}

// ExpectBlocked polls until the given path returns HTTP 403 (blocked by WAF).
func (g *GatewayProxy) ExpectBlocked(path string) {
	g.s.T.Helper()
	g.ExpectStatus(path, http.StatusForbidden)
}

// ExpectAllowed polls until the given path returns a status that is NOT 403.
// The actual status depends on the backend (typically 404 when no route is
// configured, 200 when a backend exists).
func (g *GatewayProxy) ExpectAllowed(path string) {
	g.s.T.Helper()
	var lastStatus string
	require.Eventually(g.s.T, func() bool {
		resp, err := g.httpc.Get(g.URL(path))
		if err != nil {
			lastStatus = fmt.Sprintf("error: %v", err)
			return false
		}
		defer func() {
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
		}()
		lastStatus = fmt.Sprintf("%d", resp.StatusCode)
		return resp.StatusCode != http.StatusForbidden
	}, DefaultTimeout, DefaultInterval,
		"expected %s to not be blocked (not 403), last status: %s", path, lastStatus)
}

// ExpectStatus polls until the given path returns the expected HTTP status.
func (g *GatewayProxy) ExpectStatus(path string, code int) {
	g.s.T.Helper()
	var lastStatus string
	require.Eventually(g.s.T, func() bool {
		resp, err := g.httpc.Get(g.URL(path))
		if err != nil {
			lastStatus = fmt.Sprintf("error: %v", err)
			return false
		}
		defer func() {
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
		}()
		lastStatus = fmt.Sprintf("%d", resp.StatusCode)
		return resp.StatusCode == code
	}, DefaultTimeout, DefaultInterval,
		"expected %s to return %d, last status: %s", path, code, lastStatus)
}

// HTTPResult holds the result of an HTTP request.
type HTTPResult struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	Err        error
}

// -----------------------------------------------------------------------------
// Port Forward Management
// -----------------------------------------------------------------------------

func (g *GatewayProxy) maintain(ctx context.Context) {
	backoff := time.Second
	const maxBackoff = 10 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := g.runPortForward(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			g.s.T.Logf("port-forward %s/%s restarting (backoff %s): %v",
				g.namespace, g.gateway, backoff, err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff = min(backoff*2, maxBackoff)
	}
}

func (g *GatewayProxy) runPortForward(ctx context.Context) error {
	labelSelector := fmt.Sprintf(
		"gateway.networking.k8s.io/gateway-name=%s", g.gateway,
	)

	pods, err := g.s.F.KubeClient.CoreV1().Pods(g.namespace).List(
		ctx,
		metav1.ListOptions{LabelSelector: labelSelector},
	)
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods matching %s", labelSelector)
	}

	podName := pods.Items[0].Name
	cmd := exec.CommandContext(ctx, "kubectl", g.s.F.kubectlArgs(
		g.namespace, "port-forward", podName,
		fmt.Sprintf("%s:80", g.localPort),
	)...)

	g.mu.Lock()
	g.cmd = cmd
	g.mu.Unlock()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start port-forward: %w", err)
	}
	// Reset backoff on successful start (connection established)
	return cmd.Wait()
}
