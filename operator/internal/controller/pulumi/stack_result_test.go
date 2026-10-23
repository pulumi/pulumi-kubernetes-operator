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
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
)

func TestMarkStackResult(t *testing.T) {
	tests := []struct {
		name        string
		lastUpdate  shared.StackUpdateState
		maxFailures int64
		wantReady   metav1.ConditionStatus
		wantCond    string // condition expected to carry the distinguishing reason
		wantReason  string
	}{
		{
			name:        "successful up is Ready",
			lastUpdate:  shared.StackUpdateState{Type: autov1alpha1.UpType, State: shared.SucceededStackStateMessage},
			maxFailures: defaultMaxUpdateFailures,
			wantReady:   metav1.ConditionTrue,
			wantCond:    pulumiv1.ReadyCondition,
			wantReason:  pulumiv1.ReadyCompletedReason,
		},
		{
			name:        "successful preview is Ready",
			lastUpdate:  shared.StackUpdateState{Type: autov1alpha1.PreviewType, State: shared.SucceededStackStateMessage},
			maxFailures: defaultMaxUpdateFailures,
			wantReady:   metav1.ConditionTrue,
			wantCond:    pulumiv1.ReadyCondition,
			wantReason:  pulumiv1.ReadyCompletedReason,
		},
		{
			name:        "transient up failure stays Reconciling",
			lastUpdate:  shared.StackUpdateState{Type: autov1alpha1.UpType, State: shared.FailedStackStateMessage, Failures: defaultMaxUpdateFailures - 1},
			maxFailures: defaultMaxUpdateFailures,
			wantReady:   metav1.ConditionFalse,
			wantCond:    pulumiv1.ReconcilingCondition,
			wantReason:  pulumiv1.ReconcilingRetryReason,
		},
		{
			name:        "transient preview failure is reported distinctly",
			lastUpdate:  shared.StackUpdateState{Type: autov1alpha1.PreviewType, State: shared.FailedStackStateMessage, Failures: 1},
			maxFailures: defaultMaxUpdateFailures,
			wantReady:   metav1.ConditionFalse,
			wantCond:    pulumiv1.ReconcilingCondition,
			wantReason:  pulumiv1.ReconcilingPreviewFailedReason,
		},
		{
			name:        "persistent up failure stalls",
			lastUpdate:  shared.StackUpdateState{Type: autov1alpha1.UpType, State: shared.FailedStackStateMessage, Failures: defaultMaxUpdateFailures},
			maxFailures: defaultMaxUpdateFailures,
			wantReady:   metav1.ConditionFalse,
			wantCond:    pulumiv1.StalledCondition,
			wantReason:  pulumiv1.StalledUpdateFailedReason,
		},
		{
			name:        "persistent preview failure stalls",
			lastUpdate:  shared.StackUpdateState{Type: autov1alpha1.PreviewType, State: shared.FailedStackStateMessage, Failures: defaultMaxUpdateFailures + 5},
			maxFailures: defaultMaxUpdateFailures,
			wantReady:   metav1.ConditionFalse,
			wantCond:    pulumiv1.StalledCondition,
			wantReason:  pulumiv1.StalledUpdateFailedReason,
		},
		{
			name:        "configured failure limit stalls",
			lastUpdate:  shared.StackUpdateState{Type: autov1alpha1.UpType, State: shared.FailedStackStateMessage, Failures: 2},
			maxFailures: 2,
			wantReady:   metav1.ConditionFalse,
			wantCond:    pulumiv1.StalledCondition,
			wantReason:  pulumiv1.StalledUpdateFailedReason,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &pulumiv1.StackStatus{LastUpdate: &tt.lastUpdate}
			markStackResult(status, tt.maxFailures)

			ready := apimeta.FindStatusCondition(status.Conditions, pulumiv1.ReadyCondition)
			if assert.NotNil(t, ready, "Ready condition must be set") {
				assert.Equal(t, tt.wantReady, ready.Status)
			}

			cond := apimeta.FindStatusCondition(status.Conditions, tt.wantCond)
			if assert.NotNil(t, cond, "expected %s condition to be set", tt.wantCond) {
				assert.Equal(t, tt.wantReason, cond.Reason)
			}
		})
	}
}

func TestIsSyncedHonorsMaxUpdateFailures(t *testing.T) {
	configuredLimit := int64(2)
	stack := &pulumiv1.Stack{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
		Spec:       shared.StackSpec{MaxUpdateFailures: &configuredLimit},
		Status: pulumiv1.StackStatus{LastUpdate: &shared.StackUpdateState{
			Generation:          1,
			State:               shared.FailedStackStateMessage,
			Failures:            configuredLimit,
			LastAttemptedCommit: "commit",
			LastResyncTime:      metav1.NewTime(time.Now().Add(-time.Hour)),
		}},
	}

	assert.Equal(t, configuredLimit, maxUpdateFailures(stack))
	synced, _ := isSynced(logr.Discard(), record.NewFakeRecorder(1), stack, "commit")
	assert.True(t, synced, "configured retry limit should stop retries")

	stack.Spec.MaxUpdateFailures = nil
	assert.Equal(t, defaultMaxUpdateFailures, maxUpdateFailures(stack))
	synced, _ = isSynced(logr.Discard(), record.NewFakeRecorder(1), stack, "commit")
	assert.False(t, synced, "omitting the limit should retain the default of three retries")
}
