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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	wafv1alpha1 "github.com/shaneutt/coraza-kubernetes-operator/api/v1alpha1"
)

// -----------------------------------------------------------------------------
// Engine Controller - RBAC
// -----------------------------------------------------------------------------

// +kubebuilder:rbac:groups=waf.k8s.coraza.io,resources=engines,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=waf.k8s.coraza.io,resources=engines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=waf.k8s.coraza.io,resources=engines/finalizers,verbs=update

// -----------------------------------------------------------------------------
// Engine Controller - Consts
// -----------------------------------------------------------------------------

const (
	// EngineFinalizer is the finalizer used for Engine resource cleanup.
	EngineFinalizer = "waf.k8s.coraza.io/engine-finalizer"
)

// -----------------------------------------------------------------------------
// Engine Controller
// -----------------------------------------------------------------------------

// EngineReconciler reconciles an Engine object
type EngineReconciler struct {
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	client.Client
	ruleSetCacheServerCluster string
}

// SetupWithManager sets up the controller with the Manager.
func (r *EngineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	wasmPlugin := &unstructured.Unstructured{}
	wasmPlugin.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "extensions.istio.io",
		Version: "v1alpha1",
		Kind:    "WasmPlugin",
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&wafv1alpha1.Engine{}).
		Owns(wasmPlugin).
		Named("engine").
		Complete(r)
}

// -----------------------------------------------------------------------------
// Engine Controller - Reconciler
// -----------------------------------------------------------------------------

