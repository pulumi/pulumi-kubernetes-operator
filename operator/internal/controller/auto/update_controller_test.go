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

package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	agentpb "github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/proto"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
)

var _ = Describe("Update Controller", func() {
	var r *UpdateReconciler
	var objName types.NamespacedName
	var obj *autov1alpha1.Update
	// var progressing, complete, failed *metav1.Condition

	BeforeEach(func(ctx context.Context) {
		objName = types.NamespacedName{
			Name:      fmt.Sprintf("update-%s", utilrand.String(8)),
			Namespace: "default",
		}
		obj = &autov1alpha1.Update{
			ObjectMeta: metav1.ObjectMeta{
				Name:      objName.Name,
				Namespace: objName.Namespace,
			},
		}
	})

	JustBeforeEach(func(ctx context.Context) {
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func(ctx context.Context) {
			_ = k8sClient.Delete(ctx, obj)
		})
		r = &UpdateReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	})

	reconcileF := func(ctx context.Context) (result reconcile.Result, err error) {
		result, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: objName})

		// refresh the object and find its status condition(s)
		obj = &autov1alpha1.Update{}
		_ = k8sClient.Get(ctx, objName, obj)

		// progressing = meta.FindStatusCondition(obj.Status.Conditions, autov1alpha1.UpdateConditionTypeProgressing)
		// failed = meta.FindStatusCondition(obj.Status.Conditions, autov1alpha1.UpdateConditionTypeFailed)
		// complete = meta.FindStatusCondition(obj.Status.Conditions, autov1alpha1.UpdateConditionTypeComplete)

		return
	}

	Describe("Retention", func() {
		JustBeforeEach(func(ctx context.Context) {
			// make the update be in a completed state to test the retention logic.
			completedAt := metav1.NewTime(time.Now().Add(-5 * time.Minute))
			obj.Status.ObservedGeneration = obj.Generation
			obj.Status.Conditions = []metav1.Condition{
				{Type: autov1alpha1.UpdateConditionTypeProgressing, Status: metav1.ConditionFalse, Reason: "Test", LastTransitionTime: completedAt},
				{Type: autov1alpha1.UpdateConditionTypeComplete, Status: metav1.ConditionTrue, Reason: "Test", LastTransitionTime: completedAt},
				{Type: autov1alpha1.UpdateConditionTypeFailed, Status: metav1.ConditionFalse, Reason: "Test", LastTransitionTime: completedAt},
			}
			Expect(k8sClient.Status().Update(ctx, obj)).To(Succeed())
		})

		DescribeTableSubtree("TtlAfterCompleted",
			func(ttl *metav1.Duration, expectDeletion bool) {
				BeforeEach(func() {
					obj.Spec.TtlAfterCompleted = ttl
				})
				It("reconciles", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					if expectDeletion {
						Expect(obj.UID).To(BeEmpty())
					} else {
						Expect(obj.UID).ToNot(BeEmpty())
					}
				})
			},
			Entry("TTL is not set", nil, false),
			Entry("TTL has not expired", &metav1.Duration{Duration: 1 * time.Hour}, false),
			Entry("TTL has expired", &metav1.Duration{Duration: 1 * time.Minute}, true),
		)
	})
})

