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
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestIntegration(t *testing.T) {
	ctx := context.Background()

	t.Log("Verifying Gateway exists")
	gateways, err := dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "gateways",
	}).Namespace(namespace).List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, len(gateways.Items), "Expected exactly one Gateway resource")

	t.Log("Testing before WAF is configured (should get 404)")
	resp, err := httpc.Get(fmt.Sprintf("http://localhost:%s/?test=evilmonkey", gatewayLocalPort))
	require.NoError(t, err)
	require.NotNil(t, resp)
	if resp.Body != nil {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
		}()
	}
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Equal(t, "istio-envoy", resp.Header.Get("Server"))

	t.Log("Applying RuleSet and Engine configurations")
	output, err := kubectl("apply", "-f", "../../config/samples/ruleset.yaml").CombinedOutput()
	require.NoError(t, err, "Failed to apply ruleset.yaml: %s", string(output))
	output, err = kubectl("apply", "-f", "../../config/samples/engine.yaml").CombinedOutput()
	require.NoError(t, err, "Failed to apply engine.yaml: %s", string(output))
	t.Cleanup(func() {
		t.Log("Cleaning up resources")
		if err := kubectl("delete", "-f", "../../config/samples/engine.yaml", "--ignore-not-found=true").Run(); err != nil {
			t.Logf("Failed to delete engine: %v", err)
		}
		if err := kubectl("delete", "-f", "../../config/samples/ruleset.yaml", "--ignore-not-found=true").Run(); err != nil {
			t.Logf("Failed to delete RuleSet: %v", err)
		}
	})

	t.Log("Waiting for WasmPlugin to be created")
	wasmPluginGVR := schema.GroupVersionResource{
		Group:    "extensions.istio.io",
		Version:  "v1alpha1",
		Resource: "wasmplugins",
	}
	require.Eventually(t, func() bool {
		_, err := dynamicClient.Resource(wasmPluginGVR).Namespace(namespace).Get(ctx, "coraza-engine-coraza", metav1.GetOptions{})
		return err == nil
	}, 30*time.Second, 1*time.Second)

	t.Log("Testing WAF blocks evilmonkey (should get 403)")
	require.Eventually(t, func() bool {
		resp, err := httpc.Get(fmt.Sprintf("http://localhost:%s/?test=evilmonkey", gatewayLocalPort))
		if err != nil {
			return false
		}
		defer func() {
			if resp != nil && resp.Body != nil {
				if err := resp.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
			}
		}()
		return resp.StatusCode == http.StatusForbidden
	}, 60*time.Second, 2*time.Second, "Expected WAF to block evilmonkey")

	t.Log("Testing sinistermonkey before adding rule (should get 404)")
	resp, err = httpc.Get(fmt.Sprintf("http://localhost:%s/sinistermonkey", gatewayLocalPort))
	require.NoError(t, err)
	if resp.Body != nil {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
		}()
	}
	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	t.Log("Creating ConfigMap with sinistermonkey rule")
	sinisterMonkeyConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-sinister-monkeys",
			Namespace: namespace,
		},
		Data: map[string]string{
			"rules": genSimpleSecRule("sinistermonkey"),
		},
	}
	_, err = kubeClient.CoreV1().ConfigMaps(namespace).Create(ctx, sinisterMonkeyConfigMap, metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := kubeClient.CoreV1().ConfigMaps(namespace).Delete(context.Background(), "no-sinister-monkeys", metav1.DeleteOptions{}); err != nil {
			t.Logf("Failed to delete ConfigMap: %v", err)
		}
	})

	t.Log("Updating RuleSet to include sinistermonkey rule")
	ruleSetGVR := schema.GroupVersionResource{
		Group:    "waf.k8s.coraza.io",
		Version:  "v1alpha1",
		Resource: "rulesets",
	}
	ruleSetObj, err := dynamicClient.Resource(ruleSetGVR).Namespace(namespace).Get(ctx, "default-ruleset", metav1.GetOptions{})
	require.NoError(t, err)

	t.Log("Adding ConfigMap reference to RuleSet")
	rules, found, err := unstructured.NestedSlice(ruleSetObj.Object, "spec", "rules")
	require.NoError(t, err)
	require.True(t, found)
	rules = append(rules, map[string]interface{}{
		"name": "no-sinister-monkeys",
	})
	err = unstructured.SetNestedSlice(ruleSetObj.Object, rules, "spec", "rules")
	require.NoError(t, err)
	_, err = dynamicClient.Resource(ruleSetGVR).Namespace(namespace).Update(ctx, ruleSetObj, metav1.UpdateOptions{})
	require.NoError(t, err)

	t.Log("Testing sinistermonkey after adding rule (should get 403)")
	require.Eventually(t, func() bool {
		resp, err := httpc.Get(fmt.Sprintf("http://localhost:%s/sinistermonkey", gatewayLocalPort))
		if err != nil {
			return false
		}

		defer func() {
			if resp != nil && resp.Body != nil {
				body, _ := io.ReadAll(resp.Body)
				if err := resp.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}

				if resp.StatusCode == http.StatusForbidden {
					t.Logf("Rule update successful - sinistermonkey blocked: %s", string(body))
				}
			}
		}()

		return resp.StatusCode == http.StatusForbidden
	}, 60*time.Second, 2*time.Second, "Expected WAF to block sinistermonkey after rule update")

	t.Log("Updating ConfigMap to replace sinistermonkey with maniacalmonkey")
	sinisterMonkeyConfigMap, err = kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, "no-sinister-monkeys", metav1.GetOptions{})
	require.NoError(t, err)
	sinisterMonkeyConfigMap.Data["rules"] = genSimpleSecRule("maniacalmonkey")
	_, err = kubeClient.CoreV1().ConfigMaps(namespace).Update(ctx, sinisterMonkeyConfigMap, metav1.UpdateOptions{})
	require.NoError(t, err)

	t.Log("Testing sinistermonkey is no longer blocked (should get 404)")
	require.Eventually(t, func() bool {
		resp, err := httpc.Get(fmt.Sprintf("http://localhost:%s/sinistermonkey", gatewayLocalPort))
		if err != nil {
			return false
		}
		defer func() {
			if resp != nil && resp.Body != nil {
				if err := resp.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
			}
		}()
		return resp.StatusCode == http.StatusNotFound
	}, 60*time.Second, 2*time.Second, "Expected sinistermonkey to not be blocked after rule update")

	t.Log("Testing maniacalmonkey is now blocked (should get 403)")
	require.Eventually(t, func() bool {
		resp, err := httpc.Get(fmt.Sprintf("http://localhost:%s/maniacalmonkey", gatewayLocalPort))
		if err != nil {
			return false
		}
		defer func() {
			if resp != nil && resp.Body != nil {
				body, _ := io.ReadAll(resp.Body)
				if err := resp.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
				if resp.StatusCode == http.StatusForbidden {
					t.Logf("ConfigMap update successful - maniacalmonkey blocked: %s", string(body))
				}
			}
		}()
		return resp.StatusCode == http.StatusForbidden
	}, 60*time.Second, 2*time.Second, "Expected WAF to block maniacalmonkey after ConfigMap update")
}

func genSimpleSecRule(target string) string {
	return fmt.Sprintf(`SecRule ARGS|REQUEST_URI|REQUEST_HEADERS "@contains %s" "id:3002,phase:2,deny,status:403,t:none,t:urlDecodeUni,msg:'%s Detected',severity:'CRITICAL'"`,
		target,
		target,
	)
}
