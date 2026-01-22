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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	wafv1alpha1 "github.com/shaneutt/coraza-kubernetes-operator/api/v1alpha1"
	"github.com/shaneutt/coraza-kubernetes-operator/internal/rulesets/cache"
)

// -----------------------------------------------------------------------------
// RuleSet Controller - RBAC
// -----------------------------------------------------------------------------

// +kubebuilder:rbac:groups=waf.k8s.coraza.io,resources=rulesets,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=waf.k8s.coraza.io,resources=rulesets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=waf.k8s.coraza.io,resources=rulesets/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// -----------------------------------------------------------------------------
// RuleSet Controller
// -----------------------------------------------------------------------------

// RuleSetReconciler reconciles a RuleSet object
type RuleSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Cache  *cache.RuleSetCache
}

// SetupWithManager sets up the controller with the Manager.
func (r *RuleSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&wafv1alpha1.RuleSet{}).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.findRuleSetsForConfigMap),
		).
		Named("ruleset").
		Complete(r)
}

// Reconcile handles reconciliation of RuleSet resources
func (r *RuleSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	LogDebug(log, req, "RuleSet", "Starting reconciliation")
	var ruleset wafv1alpha1.RuleSet
	if err := r.Get(ctx, req.NamespacedName, &ruleset); err != nil {
		if errors.IsNotFound(err) {
			LogDebug(log, req, "RuleSet", "Resource not found")
			return ctrl.Result{}, nil
		}
		LogError(log, req, "RuleSet", err, "Failed to GET")
		return ctrl.Result{}, err
	}

	LogInfo(log, req, "RuleSet", "Processing")

	LogDebug(log, req, "RuleSet", "Aggregating rules from sources", "ruleCount", len(ruleset.Spec.Rules))
	var aggregatedRules strings.Builder
	for i, rule := range ruleset.Spec.Rules {
		LogDebug(log, req, "RuleSet", "Processing rule source", "index", i, "ruleName", rule.Name, "kind", rule.Kind)
		if rule.Kind != "ConfigMap" {
			return ctrl.Result{}, fmt.Errorf("unsupported rule kind: %s", rule.Kind)
		}

		LogDebug(log, req, "RuleSet", "Fetching ConfigMap", "configMapName", rule.Name, "configMapNamespace", ruleset.Namespace)
		var cm corev1.ConfigMap
		if err := r.Get(ctx, types.NamespacedName{
			Name:      rule.Name,
			Namespace: ruleset.Namespace,
		}, &cm); err != nil {
			if errors.IsNotFound(err) {
				LogInfo(log, req, "RuleSet", "ConfigMap not found, requeueing", "configMapName", rule.Name)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			LogError(log, req, "RuleSet", err, "Failed to get ConfigMap", "configMapName", rule.Name)
			return ctrl.Result{}, err
		}

		data, ok := cm.Data["rules"]
		if !ok {
			LogError(log, req, "RuleSet", nil, "ConfigMap missing 'rules' key", "configMapName", rule.Name)
			return ctrl.Result{}, fmt.Errorf("ConfigMap %s missing 'rules' key", rule.Name)
		}

		aggregatedRules.WriteString(data)
		if i < len(ruleset.Spec.Rules)-1 {
			aggregatedRules.WriteString("\n")
		}
	}

	LogDebug(log, req, "RuleSet", "Storing aggregated rules in cache")
	instance := ruleset.Spec.Instance
	r.Cache.Put(instance, aggregatedRules.String())
	LogInfo(log, req, "RuleSet", "Stored rules in cache", "instance", instance)

	return ctrl.Result{}, nil
}
