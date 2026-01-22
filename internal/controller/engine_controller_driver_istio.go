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
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	wafv1alpha1 "github.com/shaneutt/coraza-kubernetes-operator/api/v1alpha1"
)

// -----------------------------------------------------------------------------
// Engine Controller - Istio RBAC
// -----------------------------------------------------------------------------

// +kubebuilder:rbac:groups=extensions.istio.io,resources=wasmplugins,verbs=get;list;watch;create;update;patch;delete

// -----------------------------------------------------------------------------
// Engine Controller - Istio Consts
// -----------------------------------------------------------------------------

// WasmPluginNamePrefix is the prefix used for all created WasmPlugin resources
const WasmPluginNamePrefix = "coraza-engine-"

// -----------------------------------------------------------------------------
// Engine Controller - Istio Driver - Provisioning
// -----------------------------------------------------------------------------

// provisionIstioEngine provisions the Istio WasmPlugin resource for the Engine.
func (r *EngineReconciler) provisionIstioEngine(ctx context.Context, log logr.Logger, req ctrl.Request, engine wafv1alpha1.Engine) (ctrl.Result, error) {
	LogDebug(log, req, "Engine", "Building WasmPlugin resource")
	wasmPlugin := r.buildWasmPlugin(&engine)

	LogDebug(log, req, "Engine", "Setting controller reference on WasmPlugin")
	if err := controllerutil.SetControllerReference(&engine, wasmPlugin, r.Scheme); err != nil {
		LogError(log, req, "Engine", err, "Failed to set owner reference on WasmPlugin")
		return ctrl.Result{}, err
	}

	LogDebug(log, req, "Engine", "Creating or updating WasmPlugin", "wasmPluginName", wasmPlugin.GetName())
	if err := CreateOrUpdate(ctx, r.Client, wasmPlugin); err != nil {
		LogError(log, req, "Engine", err, "Failed to create or update WasmPlugin")
		return ctrl.Result{}, err
	}
	LogInfo(log, req, "Engine", "WasmPlugin provisioned", "wasmNamespace", wasmPlugin.GetNamespace(), "wasmName", wasmPlugin.GetName())

	LogDebug(log, req, "Engine", "Updating status")
	apimeta.SetStatusCondition(&engine.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: engine.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             "Configured",
		Message:            "WasmPlugin successfully created/updated",
	})

	wasmPluginRef := corev1.ObjectReference{
		APIVersion: "extensions.istio.io/v1alpha1",
		Kind:       "WasmPlugin",
		Name:       wasmPlugin.GetName(),
		Namespace:  wasmPlugin.GetNamespace(),
	}

	found := false
	for i := range engine.Status.OwnedResources {
		if engine.Status.OwnedResources[i].Kind == "WasmPlugin" &&
			engine.Status.OwnedResources[i].APIVersion == "extensions.istio.io/v1alpha1" {
			engine.Status.OwnedResources[i] = wasmPluginRef
			found = true
			break
		}
	}

	if !found {
		engine.Status.OwnedResources = append(engine.Status.OwnedResources, wasmPluginRef)
	}

	if err := r.Status().Update(ctx, &engine); err != nil {
		LogError(log, req, "Engine", err, "Failed to update status")
		return ctrl.Result{}, err
	}

	r.Recorder.Event(&engine, "Normal", "WasmPluginCreated", fmt.Sprintf("Created WasmPlugin %s/%s", wasmPlugin.GetNamespace(), wasmPlugin.GetName()))

	return ctrl.Result{}, nil

}

// -----------------------------------------------------------------------------
// Engine Controller - Istio Driver - Cleanup
// -----------------------------------------------------------------------------