func TestUpdate(t *testing.T) {
	tests := []struct {
		name    string
		obj     autov1alpha1.Update
		client  func(*gomock.Controller) upper
		kclient func(*gomock.Controller) creater

		want    autov1alpha1.UpdateStatus
		wantErr string
	}{
		{
			name: "update success result with outputs",
			obj:  autov1alpha1.Update{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "uid"}},
			client: func(ctrl *gomock.Controller) upper {
				upper := NewMockupper(ctrl)
				recver := NewMockrecver[agentpb.UpStream](ctrl)

				result := &agentpb.UpStream_Result{Result: &agentpb.UpResult{
					Summary: &agentpb.UpdateSummary{Result: "succeeded"},
					Outputs: map[string]*agentpb.OutputValue{
						"username":        {Value: []byte("username")},
						"password":        {Value: []byte("hunter2"), Secret: true},
						"with whitespace": {Value: []byte("with whitespace"), Secret: true},
					},
				}}

				gomock.InOrder(
					upper.EXPECT().
						Up(gomock.Any(), protoMatcher{&agentpb.UpRequest{}}, grpc.WaitForReady(true)).
						Return(recver, nil),
					recver.EXPECT().
						Recv().
						Return(&agentpb.UpStream{Response: &agentpb.UpStream_Event{}}, nil),
					recver.EXPECT().Recv().Return(&agentpb.UpStream{Response: result}, nil),
					recver.EXPECT().CloseSend().Return(nil),
				)
				return upper
			},
			kclient: func(ctrl *gomock.Controller) creater {
				want := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo-stack-outputs",
						Annotations: map[string]string{
							"pulumi.com/secrets": `["password","with_whitespace"]`,
						},
						OwnerReferences: []metav1.OwnerReference{{UID: "uid", Name: "foo"}},
					},
					Data: map[string][]byte{
						"username":        []byte("username"),
						"password":        []byte("hunter2"),
						"with_whitespace": []byte("with whitespace"),
					},
					Immutable: ptr.To(true),
				}
				c := NewMockcreater(ctrl)
				c.EXPECT().Create(gomock.Any(), want).Return(nil)
				return c
			},
			want: autov1alpha1.UpdateStatus{
				Outputs:   "foo-stack-outputs",
				StartTime: metav1.NewTime(time.Unix(0, 0).UTC()),
				EndTime:   metav1.NewTime(time.Unix(0, 0).UTC()),
				Conditions: []metav1.Condition{
					{Type: "Progressing", Status: "False", Reason: "Complete"},
					{Type: "Failed", Status: "False", Reason: "UpdateSucceeded"},
					{Type: "Complete", Status: "True", Reason: "Updated"},
				},
			},
		},
		{
			// Auto API failures are currently returned as non-nil errors, but
			// we should also be able to handle explicit "failed" results.
			name: "update failed result",
			obj:  autov1alpha1.Update{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "uid"}},
			client: func(ctrl *gomock.Controller) upper {
				upper := NewMockupper(ctrl)
				recver := NewMockrecver[agentpb.UpStream](ctrl)

				result := &agentpb.UpStream_Result{Result: &agentpb.UpResult{
					Summary: &agentpb.UpdateSummary{
						Result:  "failed",
						Message: "something went wrong",
					},
				}}

				gomock.InOrder(
					upper.EXPECT().
						Up(gomock.Any(), protoMatcher{&agentpb.UpRequest{}}, grpc.WaitForReady(true)).
						Return(recver, nil),
					recver.EXPECT().
						Recv().
						Return(&agentpb.UpStream{Response: &agentpb.UpStream_Event{}}, nil),
					recver.EXPECT().Recv().Return(&agentpb.UpStream{Response: result}, nil),
					recver.EXPECT().CloseSend().Return(nil),
				)
				return upper
			},
			kclient: func(_ *gomock.Controller) creater { return nil },
			want: autov1alpha1.UpdateStatus{
				Message:   "something went wrong",
				StartTime: metav1.NewTime(time.Unix(0, 0).UTC()),
				EndTime:   metav1.NewTime(time.Unix(0, 0).UTC()),
				Conditions: []metav1.Condition{
					{Type: "Progressing", Status: "False", Reason: "Complete"},
					{
						Type:    "Failed",
						Status:  "True",
						Reason:  "UpdateFailed",
						Message: "something went wrong",
					},
					{Type: "Complete", Status: "True", Reason: "Updated"},
				},
			},
		},
		{
			name: "update grpc error",
			obj:  autov1alpha1.Update{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "uid"}},
			client: func(ctrl *gomock.Controller) upper {
				upper := NewMockupper(ctrl)
				recver := NewMockrecver[agentpb.UpStream](ctrl)

				gomock.InOrder(
					upper.EXPECT().
						Up(gomock.Any(), protoMatcher{&agentpb.UpRequest{}}, grpc.WaitForReady(true)).
						Return(recver, nil),
					recver.EXPECT().
						Recv().
						Return(nil, fmt.Errorf("failed to run update: exit status 255")),
					recver.EXPECT().CloseSend().Return(nil),
				)
				return upper
			},
			kclient: func(*gomock.Controller) creater { return nil },
			wantErr: "failed to run update: exit status 255",
		},
		{
			name: "workspace grpc failure",
			obj:  autov1alpha1.Update{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "uid"}},
			client: func(ctrl *gomock.Controller) upper {
				upper := NewMockupper(ctrl)

				gomock.InOrder(
					upper.EXPECT().
						Up(gomock.Any(), protoMatcher{&agentpb.UpRequest{}}, grpc.WaitForReady(true)).
						Return(nil, status.Error(codes.Unavailable, "transient workspace error")),
				)
				return upper
			},
			kclient: func(*gomock.Controller) creater { return nil },
			wantErr: "transient workspace error",
		},
		{
			name: "response stream grpc failure",
			obj:  autov1alpha1.Update{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "uid"}},
			client: func(ctrl *gomock.Controller) upper {
				upper := NewMockupper(ctrl)
				recver := NewMockrecver[agentpb.UpStream](ctrl)

				gomock.InOrder(
					upper.EXPECT().
						Up(gomock.Any(), protoMatcher{&agentpb.UpRequest{}}, grpc.WaitForReady(true)).
						Return(recver, nil),
					recver.EXPECT().
						Recv().
						Return(nil, status.Error(codes.Unavailable, "transient stream error")),
					recver.EXPECT().CloseSend().Return(nil),
				)
				return upper
			},
			kclient: func(*gomock.Controller) creater { return nil },
			wantErr: "transient stream error",
		},
		{
			name: "surfacing pulumi cli failures: stack locked",
			obj:  autov1alpha1.Update{ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "uid"}},
			client: func(ctrl *gomock.Controller) upper {
				upper := NewMockupper(ctrl)
				recver := NewMockrecver[agentpb.UpStream](ctrl)

				st := status.New(codes.Unknown, "up failed")
				d, _ := st.WithDetails(&agentpb.PulumiErrorInfo{
					Message: "Another update is currently in progress",
					Reason:  "UpdateConflict",
					Code:    409,
				})

				gomock.InOrder(
					upper.EXPECT().
						Up(gomock.Any(), protoMatcher{&agentpb.UpRequest{}}, grpc.WaitForReady(true)).
						Return(recver, nil),
					recver.EXPECT().
						Recv().
						Return(nil, d.Err()),
					recver.EXPECT().CloseSend().Return(nil),
				)
				return upper
			},
			kclient: func(_ *gomock.Controller) creater { return nil },
			want: autov1alpha1.UpdateStatus{
				Message:   "Another u",
				StartTime: metav1.NewTime(time.Unix(0, 0).UTC()),
				EndTime:   metav1.NewTime(time.Unix(0, 0).UTC()),
				Conditions: []metav1.Condition{
					{Type: "Progressing", Status: "False", Reason: "Complete"},
					{
						Type:    "Failed",
						Status:  "True",
						Reason:  "UpdateConflict",
						Message: "Another update is currently in progress",
					},
					{Type: "Complete", Status: "True", Reason: "Updated"},
				},
			},
			wantErr: "up failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			scheme.Scheme.AddKnownTypes(
				schema.GroupVersion{Group: "auto.pulumi.com", Version: "v1alpha1"},
				&autov1alpha1.Update{},
			)
			builder := fake.NewClientBuilder().
				WithObjects(&tt.obj).
				WithStatusSubresource(&tt.obj)
			rs := newReconcileSession(builder.Build(), &tt.obj)

			ctrl := gomock.NewController(t)
			_, err := rs.Update(
				ctx,
				&tt.obj,
				tt.client(ctrl),
				tt.kclient(ctrl),
			)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				//TODO(rquitales): Also check the return statues!
				return
			}
			assert.NoError(t, err)

			var res autov1alpha1.Update
			require.NoError(
				t,
				rs.client.Get(
					ctx,
					types.NamespacedName{Namespace: tt.obj.Namespace, Name: tt.obj.Name},
					&res,
				),
			)
			assert.EqualExportedValues(t, tt.want, res.Status)
		})
	}
}

type protoMatcher struct {
	proto.Message
}

func (pm protoMatcher) Matches(v any) bool {
	msg, ok := v.(proto.Message)
	if !ok {
		return false
	}
	return proto.Equal(pm.Message, msg)
}

func (pm protoMatcher) String() string {
	return fmt.Sprintf("%+v", pm.Message)
}
