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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	wafv1alpha1 "github.com/networking-incubator/coraza-kubernetes-operator/api/v1alpha1"
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

// provisionIstioEngineWithWasm provisions the Istio WasmPlugin resource for
// the Engine.
func (r *EngineReconciler) provisionIstioEngineWithWasm(ctx context.Context, log logr.Logger, req ctrl.Request, engine wafv1alpha1.Engine) (ctrl.Result, error) {
	logDebug(log, req, "Engine", "Building WasmPlugin resource")
	wasmPlugin := r.buildWasmPlugin(&engine)

	logDebug(log, req, "Engine", "Setting controller reference on WasmPlugin")
	if err := controllerutil.SetControllerReference(&engine, wasmPlugin, r.Scheme); err != nil {
		logError(log, req, "Engine", err, "Failed to set owner reference on WasmPlugin")
		return ctrl.Result{}, err
	}

	logDebug(log, req, "Engine", "Applying WasmPlugin", "wasmPluginName", wasmPlugin.GetName())
	if err := serverSideApply(ctx, r.Client, wasmPlugin); err != nil {
		logError(log, req, "Engine", err, "Failed to create or update WasmPlugin")
		r.Recorder.Event(&engine, "Warning", "ProvisioningFailed", fmt.Sprintf("Failed to create WasmPlugin: %v", err))

		patch := client.MergeFrom(engine.DeepCopy())
		setStatusConditionDegraded(log, req, "Engine", &engine.Status.Conditions, engine.Generation, "ProvisioningFailed", fmt.Sprintf("Failed to create or update WasmPlugin: %v", err))
		if updateErr := r.Status().Patch(ctx, &engine, patch); updateErr != nil {
			logError(log, req, "Engine", updateErr, "Failed to patch status after provisioning failure")
		}

		return ctrl.Result{}, err
	}
	logInfo(log, req, "Engine", "WasmPlugin provisioned", "wasmNamespace", wasmPlugin.GetNamespace(), "wasmName", wasmPlugin.GetName())

	logDebug(log, req, "Engine", "Updating status after successful provisioning")
	patch := client.MergeFrom(engine.DeepCopy())
	setStatusReady(log, req, "Engine", &engine.Status.Conditions, engine.Generation, "Configured", "WasmPlugin successfully created/updated")
	setStatusEngineOwnedResources(&engine, wasmPlugin.GetNamespace(), wasmPlugin.GetName())
	if err := r.Status().Patch(ctx, &engine, patch); err != nil {
		logError(log, req, "Engine", err, "Failed to patch status")
		return ctrl.Result{}, err
	}
	r.Recorder.Event(&engine, "Normal", "WasmPluginCreated", fmt.Sprintf("Created WasmPlugin %s/%s", wasmPlugin.GetNamespace(), wasmPlugin.GetName()))

	return ctrl.Result{}, nil
}

// -----------------------------------------------------------------------------
// Engine Controller - Istio Driver - Cleanup
// -----------------------------------------------------------------------------

