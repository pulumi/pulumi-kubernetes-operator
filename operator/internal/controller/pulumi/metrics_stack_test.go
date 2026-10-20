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
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func cleanUpStackMetrics(t *testing.T) {
	t.Helper()

	t.Cleanup(numStacks.Reset)
	t.Cleanup(numStacksFailing.Reset)
	t.Cleanup(numStacksReconciling.Reset)
}

func stackIn(namespace, name string) *pulumiv1.Stack {
	return &pulumiv1.Stack{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
	}
}

func TestStacksActiveTracksCreatesAndDeletes(t *testing.T) {
	cleanUpStackMetrics(t)

	stack := stackIn("test", "test")
	active := func() float64 {
		return testutil.ToFloat64(numStacks.WithLabelValues(stack.Namespace))
	}

	newStackCallback(stack)
	require.Equal(t, 1.0, active())

	newStackCallback(stack)
	require.Equal(t, 2.0, active())

	deleteStackCallback(stack)
	require.Equal(t, 1.0, active())

	deleteStackCallback(stack)
	require.Equal(t, 0.0, active())

	deleteStackCallback(stack)
	require.Equal(t, 0.0, active(), "the gauge must not go negative")
}

func TestStacksActiveIsCountedPerNamespace(t *testing.T) {
	cleanUpStackMetrics(t)

	newStackCallback(stackIn("alpha", "one"))
	newStackCallback(stackIn("alpha", "two"))
	newStackCallback(stackIn("beta", "one"))

	require.Equal(t, 2.0, testutil.ToFloat64(numStacks.WithLabelValues("alpha")))
	require.Equal(t, 1.0, testutil.ToFloat64(numStacks.WithLabelValues("beta")))

	deleteStackCallback(stackIn("alpha", "two"))

	require.Equal(t, 1.0, testutil.ToFloat64(numStacks.WithLabelValues("alpha")))
	require.Equal(t, 1.0, testutil.ToFloat64(numStacks.WithLabelValues("beta")))
}

func TestDeleteStackCallbackUnwrapsATombstone(t *testing.T) {
	cleanUpStackMetrics(t)

	stack := stackIn("alpha", "one")
	newStackCallback(stack)
	numStacksFailing.WithLabelValues(stack.Namespace, stack.Name).Set(1)
	numStacksReconciling.WithLabelValues(stack.Namespace, stack.Name).Set(1)

	deleteStackCallback(cache.DeletedFinalStateUnknown{Key: "alpha/one", Obj: stack})

	require.Equal(t, 0.0, testutil.ToFloat64(numStacks.WithLabelValues(stack.Namespace)))
	require.Equal(t, 0.0, testutil.ToFloat64(numStacksFailing.WithLabelValues(stack.Namespace, stack.Name)))
	require.Equal(t, 0.0, testutil.ToFloat64(numStacksReconciling.WithLabelValues(stack.Namespace, stack.Name)))
}

func TestStacksFailingFollowsLastUpdateState(t *testing.T) {
	cleanUpStackMetrics(t)

	stack := stackIn("test", "test")
	failing := func() float64 {
		return testutil.ToFloat64(numStacksFailing.WithLabelValues(stack.Namespace, stack.Name))
	}

	stack.Status.MarkStalledCondition("reason", "message")
	stack.Status.LastUpdate = &shared.StackUpdateState{State: shared.FailedStackStateMessage}
	updateStackCallback(nil, stack)
	require.Equal(t, 1.0, failing())

	stack.Status.MarkReadyCondition()
	stack.Status.LastUpdate = &shared.StackUpdateState{State: shared.SucceededStackStateMessage}
	updateStackCallback(nil, stack)
	require.Equal(t, 0.0, failing())
}

func TestStacksReconcilingFollowsReconcilingCondition(t *testing.T) {
	cleanUpStackMetrics(t)

	stack := stackIn("test", "test")
	reconciling := func() float64 {
		return testutil.ToFloat64(numStacksReconciling.WithLabelValues(stack.Namespace, stack.Name))
	}

	stack.Status.MarkReconcilingCondition("reason", "message")
	updateStackCallback(nil, stack)
	require.Equal(t, 1.0, reconciling())

	stack.Status.MarkReadyCondition()
	updateStackCallback(nil, stack)
	require.Equal(t, 0.0, reconciling())
}

func TestDeleteStackCallbackResetsPerStackMetrics(t *testing.T) {
	cleanUpStackMetrics(t)

	stack := stackIn("test", "test")
	stack.Status.LastUpdate = &shared.StackUpdateState{State: shared.SucceededStackStateMessage}

	numStacks.WithLabelValues(stack.Namespace).Set(1)
	numStacksFailing.WithLabelValues(stack.Namespace, stack.Name).Set(1)
	numStacksReconciling.WithLabelValues(stack.Namespace, stack.Name).Set(1)

	deleteStackCallback(stack)

	require.Equal(t, 0.0, testutil.ToFloat64(numStacks.WithLabelValues(stack.Namespace)))
	require.Equal(t, 0.0, testutil.ToFloat64(numStacksFailing.WithLabelValues(stack.Namespace, stack.Name)))
	require.Equal(t, 0.0, testutil.ToFloat64(numStacksReconciling.WithLabelValues(stack.Namespace, stack.Name)))
}
