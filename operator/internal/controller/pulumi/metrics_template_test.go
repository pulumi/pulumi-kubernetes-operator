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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	pulumiv1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Template Metrics", func() {
	var (
		oldTemplate *pulumiv1alpha1.Template
		newTemplate *pulumiv1alpha1.Template
	)

	Context("when a new template is created", Ordered, func() {
		BeforeAll(func() {
			// Reset the metrics
			numTemplates.Set(0)
			numTemplatesFailing.Reset()
			numTemplatesReconciling.Reset()
			numTemplateInstances.Reset()

			// Create a new template object
			newTemplate = &pulumiv1alpha1.Template{}
		})

		It("should increment the numTemplates metric", func() {
			newTemplateCallback(newTemplate)

			// Check if the numTemplates metric has been incremented
			expected := 1.0
			actual := testutil.ToFloat64(numTemplates)
			Expect(actual).To(Equal(expected))
		})

		It("should increment the numTemplates metric again if another template is created", func() {
			newTemplateCallback(newTemplate)

			// Check if the numTemplates metric has been incremented
			expected := 2.0
			actual := testutil.ToFloat64(numTemplates)
			Expect(actual).To(Equal(expected))
		})

		It("should decrement the numTemplates metric if a template is deleted", func() {
			deleteTemplateCallback(newTemplate)

			// Check if the numTemplates metric has been decremented
			expected := 1.0
			actual := testutil.ToFloat64(numTemplates)
			Expect(actual).To(Equal(expected))
		})

		It("should decrement the numTemplates metric again if another template is deleted", func() {
			deleteTemplateCallback(newTemplate)

			// Check if the numTemplates metric has been decremented
			expected := 0.0
			actual := testutil.ToFloat64(numTemplates)
			Expect(actual).To(Equal(expected))
		})

		It("should not decrement the numTemplates metric if a template is deleted and the metric is already at 0", func() {
			deleteTemplateCallback(newTemplate)

			// Check if the numTemplates metric has been decremented
			expected := 0.0
			actual := testutil.ToFloat64(numTemplates)
			Expect(actual).To(Equal(expected))
		})
	})

	Context("when a template is updated", func() {
		BeforeEach(func() {
			// Create old and new template objects for updateTemplateCallback.
			oldTemplate = &pulumiv1alpha1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "test-template",
				},
			}
			newTemplate = &pulumiv1alpha1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "test-template",
				},
			}
			numTemplates.Set(0)
			numTemplatesFailing.Reset()
			numTemplatesReconciling.Reset()
			numTemplateInstances.Reset()
		})

		Describe("when testing the templates_failing metric", Ordered, func() {
			BeforeAll(func() {
				// Set the new template to a failed state
				newTemplate.Status.Conditions = []metav1.Condition{
					{
						Type:   pulumiv1alpha1.TemplateConditionTypeReady,
						Status: metav1.ConditionFalse,
						Reason: "Failed",
					},
				}

				oldTemplate.Status.Conditions = []metav1.Condition{
					{
						Type:   pulumiv1alpha1.TemplateConditionTypeReady,
						Status: metav1.ConditionTrue,
						Reason: "Ready",
					},
				}
			})

			It("should update the numTemplatesFailing metric to 1 when the template fails", func() {
				// Call the updateTemplateCallback function
				updateTemplateCallback(oldTemplate, newTemplate)

				// Check if the numTemplatesFailing metric has been updated
				expectedFailing := 1.0
				actualFailing := testutil.ToFloat64(numTemplatesFailing.With(prometheus.Labels{"namespace": oldTemplate.Namespace, "name": oldTemplate.Name}))
				Expect(actualFailing).To(Equal(expectedFailing))
			})

			It("should reset the numTemplatesFailing metric when the template succeeds", func() {
				// Update the template objects to be in a succeeded state.
				newTemplate.Status.Conditions = []metav1.Condition{
					{
						Type:   pulumiv1alpha1.TemplateConditionTypeReady,
						Status: metav1.ConditionTrue,
						Reason: "Ready",
					},
				}
				oldTemplate.Status.Conditions = []metav1.Condition{
					{
						Type:   pulumiv1alpha1.TemplateConditionTypeReady,
						Status: metav1.ConditionFalse,
						Reason: "Failed",
					},
				}
				// Call the updateTemplateCallback function
				updateTemplateCallback(oldTemplate, newTemplate)

				// Check if the numTemplatesFailing metric has been updated
				expectedFailing := 0.0
				actualFailing := testutil.ToFloat64(numTemplatesFailing.With(prometheus.Labels{"namespace": oldTemplate.Namespace, "name": oldTemplate.Name}))
				Expect(actualFailing).To(Equal(expectedFailing))
			})
		})

		Describe("when testing the templates_reconciling metric", Ordered, func() {
			BeforeAll(func() {
				// Set the new template to a reconciling state
				newTemplate.Status.Conditions = []metav1.Condition{
					{
						Type:   pulumiv1alpha1.TemplateConditionTypeReconciling,
						Status: metav1.ConditionTrue,
						Reason: "Reconciling",
					},
				}
				oldTemplate.Status.Conditions = []metav1.Condition{
					{
						Type:   pulumiv1alpha1.TemplateConditionTypeReady,
						Status: metav1.ConditionTrue,
						Reason: "Ready",
					},
				}
			})

			It("should update the numTemplatesReconciling metric to 1 when the template is reconciling", func() {
				// Call the updateTemplateCallback function
				updateTemplateCallback(oldTemplate, newTemplate)

				// Check if the numTemplatesReconciling metric has been updated
				expectedReconciling := 1.0
				actualReconciling := testutil.ToFloat64(numTemplatesReconciling.With(prometheus.Labels{"namespace": oldTemplate.Namespace, "name": oldTemplate.Name}))
				Expect(actualReconciling).To(Equal(expectedReconciling))
			})

			It("should reset the numTemplatesReconciling metric when the template is finished reconciling", func() {
				// Update the template objects to be in a succeeded state.
				newTemplate.Status.Conditions = []metav1.Condition{
					{
						Type:   pulumiv1alpha1.TemplateConditionTypeReady,
						Status: metav1.ConditionTrue,
						Reason: "Ready",
					},
				}
				oldTemplate.Status.Conditions = []metav1.Condition{
					{
						Type:   pulumiv1alpha1.TemplateConditionTypeReconciling,
						Status: metav1.ConditionTrue,
						Reason: "Reconciling",
					},
				}
				// Call the updateTemplateCallback function
				updateTemplateCallback(oldTemplate, newTemplate)

				// Check if the numTemplatesReconciling metric has been updated
				expectedReconciling := 0.0
				actualReconciling := testutil.ToFloat64(numTemplatesReconciling.With(prometheus.Labels{"namespace": oldTemplate.Namespace, "name": oldTemplate.Name}))
				Expect(actualReconciling).To(Equal(expectedReconciling))
			})
		})

		Describe("when testing the template_instances_total metric", func() {
			It("should update the instance count metric", func() {
				newTemplate.Status.InstanceCount = 5
				updateTemplateCallback(oldTemplate, newTemplate)

				expectedCount := 5.0
				actualCount := testutil.ToFloat64(numTemplateInstances.With(prometheus.Labels{"namespace": newTemplate.Namespace, "name": newTemplate.Name}))
				Expect(actualCount).To(Equal(expectedCount))
			})
		})
	})

	Context("when a template is deleted", func() {
		BeforeEach(func() {
			// Set the metrics
			numTemplates.Set(1)
			numTemplatesFailing.With(prometheus.Labels{"namespace": "test", "name": "test-template"}).Set(1)
			numTemplatesReconciling.With(prometheus.Labels{"namespace": "test", "name": "test-template"}).Set(1)
			numTemplateInstances.With(prometheus.Labels{"namespace": "test", "name": "test-template"}).Set(3)

			// Create an old template object for deleteTemplateCallback
			oldTemplate = &pulumiv1alpha1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "test-template",
				},
				Status: pulumiv1alpha1.TemplateStatus{
					Conditions: []metav1.Condition{
						{
							Type:   pulumiv1alpha1.TemplateConditionTypeReady,
							Status: metav1.ConditionTrue,
						},
					},
				},
			}
		})

		It("should decrement the numTemplates metric and reset the other metrics", func() {
			// Call the deleteTemplateCallback function
			deleteTemplateCallback(oldTemplate)

			// Check if the numTemplates metric has been decremented
			expected := 0.0
			actual := testutil.ToFloat64(numTemplates)
			Expect(actual).To(Equal(expected))

			// Check if the numTemplatesFailing metric has been reset
			expectedFailing := 0.0
			actualFailing := testutil.ToFloat64(numTemplatesFailing.With(prometheus.Labels{"namespace": oldTemplate.Namespace, "name": oldTemplate.Name}))
			Expect(actualFailing).To(Equal(expectedFailing))

			// Check if the numTemplatesReconciling metric has been reset
			expectedReconciling := 0.0
			actualReconciling := testutil.ToFloat64(numTemplatesReconciling.With(prometheus.Labels{"namespace": oldTemplate.Namespace, "name": oldTemplate.Name}))
			Expect(actualReconciling).To(Equal(expectedReconciling))

			// Check if the numTemplateInstances metric has been reset
			expectedInstances := 0.0
			actualInstances := testutil.ToFloat64(numTemplateInstances.With(prometheus.Labels{"namespace": oldTemplate.Namespace, "name": oldTemplate.Name}))
			Expect(actualInstances).To(Equal(expectedInstances))
		})
	})

	Context("when recording reconcile duration", func() {
		It("should record the duration of a reconciliation", func() {
			startTime := time.Now().Add(-1 * time.Second)
			RecordTemplateReconcileDuration("test-ns", "test-template", startTime, "success")

			// Check that the histogram has been updated (we can't easily check the exact value)
			// Just verify no panic occurred
		})
	})
})