// cleanupIstioEngineWithWasm attempts to delete all owned resources (e.g.
// WasmPlugins). Tracks resources that remain (still deleting or failed to
// delete). Returns true only when all resources are confirmed gone.
//
// Even though we use OwnerReferences, this explicit cleanup helps to avoid
// race conditions that could orphan resources.
func (r *EngineReconciler) cleanupIstioEngineWithWasm(ctx context.Context, log logr.Logger, engine *wafv1alpha1.Engine) (bool, error) {
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: engine.Namespace, Name: engine.Name}}

	if len(engine.Status.OwnedResources) == 0 {
		logDebug(log, req, "Engine", "No resources to clean up")
		return true, nil
	}

	logDebug(log, req, "Engine", "Cleaning up owned resources", "count", len(engine.Status.OwnedResources))
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
				logDebug(log, req, "Engine", "Owned resource already deleted", "kind", resourceRef.Kind, "resourceName", resourceRef.Name)
				continue
			}

			logError(log, req, "Engine", err, "Failed to get owned resource for deletion", "kind", resourceRef.Kind, "resourceName", resourceRef.Name)
			remainingResources = append(remainingResources, resourceRef)
			allOwnedResourcesDeleted = false
			continue
		}

		logInfo(log, req, "Engine", "Deleting owned resource", "kind", resource.GetKind(), "resourceName", resource.GetName(), "resourceNamespace", resource.GetNamespace())

		if err := r.Client.Delete(ctx, resource); err != nil {
			logError(log, req, "Engine", err, "Failed to delete resource", "kind", resource.GetKind(), "resourceName", resource.GetName())
			r.Recorder.Event(engine, "Warning", "FailedDelete", fmt.Sprintf("Failed to delete %s/%s: %v", resource.GetKind(), resource.GetName(), err))
			remainingResources = append(remainingResources, resourceRef)
			allOwnedResourcesDeleted = false
			continue
		}

		logDebug(log, req, "Engine", "Waiting for owned resource deletion", "kind", resource.GetKind(), "resourceName", resource.GetName())
		remainingResources = append(remainingResources, resourceRef)
		allOwnedResourcesDeleted = false
	}

	logDebug(log, req, "Engine", "Updating status after cleanup", "remainingResourceCount", len(remainingResources))
	if len(remainingResources) != len(engine.Status.OwnedResources) {
		patch := client.MergeFrom(engine.DeepCopy())
		engine.Status.OwnedResources = remainingResources
		if err := r.Status().Patch(ctx, engine, patch); err != nil {
			logError(log, req, "Engine", err, "Failed to patch status after resource deletion")
			return false, err
		}
	}

	if allOwnedResourcesDeleted {
		logInfo(log, req, "Engine", "All owned resources deleted successfully")
	}

	return allOwnedResourcesDeleted, nil
}

// -----------------------------------------------------------------------------
// Engine Controller - Istio Driver - WasmPlugin Builder
// -----------------------------------------------------------------------------

func (r *EngineReconciler) buildWasmPlugin(engine *wafv1alpha1.Engine) *unstructured.Unstructured {
	namespace := engine.Spec.RuleSet.Namespace
	if namespace == "" {
		namespace = engine.Namespace
	}
	rulesetKey := fmt.Sprintf("%s/%s", namespace, engine.Spec.RuleSet.Name)

	pluginConfig := map[string]interface{}{
		"cache_server_instance": rulesetKey,
		"cache_server_cluster":  r.ruleSetCacheServerCluster,
	}

	if engine.Spec.Driver.Istio.Wasm.RuleSetCacheServer != nil {
		pluginConfig["rule_reload_interval_seconds"] = engine.Spec.Driver.Istio.Wasm.RuleSetCacheServer.PollIntervalSeconds
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
				"url":          engine.Spec.Driver.Istio.Wasm.Image,
				"pluginConfig": pluginConfig,
				"selector": map[string]interface{}{
					"matchLabels": engine.Spec.Driver.Istio.Wasm.WorkloadSelector.MatchLabels,
				},
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

// -----------------------------------------------------------------------------
// Engine Controller - Istio Driver - Status Helpers
// -----------------------------------------------------------------------------

// setStatusEngineOwnedResources updates the Engine's OwnedResources status field
// with the WasmPlugin reference. If a WasmPlugin entry already exists, it updates it;
// otherwise, it appends a new entry.
func setStatusEngineOwnedResources(engine *wafv1alpha1.Engine, namespace, name string) {
	wasmPluginRef := corev1.ObjectReference{
		APIVersion: "extensions.istio.io/v1alpha1",
		Kind:       "WasmPlugin",
		Name:       name,
		Namespace:  namespace,
	}

	for i := range engine.Status.OwnedResources {
		if engine.Status.OwnedResources[i].Kind == "WasmPlugin" &&
			engine.Status.OwnedResources[i].APIVersion == "extensions.istio.io/v1alpha1" {
			engine.Status.OwnedResources[i] = wasmPluginRef
			return
		}
	}

	engine.Status.OwnedResources = append(engine.Status.OwnedResources, wasmPluginRef)
}
