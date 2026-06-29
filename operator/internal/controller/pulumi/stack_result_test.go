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

	"github.com/stretchr/testify/assert"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
)

func TestMarkStackResult(t *testing.T) {
	tests := []struct {
		name       string
		lastUpdate shared.StackUpdateState
		wantReady  metav1.ConditionStatus
		wantCond   string // condition expected to carry the distinguishing reason
		wantReason string
	}{
		{
			name:       "successful up is Ready",
			lastUpdate: shared.StackUpdateState{Type: autov1alpha1.UpType, State: shared.SucceededStackStateMessage},
			wantReady:  metav1.ConditionTrue,
			wantCond:   pulumiv1.ReadyCondition,
			wantReason: pulumiv1.ReadyCompletedReason,
		},
		{
			name:       "successful preview is Ready",
			lastUpdate: shared.StackUpdateState{Type: autov1alpha1.PreviewType, State: shared.SucceededStackStateMessage},
			wantReady:  metav1.ConditionTrue,
			wantCond:   pulumiv1.ReadyCondition,
			wantReason: pulumiv1.ReadyCompletedReason,
		},
		{
			name:       "transient up failure stays Reconciling",
			lastUpdate: shared.StackUpdateState{Type: autov1alpha1.UpType, State: shared.FailedStackStateMessage, Failures: maxUpdateFailures - 1},
			wantReady:  metav1.ConditionFalse,
			wantCond:   pulumiv1.ReconcilingCondition,
			wantReason: pulumiv1.ReconcilingRetryReason,
		},
		{
			name:       "transient preview failure is reported distinctly",
			lastUpdate: shared.StackUpdateState{Type: autov1alpha1.PreviewType, State: shared.FailedStackStateMessage, Failures: 1},
			wantReady:  metav1.ConditionFalse,
			wantCond:   pulumiv1.ReconcilingCondition,
			wantReason: pulumiv1.ReconcilingPreviewFailedReason,
		},
		{
			name:       "persistent up failure stalls",
			lastUpdate: shared.StackUpdateState{Type: autov1alpha1.UpType, State: shared.FailedStackStateMessage, Failures: maxUpdateFailures},
			wantReady:  metav1.ConditionFalse,
			wantCond:   pulumiv1.StalledCondition,
			wantReason: pulumiv1.StalledUpdateFailedReason,
		},
		{
			name:       "persistent preview failure stalls",
			lastUpdate: shared.StackUpdateState{Type: autov1alpha1.PreviewType, State: shared.FailedStackStateMessage, Failures: maxUpdateFailures + 5},
			wantReady:  metav1.ConditionFalse,
			wantCond:   pulumiv1.StalledCondition,
			wantReason: pulumiv1.StalledUpdateFailedReason,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &pulumiv1.StackStatus{LastUpdate: &tt.lastUpdate}
			markStackResult(status)

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