// cleanupIstioEngine attempts to delete all owned resources (e.g. WasmPlugins).
// Tracks resources that remain (still deleting or failed to delete). Returns
// true only when all resources are confirmed gone.
//
// Even though we use OwnerReferences, this explicit cleanup helps to avoid
// race conditions that could orphan resources.
func (r *EngineReconciler) cleanupIstioEngine(ctx context.Context, log logr.Logger, engine *wafv1alpha1.Engine) (bool, error) {
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: engine.Namespace, Name: engine.Name}}

	if len(engine.Status.OwnedResources) == 0 {
		LogDebug(log, req, "Engine", "No resources to clean up")
		return true, nil
	}

	LogDebug(log, req, "Engine", "Cleaning up owned resources", "count", len(engine.Status.OwnedResources))
	remainingResources := make([]corev1.ObjectReference, 0, len(engine.Status.OwnedResources))
	allOwnedResourcesDeleted := true
	for _, resourceRef := range engine.Status.OwnedResources {
		resource := &unstructured.Unstructured{}
		resource.SetAPIVersion(resourceRef.APIVersion)
		resource.SetKind(resourceRef.Kind)
		resource.SetName(resourceRef.Name)
		resource.SetNamespace(resourceRef.Namespace)

		err := r.Client.Get(ctx, client.ObjectKeyFromObject(resource), resource)
		if err != nil {
			if apierrors.IsNotFound(err) {
				LogDebug(log, req, "Engine", "Owned resource already deleted", "kind", resourceRef.Kind, "resourceName", resourceRef.Name)
				continue
			}

			LogError(log, req, "Engine", err, "Failed to get owned resource for deletion", "kind", resourceRef.Kind, "resourceName", resourceRef.Name)
			remainingResources = append(remainingResources, resourceRef)
			allOwnedResourcesDeleted = false
			continue
		}

		LogInfo(log, req, "Engine", "Deleting owned resource", "kind", resource.GetKind(), "resourceName", resource.GetName(), "resourceNamespace", resource.GetNamespace())

		if err := r.Client.Delete(ctx, resource); err != nil {
			LogError(log, req, "Engine", err, "Failed to delete resource", "kind", resource.GetKind(), "resourceName", resource.GetName())
			r.Recorder.Event(engine, "Warning", "FailedDelete", fmt.Sprintf("Failed to delete %s/%s: %v", resource.GetKind(), resource.GetName(), err))
			remainingResources = append(remainingResources, resourceRef)
			allOwnedResourcesDeleted = false
			continue
		}

		LogDebug(log, req, "Engine", "Waiting for owned resource deletion", "kind", resource.GetKind(), "resourceName", resource.GetName())
		remainingResources = append(remainingResources, resourceRef)
		allOwnedResourcesDeleted = false
	}

	LogDebug(log, req, "Engine", "Updating status after cleanup", "remainingResourceCount", len(remainingResources))
	if len(remainingResources) != len(engine.Status.OwnedResources) {
		engine.Status.OwnedResources = remainingResources
		if err := r.Status().Update(ctx, engine); err != nil {
			LogError(log, req, "Engine", err, "Failed to update status after resource deletion")
			return false, err
		}
	}

	if allOwnedResourcesDeleted {
		LogInfo(log, req, "Engine", "All owned resources deleted successfully")
	}

	return allOwnedResourcesDeleted, nil
}

// -----------------------------------------------------------------------------
// Engine Controller - Istio Driver - WasmPlugin Builder
// -----------------------------------------------------------------------------

func (r *EngineReconciler) buildWasmPlugin(engine *wafv1alpha1.Engine) *unstructured.Unstructured {
	pluginConfig := map[string]interface{}{
		"cache_server_instance": engine.Spec.Instance,
		"cache_server_cluster":  r.ruleSetCacheServerCluster,
	}

	if engine.Spec.Driver.RuleSetCacheServer != nil {
		pluginConfig["rule_reload_interval_seconds"] = engine.Spec.Driver.RuleSetCacheServer.PollIntervalSeconds
	}

	wasmPlugin := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "extensions.istio.io/v1alpha1",
			"kind":       "WasmPlugin",
			"metadata": map[string]interface{}{
				"name":      fmt.Sprintf("%s%s", WasmPluginNamePrefix, engine.Name),
				"namespace": engine.Namespace,
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": engine.Spec.Driver.Istio.WorkloadSelector.MatchLabels,
				},
				"url":          engine.Spec.Driver.Wasm.Image,
				"pluginConfig": pluginConfig,
			},
		},
	}

	wasmPlugin.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "extensions.istio.io",
		Version: "v1alpha1",
		Kind:    "WasmPlugin",
	})

	return wasmPlugin
}
