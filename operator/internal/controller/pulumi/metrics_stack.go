// Copyright 2021, Pulumi Corporation.  All rights reserved.

package pulumi

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	numStacks = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "stacks_active",
		Help: "Number of stacks currently tracked by the Pulumi Kubernetes Operator",
	})

	numStacksFailing = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "stacks_failing",
			Help: "Number of stacks currently registered where the last reconcile failed",
		},
		[]string{"namespace", "name"},
	)

	numStacksReconciling = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "stacks_reconciling",
			Help: "Number of stacks currently registered where the last reconcile is in progress",
		},
		[]string{"namespace", "name"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(numStacks, numStacksFailing)
}

// newStackCallback is a callback that is called when a new Stack object is created.
func newStackCallback(obj any) {
	numStacks.Inc()
}

// updateStackCallback is a callback that is called when a Stack object is updated.
func updateStackCallback(_, newObj any) {
	newStack, ok := newObj.(*pulumiv1.Stack)
	if !ok {
		return
	}

	// We always set the gauge to 1 or 0, so we don't need to worry about the previous value. There should be minimal
	// overhead in setting the gauge to the same value.
	updateStackFailureMetrics(newStack)
	updateStackReconcilingMetrics(newStack)
}

func updateStackFailureMetrics(newStack *pulumiv1.Stack) {
	if newStack.Status.LastUpdate == nil {
		return
	}

	switch newStack.Status.LastUpdate.State {
	case shared.FailedStackStateMessage:
		numStacksFailing.With(prometheus.Labels{"namespace": newStack.Namespace, "name": newStack.Name}).Set(1)
	case shared.SucceededStackStateMessage:
		numStacksFailing.With(prometheus.Labels{"namespace": newStack.Namespace, "name": newStack.Name}).Set(0)
	}
}

func updateStackReconcilingMetrics(newStack *pulumiv1.Stack) {
	switch apimeta.IsStatusConditionTrue(newStack.Status.Conditions, pulumiv1.ReconcilingCondition) {
	case true:
		numStacksReconciling.With(prometheus.Labels{"namespace": newStack.Namespace, "name": newStack.Name}).Set(1)
	case false:
		numStacksReconciling.With(prometheus.Labels{"namespace": newStack.Namespace, "name": newStack.Name}).Set(0)
	}
}

// deleteStackCallback is a callback that is called when a Stack object is deleted.
func deleteStackCallback(oldObj any) {
	numStacks.Dec()
	val, err := getGaugeValue(numStacks)
	if err == nil && val < 0 {
		numStacks.Set(0)
	}

	oldStack, ok := oldObj.(*pulumiv1.Stack)
	if !ok {
		return
	}

	// Reset any gauge metrics associated with the old stack.
	numStacksFailing.With(prometheus.Labels{"namespace": oldStack.Namespace, "name": oldStack.Name}).Set(0)
	numStacksReconciling.With(prometheus.Labels{"namespace": oldStack.Namespace, "name": oldStack.Name}).Set(0)
}
