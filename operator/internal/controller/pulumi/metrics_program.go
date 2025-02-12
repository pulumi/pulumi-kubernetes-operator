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

// newProgramCallback is a callback that is called when a new Program object is created.
func newProgramCallback(_ any) {
	numPrograms.Inc()
}

// updateProgramCallback is a callback that is called when a Program object is updated.
func deleteProgramCallback(_ any) {
	numPrograms.Dec()

	val, err := getGaugeValue(numPrograms)
	if err == nil && val < 0 {
		numPrograms.Set(0)
	}
}
