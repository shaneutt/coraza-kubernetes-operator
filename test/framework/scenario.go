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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DefaultTimeout is the default timeout for polling operations.
	DefaultTimeout = 60 * time.Second

	// DefaultInterval is the default polling interval.
	DefaultInterval = 2 * time.Second
)

// Scenario manages the lifecycle of a single test scenario: namespace
// creation, resource cleanup, and step tracking.
//
// Usage:
//
//	s := fw.NewScenario(t)
//	defer s.Cleanup()
//	s.CreateNamespace("test-ns")
//	s.Step("do something")
//	// ... test logic ...
type Scenario struct {
	T        *testing.T
	F        *Framework
	cleanups []func()
}

// NewScenario creates a Scenario bound to the given test. Always defer
// Cleanup() immediately after creation.
func (f *Framework) NewScenario(t *testing.T) *Scenario {
	t.Helper()
	return &Scenario{
		T: t,
		F: f,
	}
}

// Cleanup runs all registered cleanup functions in reverse order.
func (s *Scenario) Cleanup() {
	for i := len(s.cleanups) - 1; i >= 0; i-- {
		s.cleanups[i]()
	}
}

// OnCleanup registers a function to run during Cleanup (LIFO order).
func (s *Scenario) OnCleanup(fn func()) {
	s.cleanups = append(s.cleanups, fn)
}

// Step logs a named step in the test output for readability.
func (s *Scenario) Step(name string) {
	s.T.Helper()
	s.T.Logf("--- step: %s", name)
}

// CreateNamespace creates a namespace and registers it for cleanup.
func (s *Scenario) CreateNamespace(name string) {
	s.T.Helper()
	ctx := context.Background()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	_, err := s.F.KubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	require.NoError(s.T, err, "create namespace %s", name)

	s.T.Logf("Created namespace: %s", name)
	s.OnCleanup(func() {
		if err := s.F.KubeClient.CoreV1().Namespaces().Delete(
			context.Background(), name, metav1.DeleteOptions{},
		); err != nil {
			s.T.Logf("cleanup: failed to delete namespace %s: %v", name, err)
		}
	})
}

// ApplyManifest applies a YAML manifest file via kubectl and registers
// cleanup to delete it.
func (s *Scenario) ApplyManifest(namespace, path string) {
	s.T.Helper()
	out, err := s.F.Kubectl(namespace, "apply", "-f", path).CombinedOutput()
	require.NoError(s.T, err, "apply manifest %s: %s", path, string(out))

	s.T.Logf("Applied manifest: %s", path)
	s.OnCleanup(func() {
		_ = s.F.Kubectl(namespace, "delete", "-f", path, "--ignore-not-found=true").Run()
	})
}
