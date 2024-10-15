/*
Copyright 2024.

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

package pulumi

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func exactlyOneOf(these ...bool) bool {
	var found bool
	for _, b := range these {
		if found && b {
			return false
		}
		found = found || b
	}
	return found
}

// getGaugeValue returns the value of a gauge metric. This is useful to check that a gauge
// does not go into negative values.
func getGaugeValue(metric prometheus.Gauge) (float64, error) {
	var m = &dto.Metric{}
	if err := metric.Write(m); err != nil {
		return 0, err
	}
	return m.Gauge.GetValue(), nil
}

// FinalizerAddedPredicate detects when a finalizer is added to an object.
// It is used to suppress reconciliation when the stack controller adds its finalizer, which causes
// a generation change that would otherwise trigger reconciliation.
type FinalizerAddedPredicate struct {
	predicate.Funcs
}

func (p *FinalizerAddedPredicate) Create(e event.CreateEvent) bool {
	return false
}

func (p *FinalizerAddedPredicate) Delete(e event.DeleteEvent) bool {
	return false
}

func (p *FinalizerAddedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	return !controllerutil.ContainsFinalizer(e.ObjectOld, pulumiFinalizer) && controllerutil.ContainsFinalizer(e.ObjectNew, pulumiFinalizer)
}

func (p *FinalizerAddedPredicate) Generic(e event.GenericEvent) bool {
	return false
}
