// Copyright 2021, Pulumi Corporation.  All rights reserved.

//go:build integration
// +build integration

package tests

import (
	"context"

	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
			waitForStackFailure(&stack)
			expectStalledWithReason(stack.Status.Conditions, pulumiv1.StalledSpecInvalidReason)
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

	checkInvalidSpecStalls("git and flux sources are both given", func(s *pulumiv1.Stack) {
		s.Name = "git-and-flux-source"
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

	checkInvalidSpecStalls("git and program sources are both given", func(s *pulumiv1.Stack) {
		s.Name = "git-and-program-source"
		s.Spec.GitSource = &shared.GitSource{
			ProjectRepo: "https://github.com/pulumi/pulumi-kubernetes-operator",
			Branch:      "default",
		}
		s.Spec.ProgramRef = &shared.ProgramReference{
			Name: "foo",
		}
	})

	checkInvalidSpecStalls("flux and program sources are both given", func(s *pulumiv1.Stack) {
		s.Name = "flux-and-program-source"
		s.Spec.FluxSource = &shared.FluxSource{
			SourceRef: shared.FluxSourceReference{
				Name: "foo",
			},
		}
		s.Spec.ProgramRef = &shared.ProgramReference{
			Name: "foo",
		}
	})

})
