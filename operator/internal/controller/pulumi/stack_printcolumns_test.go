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
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"

	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
)

var _ = Describe("Stack printer columns", func() {
	var (
		clientset *kubernetes.Clientset
		stack     *pulumiv1.Stack
	)

	BeforeEach(func(ctx context.Context) {
		var err error
		clientset, err = kubernetes.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		stack = &pulumiv1.Stack{
			TypeMeta: metav1.TypeMeta{
				APIVersion: pulumiv1.GroupVersion.String(),
				Kind:       "Stack",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("printcolumns-%s", utilrand.String(8)),
				Namespace: "default",
			},
			Spec: shared.StackSpec{
				Stack: "organization/project/dev",
			},
		}
	})

	AfterEach(func(ctx context.Context) {
		_ = k8sClient.Delete(ctx, stack)
	})

	// The API server renders the printer columns of a CRD server-side, so asking
	// it for the Table representation exercises the columns as kubectl sees them.
	// Unlike kubectl, the Table carries the priority=10 columns too.
	renderCells := func(ctx context.Context) map[string]any {
		raw, err := clientset.Discovery().RESTClient().Get().
			AbsPath(fmt.Sprintf("/apis/%s/namespaces/%s/stacks/%s",
				pulumiv1.GroupVersion.String(), stack.Namespace, stack.Name)).
			SetHeader("Accept", "application/json;as=Table;v=v1;g=meta.k8s.io").
			DoRaw(ctx)
		Expect(err).NotTo(HaveOccurred())

		var table metav1.Table
		Expect(json.Unmarshal(raw, &table)).To(Succeed())
		Expect(table.ColumnDefinitions).NotTo(BeEmpty())
		Expect(table.Rows).To(HaveLen(1))
		Expect(table.Rows[0].Cells).To(HaveLen(len(table.ColumnDefinitions)))

		byName := make(map[string]any, len(table.ColumnDefinitions))
		for i, col := range table.ColumnDefinitions {
			byName[col.Name] = table.Rows[0].Cells[i]
		}
		return byName
	}

	rendered := func(cell any) string {
		if cell == nil {
			return ""
		}
		return fmt.Sprint(cell)
	}

	It("renders a non-empty Age for a stack that has no status yet", func(ctx context.Context) {
		Expect(k8sClient.Create(ctx, stack)).To(Succeed())
		Expect(stack.Status.LastUpdate).To(BeNil())

		cells := renderCells(ctx)
		Expect(cells).To(HaveKey("Age"))
		Expect(rendered(cells["Age"])).NotTo(BeEmpty())
	})

	It("renders a non-empty cell for every declared column once the status is populated", func(ctx context.Context) {
		Expect(k8sClient.Create(ctx, stack)).To(Succeed())

		stack.Status.LastUpdate = &shared.StackUpdateState{
			Name:                "update-1",
			Type:                autov1alpha1.UpType,
			State:               shared.SucceededStackStateMessage,
			LastAttemptedCommit: "2cd4a8b1f0c3a9e7d6b5a4c3f2e1d0b9a8c7f6e5",
			LastResyncTime:      metav1.Now(),
			Permalink:           "https://app.pulumi.com/organization/project/dev/updates/1",
		}
		stack.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				Reason:             "Succeeded",
				Message:            "the stack is up to date",
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               "Reconciling",
				Status:             metav1.ConditionTrue,
				Reason:             "Progressing",
				Message:            "the stack is being reconciled",
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               "Stalled",
				Status:             metav1.ConditionTrue,
				Reason:             "SyncFailure",
				Message:            "the stack is stalled",
				LastTransitionTime: metav1.Now(),
			},
		}
		Expect(k8sClient.Status().Update(ctx, stack)).To(Succeed())

		cells := renderCells(ctx)
		Expect(cells).To(HaveKey("Age"))
		Expect(cells).To(HaveKey("Last Resync"))
		for name, cell := range cells {
			Expect(rendered(cell)).NotTo(BeEmpty(), "column %q rendered an empty cell", name)
		}
	})
})
