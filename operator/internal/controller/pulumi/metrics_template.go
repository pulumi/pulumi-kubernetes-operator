// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pulumi

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	pulumiv1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1alpha1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// numTemplates tracks the total number of Template CRs currently managed.
	numTemplates = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "templates_active",
		Help: "Number of Template objects currently tracked by the Pulumi Kubernetes Operator",
	})

	// numTemplatesFailing tracks Templates where the last reconcile failed.
	numTemplatesFailing = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "templates_failing",
			Help: "Number of Templates currently registered where the last reconcile failed",
		},
		[]string{"namespace", "name"},
	)

	// numTemplatesReconciling tracks Templates that are currently being reconciled.
	numTemplatesReconciling = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "templates_reconciling",
			Help: "Number of Templates currently registered where reconciliation is in progress",
		},
		[]string{"namespace", "name"},
	)

	// numTemplateInstances tracks the total number of instances across all Templates.
	numTemplateInstances = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "template_instances_total",
			Help: "Number of instances for each Template",
		},
		[]string{"namespace", "name"},
	)

	// templateReconcileDuration tracks how long reconciliations take.
	templateReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "template_reconcile_duration_seconds",
			Help:    "Duration of Template reconciliations in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"namespace", "name", "result"},
	)
)

func init() {
	// Register Template custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(
		numTemplates,
		numTemplatesFailing,
		numTemplatesReconciling,
		numTemplateInstances,
		templateReconcileDuration,
	)
}

// newTemplateCallback is a callback that is called when a new Template object is created.
func newTemplateCallback(_ any) {
	numTemplates.Inc()
}

// updateTemplateCallback is a callback that is called when a Template object is updated.
func updateTemplateCallback(_, newObj any) {
	newTemplate, ok := newObj.(*pulumiv1alpha1.Template)
	if !ok {
		return
	}

	updateTemplateFailureMetrics(newTemplate)
	updateTemplateReconcilingMetrics(newTemplate)
	updateTemplateInstanceCountMetrics(newTemplate)
}

// updateTemplateFailureMetrics updates the failing metric based on Template status.
func updateTemplateFailureMetrics(template *pulumiv1alpha1.Template) {
	labels := prometheus.Labels{"namespace": template.Namespace, "name": template.Name}

	// Check if the Ready condition is False (failed state)
	readyCondition := apimeta.FindStatusCondition(template.Status.Conditions, pulumiv1alpha1.TemplateConditionTypeReady)
	if readyCondition != nil && readyCondition.Status == metav1.ConditionFalse {
		numTemplatesFailing.With(labels).Set(1)
	} else {
		numTemplatesFailing.With(labels).Set(0)
	}
}

// updateTemplateReconcilingMetrics updates the reconciling metric based on Template status.
func updateTemplateReconcilingMetrics(template *pulumiv1alpha1.Template) {
	labels := prometheus.Labels{"namespace": template.Namespace, "name": template.Name}

	// Check if the Reconciling condition is True
	reconcilingCondition := apimeta.FindStatusCondition(template.Status.Conditions, pulumiv1alpha1.TemplateConditionTypeReconciling)
	if reconcilingCondition != nil && reconcilingCondition.Status == metav1.ConditionTrue {
		numTemplatesReconciling.With(labels).Set(1)
	} else {
		numTemplatesReconciling.With(labels).Set(0)
	}
}

// updateTemplateInstanceCountMetrics updates the instance count metric.
func updateTemplateInstanceCountMetrics(template *pulumiv1alpha1.Template) {
	labels := prometheus.Labels{"namespace": template.Namespace, "name": template.Name}
	numTemplateInstances.With(labels).Set(float64(template.Status.InstanceCount))
}

// deleteTemplateCallback is a callback that is called when a Template object is deleted.
func deleteTemplateCallback(oldObj any) {
	numTemplates.Dec()
	val, err := getGaugeValue(numTemplates)
	if err == nil && val < 0 {
		numTemplates.Set(0)
	}

	oldTemplate, ok := oldObj.(*pulumiv1alpha1.Template)
	if !ok {
		return
	}

	// Reset any gauge metrics associated with the old template.
	labels := prometheus.Labels{"namespace": oldTemplate.Namespace, "name": oldTemplate.Name}
	numTemplatesFailing.With(labels).Set(0)
	numTemplatesReconciling.With(labels).Set(0)
	numTemplateInstances.With(labels).Set(0)
}

// RecordReconcileDuration records the duration of a reconciliation.
func RecordTemplateReconcileDuration(namespace, name string, startTime time.Time, result string) {
	duration := time.Since(startTime).Seconds()
	templateReconcileDuration.With(prometheus.Labels{
		"namespace": namespace,
		"name":      name,
		"result":    result,
	}).Observe(duration)
}
