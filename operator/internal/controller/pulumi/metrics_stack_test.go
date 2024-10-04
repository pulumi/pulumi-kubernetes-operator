// Copyright 2016-2024, Pulumi Corporation.
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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Stack Metrics", func() {
	var (
		oldStack *pulumiv1.Stack
		newStack *pulumiv1.Stack
	)

	Context("when a new stack is created", Ordered, func() {
		BeforeAll(func() {
			// Reset the metrics
			numStacks.Set(0)
			numStacksFailing.Reset()
			numStacksReconciling.Reset()

			// Create a new stack object
			newStack = &pulumiv1.Stack{}
		})

		It("should increment the numStacks metric", func() {
			newStackCallback(newStack)

			// Check if the numStacks metric has been incremented
			expected := 1.0
			actual := testutil.ToFloat64(numStacks)
			Expect(actual).To(Equal(expected))
		})

		It("should increment the numStacks metric again if another stack is created", func() {
			newStackCallback(newStack)

			// Check if the numStacks metric has been incremented
			expected := 2.0
			actual := testutil.ToFloat64(numStacks)
			Expect(actual).To(Equal(expected))
		})

		It("should decrement the numStacks metric if a stack is deleted", func() {
			deleteStackCallback(newStack)

			// Check if the numStacks metric has been decremented
			expected := 1.0
			actual := testutil.ToFloat64(numStacks)
			Expect(actual).To(Equal(expected))
		})

		It("should decrement the numStacks metric again if another stack is deleted", func() {
			deleteStackCallback(newStack)

			// Check if the numStacks metric has been decremented
			expected := 0.0
			actual := testutil.ToFloat64(numStacks)
			Expect(actual).To(Equal(expected))
		})

		It("should not decrement the numStacks metric if a stack is deleted and the metric is already at 0", func() {
			deleteStackCallback(newStack)

			// Check if the numStacks metric has been decremented
			expected := 0.0
			actual := testutil.ToFloat64(numStacks)
			Expect(actual).To(Equal(expected))
		})
	})

	Context("when a stack is updated", func() {
		BeforeEach(func() {
			// Create old and new stack objects for updateStackCallback. The new stack should be in a reconciling state.
			oldStack = &pulumiv1.Stack{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "test",
				},
			}
			newStack = &pulumiv1.Stack{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "test",
				},
			}
			numStacks.Set(0)
			numStacksFailing.Reset()
			numStacksReconciling.Reset()
		})

		Describe("when testing the stacks_failing metric", Ordered, func() {
			BeforeAll(func() {
				// Set the new stack to a failed state
				newStack.Status.MarkStalledCondition("reason", "message")
				newStack.Status.LastUpdate = &shared.StackUpdateState{
					State: shared.FailedStackStateMessage,
				}

				oldStack.Status.MarkReadyCondition()
			})

			It("should update the numStacksFailing metric to 1 when the stack update fails", func() {
				// Call the updateStackCallback function
				updateStackCallback(oldStack, newStack)

				// Check if the numStacksFailing metric has been updated
				expectedFailing := 1.0
				actualFailing := testutil.ToFloat64(numStacksFailing.With(prometheus.Labels{"namespace": oldStack.Namespace, "name": oldStack.Name}))
				Expect(actualFailing).To(Equal(expectedFailing))
			})

			It("should reset the numStacksFailing metric when the stack update succeeds", func() {
				// Update the stack objects to be in a succeeded state.
				newStack.Status.MarkReadyCondition()
				newStack.Status.LastUpdate = &shared.StackUpdateState{
					State: shared.SucceededStackStateMessage,
				}
				oldStack.Status.MarkStalledCondition("reason", "message")
				oldStack.Status.LastUpdate = &shared.StackUpdateState{
					State: shared.FailedStackStateMessage,
				}
				// Call the updateStackCallback function
				updateStackCallback(oldStack, newStack)

				// Check if the numStacksFailing metric has been updated
				expectedFailing := 0.0
				actualFailing := testutil.ToFloat64(numStacksFailing.With(prometheus.Labels{"namespace": oldStack.Namespace, "name": oldStack.Name}))
				Expect(actualFailing).To(Equal(expectedFailing))
			})
		})

		Describe("when testing the stacks_reconciling metric", Ordered, func() {
			BeforeAll(func() {
				// Set the new stack to a failed state
				newStack.Status.MarkReconcilingCondition("reason", "message")
				oldStack.Status.MarkReadyCondition()
			})

			It("should update the numStackReconciling metric to 1 when the stack is reconciling", func() {
				// Call the updateStackCallback function
				updateStackCallback(oldStack, newStack)

				// Check if the numStacksReconciling metric has been updated
				expectedReconciling := 1.0
				actualReconciling := testutil.ToFloat64(numStacksReconciling.With(prometheus.Labels{"namespace": oldStack.Namespace, "name": oldStack.Name}))
				Expect(actualReconciling).To(Equal(expectedReconciling))
			})

			It("should reset the numStackReconciling metric when the stack is finished reconciling", func() {
				// Update the stack objects to be in a succeeded state.
				newStack.Status.MarkReadyCondition()
				oldStack.Status.MarkReconcilingCondition("reason", "message")
				// Call the updateStackCallback function
				updateStackCallback(oldStack, newStack)

				// Check if the numStacksReconciling metric has been updated
				expectedReconciling := 0.0
				actualReconciling := testutil.ToFloat64(numStacksReconciling.With(prometheus.Labels{"namespace": oldStack.Namespace, "name": oldStack.Name}))
				Expect(actualReconciling).To(Equal(expectedReconciling))
			})
		})
	})

	Context("when a stack is deleted", func() {
		BeforeEach(func() {
			// Set the metrics
			numStacks.Set(1)
			numStacksFailing.With(prometheus.Labels{"namespace": "test", "name": "test"}).Set(1)
			numStacksReconciling.With(prometheus.Labels{"namespace": "test", "name": "test"}).Set(1)

			// Create an old stack object for deleteStackCallback
			oldStack = &pulumiv1.Stack{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "test",
				},
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						State: shared.SucceededStackStateMessage,
					},
				},
			}
		})

		It("should decrement the numStacks metric and reset the numStacksFailing and numStacksReconciling metrics", func() {
			// Call the deleteStackCallback function
			deleteStackCallback(oldStack)

			// Check if the numStacks metric has been decremented
			expected := 0.0
			actual := testutil.ToFloat64(numStacks)
			Expect(actual).To(Equal(expected))

			// Check if the numStacksFailing metric has been decremented
			expectedFailing := 0.0
			actualFailing := testutil.ToFloat64(numStacksFailing.With(prometheus.Labels{"namespace": oldStack.Namespace, "name": oldStack.Name}))
			Expect(actualFailing).To(Equal(expectedFailing))

			// Check if the numStacksReconciling metric has been decremented
			expectedReconciling := 0.0
			actualReconciling := testutil.ToFloat64(numStacksReconciling.With(prometheus.Labels{"namespace": oldStack.Namespace, "name": oldStack.Name}))
			Expect(actualReconciling).To(Equal(expectedReconciling))
		})
	})
})
