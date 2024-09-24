// Copyright 2021, Pulumi Corporation.  All rights reserved.

package pulumi

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	numPrograms = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "programs_active",
		Help: "Number of Program objects currently tracked by the Pulumi Kubernetes Operator",
	})
)

// init registers Program custom metrics with the global prometheus registry on startup.
func init() {
	metrics.Registry.MustRegister(numPrograms)
}

func newProgramCallback(_ any) {
	numPrograms.Inc()
}

func deleteProgramCallback(_ any) {
	numStacks.Dec()
}
