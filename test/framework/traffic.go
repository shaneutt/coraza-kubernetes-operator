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
	"fmt"
	"io"
	"net/http"
	"os/exec"
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
	cmd       *exec.Cmd
	stopCh    chan struct{}
}

// ProxyToGateway sets up a kubectl port-forward to the named Gateway's pod
// and returns a GatewayProxy for making HTTP requests. The port-forward is
// automatically cleaned up when the scenario ends.
func (s *Scenario) ProxyToGateway(namespace, gatewayName string) *GatewayProxy {
	s.T.Helper()
	port := AllocatePort()

	proxy := &GatewayProxy{
		s:         s,
		namespace: namespace,
		gateway:   gatewayName,
		localPort: port,
		baseURL:   fmt.Sprintf("http://localhost:%s", port),
		httpc:     &http.Client{Timeout: 10 * time.Second},
		stopCh:    make(chan struct{}),
	}

	// Start a goroutine that maintains the port-forward connection,
	// restarting it if the process exits.
	go proxy.maintain()

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
		close(proxy.stopCh)
		time.Sleep(1 * time.Second)
		if proxy.cmd != nil && proxy.cmd.Process != nil {
			_ = proxy.cmd.Process.Kill()
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
	require.Eventually(g.s.T, func() bool {
		resp, err := g.httpc.Get(g.URL(path))
		if err != nil {
			return false
		}
		defer func() {
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
		}()
		return resp.StatusCode != http.StatusForbidden
	}, DefaultTimeout, DefaultInterval, "expected %s to not be blocked (not 403)", path)
}

// ExpectStatus polls until the given path returns the expected HTTP status.
func (g *GatewayProxy) ExpectStatus(path string, code int) {
	g.s.T.Helper()
	require.Eventually(g.s.T, func() bool {
		resp, err := g.httpc.Get(g.URL(path))
		if err != nil {
			return false
		}
		defer func() {
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
		}()
		return resp.StatusCode == code
	}, DefaultTimeout, DefaultInterval, "expected %s to return %d", path, code)
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

func (g *GatewayProxy) maintain() {
	for {
		select {
		case <-g.stopCh:
			return
		default:
			g.runPortForward()
			time.Sleep(2 * time.Second)
		}
	}
}

func (g *GatewayProxy) runPortForward() {
	labelSelector := fmt.Sprintf(
		"gateway.networking.k8s.io/gateway-name=%s", g.gateway,
	)

	pods, err := g.s.F.KubeClient.CoreV1().Pods(g.namespace).List(
		g.s.T.Context(),
		metav1.ListOptions{LabelSelector: labelSelector},
	)
	if err != nil || len(pods.Items) == 0 {
		return
	}

	podName := pods.Items[0].Name
	g.cmd = g.s.F.Kubectl(
		g.namespace, "port-forward", podName,
		fmt.Sprintf("%s:80", g.localPort),
	)
	if err := g.cmd.Start(); err != nil {
		return
	}
	_ = g.cmd.Wait()
}
