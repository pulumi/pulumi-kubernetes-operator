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
	"go.uber.org/mock/gomock"
	grpc "google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	agentpb "github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/proto"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/auto/v1alpha1"
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

		want autov1alpha1.UpdateStatus
	}{
		{
			name: "with outputs",
			obj: autov1alpha1.Update{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "uid"},
				Spec:       autov1alpha1.UpdateSpec{},
				Status:     autov1alpha1.UpdateStatus{},
			},
			client: func(ctrl *gomock.Controller) upper {
				upper := NewMockupper(ctrl)
				recver := NewMockrecver[agentpb.UpStream](ctrl)

				result := &agentpb.UpStream_Result{Result: &agentpb.UpResult{
					Summary: &agentpb.UpdateSummary{},
					Outputs: map[string]*agentpb.OutputValue{
						"username": {Value: []byte("username")},
						"password": {Value: []byte("hunter2"), Secret: true},
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
							"pulumi.com/secrets": `["password"]`,
						},
						OwnerReferences: []metav1.OwnerReference{{UID: "uid", Name: "foo"}},
					},
					Data: map[string][]byte{
						"username": []byte("username"),
						"password": []byte("hunter2"),
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
			},
		},
		{
			name: "update failure",
			obj: autov1alpha1.Update{
				ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: "uid"},
				Spec:       autov1alpha1.UpdateSpec{},
				Status:     autov1alpha1.UpdateStatus{},
			},
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
			want:    autov1alpha1.UpdateStatus{Message: "failed to run update: exit status 255"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &reconcileSession{
				progressing:  &metav1.Condition{},
				complete:     &metav1.Condition{},
				failed:       &metav1.Condition{},
				updateStatus: func() error { return nil },
			}
			ctrl := gomock.NewController(t)
			_, err := u.Update(
				context.Background(),
				&tt.obj,
				tt.client(ctrl),
				tt.kclient(ctrl),
			)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, tt.obj.Status)
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
