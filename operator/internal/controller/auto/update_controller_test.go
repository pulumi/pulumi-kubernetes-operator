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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	agentpb "github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/proto"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
)

var _ = PDescribe("Update Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		update := &autov1alpha1.Update{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Update")
			err := k8sClient.Get(ctx, typeNamespacedName, update)
			if err != nil && errors.IsNotFound(err) {
				resource := &autov1alpha1.Update{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &autov1alpha1.Update{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Update")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &UpdateReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
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
