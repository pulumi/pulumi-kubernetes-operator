// Copyright 2021, Pulumi Corporation.  All rights reserved.

package tests

import (
	"context"

	//	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Stacks that should stall", func() {

	var (
		stack pulumiv1.Stack
	)

	AfterEach(func() {
		if stack.Name != "" { // assume that if it's been named, it was created in the cluster
			Expect(k8sClient.Delete(context.TODO(), &stack)).To(Succeed())
		}
	})

	When("there is no source specified", func() {

		BeforeEach(func() {
			stack = pulumiv1.Stack{}
			stack.Name = randString()
			stack.Namespace = "default"
			Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())
		})

		It("should mark a stack as stalled", func() {
			// wait until the controller has seen the stack object and completed processing it
			var s pulumiv1.Stack
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(&stack), &s)
				if err != nil {
					return false
				}
				if s.Generation == 0 {
					return false
				}
				return s.Status.ObservedGeneration == s.Generation
			}, "20s", "1s").Should(BeTrue())

			Expect(apimeta.IsStatusConditionTrue(s.Status.Conditions, pulumiv1.ReadyCondition)).To(BeFalse())
			Expect(apimeta.FindStatusCondition(s.Status.Conditions, pulumiv1.ReconcilingCondition)).To(BeNil())
			stalledCondition := apimeta.FindStatusCondition(s.Status.Conditions, pulumiv1.StalledCondition)
			Expect(stalledCondition).ToNot(BeNil())
			Expect(stalledCondition.Reason).To(Equal(pulumiv1.StalledSpecInvalidReason))
		})
	})
})
