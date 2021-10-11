// Copyright 2021, Pulumi Corporation.  All rights reserved.

package stack

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	pulumiv1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	numStacks        prometheus.Gauge
	numStacksFailing *prometheus.GaugeVec
)

func initMetrics() []prometheus.Collector {
	var collectors []prometheus.Collector

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

	collectors = append(collectors, numStacks, numStacksFailing)
	return collectors
}

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(initMetrics()...)
}

func newStackCallback(obj interface{}) {
	numStacks.Inc()
}

func updateStackCallback(oldObj, newObj interface{}) {
	oldStack, ok := oldObj.(*pulumiv1alpha1.Stack)
	if !ok {
		return
	}

	newStack, ok := newObj.(*pulumiv1alpha1.Stack)
	if !ok {
		return
	}

	// fresh transition to failure
	if newStack.Status.LastUpdate != nil && newStack.Status.LastUpdate.State == shared.FailedStackStateMessage {
		if oldStack.Status.LastUpdate == nil || oldStack.Status.LastUpdate.State != newStack.Status.LastUpdate.State {
			numStacksFailing.With(prometheus.Labels{"namespace": oldStack.Namespace, "name": oldStack.Name}).Set(1)
		}
	}

	// transition to success from failure
	if newStack.Status.LastUpdate != nil && newStack.Status.LastUpdate.State == shared.SucceededStackStateMessage {
		if oldStack.Status.LastUpdate != nil && oldStack.Status.LastUpdate.State == shared.FailedStackStateMessage {
			numStacksFailing.With(prometheus.Labels{"namespace": oldStack.Namespace, "name": oldStack.Name}).Set(0)
		}
	}
}

func deleteStackCallback(oldObj interface{}) {
	numStacks.Dec()
}
