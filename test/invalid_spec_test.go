// Copyright 2021, Pulumi Corporation.  All rights reserved.

package tests

import (
	"context"

	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func checkInvalidSpecStalls(when string, setup func(stack *pulumiv1.Stack)) {
	When(when, func() {
		var stack pulumiv1.Stack

		BeforeEach(func() {
			stack = pulumiv1.Stack{}
			stack.Namespace = "default"
			setup(&stack)
			stack.Name += ("-" + randString())
			Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())
		})

		It("should mark the stack as stalled", func() {
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
			Expect(s.Status.LastUpdate).ToNot(BeNil(), ".status.lastUpdate is recorded")
			Expect(s.Status.LastUpdate.State).To(Equal(shared.FailedStackStateMessage))
			stalledCondition := apimeta.FindStatusCondition(s.Status.Conditions, pulumiv1.StalledCondition)
			Expect(stalledCondition).ToNot(BeNil(), "stalled condition is present")
			Expect(stalledCondition.Reason).To(Equal(pulumiv1.StalledSpecInvalidReason))
			// not ready, and not in progress
			Expect(apimeta.IsStatusConditionTrue(s.Status.Conditions, pulumiv1.ReadyCondition)).To(BeFalse(), "ready condition is false")
			Expect(apimeta.FindStatusCondition(s.Status.Conditions, pulumiv1.ReconcilingCondition)).To(BeNil(), "reconciling condition is absent")
		})

		AfterEach(func() {
			if stack.Name != "" { // assume that if it's been named, it was created in the cluster
				deleteAndWaitForFinalization(&stack)
			}
		})
	})
}

var _ = Describe("Stacks that should stall because of an invalid spec", func() {

	checkInvalidSpecStalls("there is no source specified", func(s *pulumiv1.Stack) {
		s.Name = "no-source"
	})

	checkInvalidSpecStalls("a git source with no branch and no commit hash is given", func(s *pulumiv1.Stack) {
		s.Name = "invalid-git-source"
		s.Spec.GitSource = &shared.GitSource{
			ProjectRepo: "https://github.com/pulumi/pulumi-kubernetes-operator",
		}
	})

	checkInvalidSpecStalls("more than one source is given", func(s *pulumiv1.Stack) {
		s.Name = "more-than-one-source"
		s.Spec.GitSource = &shared.GitSource{
			ProjectRepo: "https://github.com/pulumi/pulumi-kubernetes-operator",
			Branch:      "default",
		}
		s.Spec.FluxSource = &shared.FluxSource{
			SourceRef: shared.FluxSourceReference{
				Name: "foo",
			},
		}
	})
})
