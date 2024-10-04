// Copyright 2021, Pulumi Corporation.  All rights reserved.

package pulumi

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
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
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(numStacks, numStacksFailing)
}

// newStackCallback is a callback that is called when a new Stack object is created.
func newStackCallback(obj interface{}) {
	numStacks.Inc()
}

// updateStackCallback is a callback that is called when a Stack object is updated.
func updateStackCallback(oldObj, newObj interface{}) {
	oldStack, ok := oldObj.(*pulumiv1.Stack)
	if !ok {
		return
	}

	newStack, ok := newObj.(*pulumiv1.Stack)
	if !ok {
		return
	}

	// transition to failure
	if newStack.Status.LastUpdate != nil && newStack.Status.LastUpdate.State == shared.FailedStackStateMessage {
		numStacksFailing.With(prometheus.Labels{"namespace": oldStack.Namespace, "name": oldStack.Name}).Set(1)
	}

	// transition to success from failure
	if newStack.Status.LastUpdate != nil && newStack.Status.LastUpdate.State == shared.SucceededStackStateMessage {
		numStacksFailing.With(prometheus.Labels{"namespace": oldStack.Namespace, "name": oldStack.Name}).Set(0)
	}
}

// deleteStackCallback is a callback that is called when a Stack object is deleted.
func deleteStackCallback(oldObj interface{}) {
	numStacks.Dec()
	oldStack, ok := oldObj.(*pulumiv1.Stack)
	if !ok {
		return
	}
	// assume that if there was a status recorded, this gauge exists
	if oldStack.Status.LastUpdate != nil {
		numStacksFailing.With(prometheus.Labels{"namespace": oldStack.Namespace, "name": oldStack.Name}).Set(0)
	}
}
