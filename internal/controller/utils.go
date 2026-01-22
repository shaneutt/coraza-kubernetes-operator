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

package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// -----------------------------------------------------------------------------
// Logging Levels
// -----------------------------------------------------------------------------

const (
	// DebugLevel is the go-logr level for debug/verbose logging
	DebugLevel = 1
)

// -----------------------------------------------------------------------------
// Logging Utilities
// -----------------------------------------------------------------------------

// LogInfo logs an info-level message with consistent structured context.
func LogInfo(log logr.Logger, req ctrl.Request, kind, msg string, keysAndValues ...interface{}) {
	args := append([]interface{}{"namespace", req.Namespace, "name", req.Name}, keysAndValues...)
	log.Info(fmt.Sprintf("%s: %s", kind, msg), args...)
}

// LogDebug logs a debug-level message with consistent structured context.
func LogDebug(log logr.Logger, req ctrl.Request, kind, msg string, keysAndValues ...interface{}) {
	args := append([]interface{}{"namespace", req.Namespace, "name", req.Name}, keysAndValues...)
	log.V(DebugLevel).Info(fmt.Sprintf("%s: %s", kind, msg), args...)
}

// LogError logs an error-level message with consistent structured context.
func LogError(log logr.Logger, req ctrl.Request, kind string, err error, msg string, keysAndValues ...interface{}) {
	args := append([]interface{}{"namespace", req.Namespace, "name", req.Name}, keysAndValues...)
	log.Error(err, fmt.Sprintf("%s: %s", kind, msg), args...)
}

// -----------------------------------------------------------------------------
// Kubernetes Client Operation Utilities
// -----------------------------------------------------------------------------

// CreateOrUpdate creates or updates an unstructured Kubernetes object.
// If the object doesn't exist, it creates it. If it exists, it updates it.
//
// The desired object must have its GVK and name set.
func CreateOrUpdate(ctx context.Context, c client.Client, desired *unstructured.Unstructured) error {
	gvk := desired.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		return errors.New("desired object must have GroupVersionKind set")
	}

	namespace, name := desired.GetNamespace(), desired.GetName()
	if name == "" {
		return errors.New("desired object must have a name set")
	}
	if namespace == "" {
		namespace = corev1.NamespaceDefault
	}

	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(desired.GetObjectKind().GroupVersionKind())

	err := c.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      desired.GetName(),
	}, resource)

	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := c.Create(ctx, desired); err != nil {
				return fmt.Errorf("failed to create %s/%s in namespace %s: %w", gvk.Kind, name, namespace, err)
			}
			return nil
		}
		return fmt.Errorf("failed to get %s/%s in namespace %s: %w", gvk.Kind, name, namespace, err)
	}

	desired.SetResourceVersion(resource.GetResourceVersion())

	if err := c.Update(ctx, desired); err != nil {
		return fmt.Errorf("failed to update %s/%s in namespace %s: %w", gvk.Kind, name, namespace, err)
	}

	return nil
}