// Reconcile handles reconciliation of Engine resources
func (r *EngineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	LogDebug(log, req, "Engine", "Starting reconciliation")
	var engine wafv1alpha1.Engine
	if err := r.Get(ctx, req.NamespacedName, &engine); err != nil {
		if apierrors.IsNotFound(err) {
			LogDebug(log, req, "Engine", "Resource not found")
			return ctrl.Result{Requeue: false}, nil
		}

		LogError(log, req, "Engine", err, "Failed to get")
		return ctrl.Result{Requeue: true}, err
	}

	// Handle Engine deletion: clean up owned resources (WasmPlugin, etc.) before removing
	// finalizer. Returns early to requeue if cleanup is incomplete.
	if !engine.DeletionTimestamp.IsZero() && engine.DeletionTimestamp.Time.Before(metav1.Now().Time) {
		LogDebug(log, req, "Engine", "Handling deletion")

		if controllerutil.ContainsFinalizer(&engine, EngineFinalizer) {
			LogInfo(log, req, "Engine", "Cleaning up resources")
			cleanupComplete, err := r.cleanupEngine(ctx, log, &engine)
			if err != nil {
				LogError(log, req, "Engine", err, "Failed to cleanup resources")
				return ctrl.Result{}, err
			}

			if !cleanupComplete {
				LogDebug(log, req, "Engine", "Cleanup not yet complete, requeueing")
				return ctrl.Result{Requeue: true}, nil
			}

			controllerutil.RemoveFinalizer(&engine, EngineFinalizer)
			if err := r.Update(ctx, &engine); err != nil {
				LogError(log, req, "Engine", err, "Failed to remove finalizer")
				return ctrl.Result{}, err
			}

			r.Recorder.Event(&engine, "Normal", "Deleted", "Engine resources cleaned up")
		}

		LogDebug(log, req, "Engine", "Cleanup handled successfully")
		return ctrl.Result{}, nil
	}

	LogDebug(log, req, "Engine", "Applying finalizers")
	if !controllerutil.ContainsFinalizer(&engine, EngineFinalizer) {
		LogDebug(log, req, "Engine", "Adding finalizer")
		controllerutil.AddFinalizer(&engine, EngineFinalizer)
		if err := r.Update(ctx, &engine); err != nil {
			LogError(log, req, "Engine", err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
	}

	LogDebug(log, req, "Engine", "Applying conditions")
	if !apimeta.IsStatusConditionPresentAndEqual(engine.Status.Conditions, "Ready", "Unknown") &&
		!apimeta.IsStatusConditionPresentAndEqual(engine.Status.Conditions, "Ready", "True") &&
		!apimeta.IsStatusConditionPresentAndEqual(engine.Status.Conditions, "Ready", "False") {
		LogDebug(log, req, "Engine", "Setting initial Ready condition")
		apimeta.SetStatusCondition(&engine.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionUnknown,
			ObservedGeneration: engine.Generation,
			LastTransitionTime: metav1.Now(),
			Reason:             "Reconciling",
			Message:            "Starting reconciliation",
		})
		if err := r.Status().Update(ctx, &engine); err != nil {
			LogError(log, req, "Engine", err, "Failed to update initial status")
			return ctrl.Result{}, err
		}
	}

	LogDebug(log, req, "Engine", "Selecting driver and provisioning", "driverType", engine.Spec.Driver.Type, "driverMode", engine.Spec.Driver.Mode)
	return r.selectDriverAndProvisioningEngine(ctx, log, req, engine)
}

// -----------------------------------------------------------------------------
// Engine Controller - Driver Provisioning
// -----------------------------------------------------------------------------

func (r *EngineReconciler) selectDriverAndProvisioningEngine(ctx context.Context, log logr.Logger, req ctrl.Request, engine wafv1alpha1.Engine) (ctrl.Result, error) {
	LogDebug(log, req, "Engine", "Evaluating driver type and mode", "type", engine.Spec.Driver.Type, "mode", engine.Spec.Driver.Mode)

	switch engine.Spec.Driver.Type {
	case wafv1alpha1.EngineDriverTypeIstio:
		switch engine.Spec.Driver.Mode {
		case wafv1alpha1.EngineDriverModeWasm:
			LogDebug(log, req, "Engine", "Using Istio driver with WASM mode")
			return r.provisionIstioEngine(ctx, log, req, engine)
		default:
			err := fmt.Errorf("unsupported driver mode for Istio: %s (only 'wasm' is currently supported)", engine.Spec.Driver.Mode)
			LogError(log, req, "Engine", err, "Invalid driver mode specified")

			r.Recorder.Event(&engine, "Warning", "InvalidConfiguration", err.Error())
			apimeta.SetStatusCondition(&engine.Status.Conditions, metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				ObservedGeneration: engine.Generation,
				LastTransitionTime: metav1.Now(),
				Reason:             "InvalidConfiguration",
				Message:            err.Error(),
			})

			if updateErr := r.Status().Update(ctx, &engine); updateErr != nil {
				LogError(log, req, "Engine", updateErr, "Failed to update status after validation error")
				return ctrl.Result{}, fmt.Errorf("validation failed: %w (status update also failed: %v)", err, updateErr)
			}

			return ctrl.Result{}, err
		}
	default:
		err := fmt.Errorf("unsupported driver type: %s (only 'istio' is currently supported)", engine.Spec.Driver.Type)
		LogError(log, req, "Engine", err, "Invalid driver type specified")

		r.Recorder.Event(&engine, "Warning", "InvalidConfiguration", err.Error())
		apimeta.SetStatusCondition(&engine.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			ObservedGeneration: engine.Generation,
			LastTransitionTime: metav1.Now(),
			Reason:             "InvalidConfiguration",
			Message:            err.Error(),
		})

		if updateErr := r.Status().Update(ctx, &engine); updateErr != nil {
			LogError(log, req, "Engine", updateErr, "Failed to update status after validation error")
			return ctrl.Result{}, fmt.Errorf("validation failed: %w (status update also failed: %v)", err, updateErr)
		}

		return ctrl.Result{}, err
	}
}

// -----------------------------------------------------------------------------
// Engine Controller - Driver Cleanup
// -----------------------------------------------------------------------------

func (r *EngineReconciler) cleanupEngine(ctx context.Context, log logr.Logger, engine *wafv1alpha1.Engine) (bool, error) {
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: engine.Namespace, Name: engine.Name}}

	LogDebug(log, req, "Engine", "Cleaning up based on driver type", "type", engine.Spec.Driver.Type, "mode", engine.Spec.Driver.Mode)

	switch engine.Spec.Driver.Type {
	case wafv1alpha1.EngineDriverTypeIstio:
		switch engine.Spec.Driver.Mode {
		case wafv1alpha1.EngineDriverModeWasm:
			LogDebug(log, req, "Engine", "Using Istio driver cleanup with WASM mode")
			return r.cleanupIstioEngine(ctx, log, engine)
		default:
			err := fmt.Errorf("unsupported driver mode for Istio during cleanup: %s", engine.Spec.Driver.Mode)
			LogError(log, req, "Engine", err, "Invalid driver mode specified")
			return false, err
		}
	default:
		err := fmt.Errorf("unsupported driver type during cleanup: %s", engine.Spec.Driver.Type)
		LogError(log, req, "Engine", err, "Invalid driver type specified")
		return false, err
	}
}
