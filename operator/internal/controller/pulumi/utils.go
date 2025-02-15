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
	dto "github.com/prometheus/client_model/go"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
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

func emitEvent(recorder record.EventRecorder, object runtime.Object, event pulumiv1.StackEvent, messageFmt string, args ...interface{}) {
	recorder.Eventf(object, event.EventType(), event.Reason(), messageFmt, args...)
}
