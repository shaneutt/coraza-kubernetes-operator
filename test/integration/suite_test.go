//go:build integration

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

package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// -----------------------------------------------------------------------------
// Integration Test Suite - Vars
// -----------------------------------------------------------------------------

var (
	kindClusterName = os.Getenv("KIND_CLUSTER_NAME")

	httpc = &http.Client{Timeout: 10 * time.Second}

	restConfig    *rest.Config
	kubeClient    *kubernetes.Clientset
	dynamicClient dynamic.Interface
	namespace     = "integration-tests"

	portForwardCmd   *exec.Cmd
	gatewayLocalPort = "28080"
	stopPortForward  = make(chan struct{})
)

// -----------------------------------------------------------------------------
// Integration Test Suite - Main
// -----------------------------------------------------------------------------

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// -----------------------------------------------------------------------------
// Integration Test Suite - Setup & Cleanup
// -----------------------------------------------------------------------------

func setup() {
	if kindClusterName == "" {
		panic("KIND_CLUSTER_NAME environment variable is required")
	}

	cmd := exec.Command("kind", "get", "kubeconfig", "--name", kindClusterName)
	output, err := cmd.Output()
	if err != nil {
		panic(fmt.Sprintf("failed to get kind kubeconfig: %v", err))
	}

	config, err := clientcmd.RESTConfigFromKubeConfig(output)
	if err != nil {
		panic(fmt.Sprintf("failed to parse kubeconfig: %v", err))
	}
	restConfig = config

	kubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(fmt.Sprintf("failed to create kubernetes client: %v", err))
	}

	dynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		panic(fmt.Sprintf("failed to create dynamic client: %v", err))
	}

	setupPortForward()
}

func cleanup() {
	close(stopPortForward)
	time.Sleep(2 * time.Second)
	if portForwardCmd != nil && portForwardCmd.Process != nil {
		if err := portForwardCmd.Process.Kill(); err != nil {
			fmt.Printf("Failed to kill port-forward process: %v\n", err)
		}
	}
}

// -----------------------------------------------------------------------------
// Integration Test Suite - Cluster Utils
// -----------------------------------------------------------------------------

func kubectl(args ...string) *exec.Cmd {
	cmdArgs := []string{"--context", fmt.Sprintf("kind-%s", kindClusterName), "-n", namespace} //nolint:prealloc
	cmdArgs = append(cmdArgs, args...)
	return exec.Command("kubectl", cmdArgs...)
}

func setupPortForward() {
	ctx := context.Background()

	gatewayGVR := schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "gateways",
	}
	gateways, err := dynamicClient.Resource(gatewayGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil || len(gateways.Items) == 0 {
		panic(fmt.Sprintf("failed to find Gateway resource: %v", err))
	}

	if len(gateways.Items) > 1 {
		panic("expected exactly one Gateway resource")
	}

	gatewayName := gateways.Items[0].GetName()
	labelSelector := fmt.Sprintf("gateway.networking.k8s.io/gateway-name=%s", gatewayName)

	go func() {
		for {
			select {
			case <-stopPortForward:
				fmt.Println("Stopping port-forward")
				return
			default:
				runPortForward(labelSelector)
				time.Sleep(2 * time.Second)
			}
		}
	}()

	waitForGatewayReady()
}

func runPortForward(labelSelector string) {
	ctx := context.Background()

	pods, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil || len(pods.Items) == 0 {
		fmt.Printf("Failed to find gateway pod (will retry): %v\n", err)
		return
	}

	podName := pods.Items[0].Name

	portForwardCmd = exec.Command("kubectl", "--context", fmt.Sprintf("kind-%s", kindClusterName), "-n", namespace, "port-forward", podName, fmt.Sprintf("%s:80", gatewayLocalPort))
	if err := portForwardCmd.Start(); err != nil {
		fmt.Printf("Failed to start port-forward (will retry): %v\n", err)
		return
	}

	fmt.Printf("Port forwarding %s:80 to localhost:%s\n", podName, gatewayLocalPort)

	if err := portForwardCmd.Wait(); err != nil {
		fmt.Printf("Port-forward exited (will retry): %v\n", err)
	}
}

func waitForGatewayReady() {
	fmt.Println("Waiting for gateway to be ready...")
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		resp, err := httpc.Get(fmt.Sprintf("http://localhost:%s", gatewayLocalPort))
		if err == nil {
			defer func() {
				if err := resp.Body.Close(); err != nil {
					fmt.Printf("Failed to close response body: %v\n", err)
				}
			}()

			if resp.StatusCode == 404 && resp.Header.Get("Server") == "istio-envoy" {
				fmt.Println("Gateway is responding")
				return
			}
		}
		time.Sleep(1 * time.Second)
	}
	panic("Gateway did not become ready in time")
}
