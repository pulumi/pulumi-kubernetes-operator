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
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var _ = Describe("Program Metrics", Ordered, func() {
	BeforeAll(func() {
		numPrograms.Set(0)
	})

	It("should increment the programs_active metric when a new Program is created", func() {
		newProgramCallback(nil)
		Expect(testutil.ToFloat64(numPrograms)).To(Equal(1.0))
	})

	It("should increment the programs_active metric when another Program is created", func() {
		newProgramCallback(nil)
		Expect(testutil.ToFloat64(numPrograms)).To(Equal(2.0))
	})

	It("should decrement the programs_active metric when a Program is deleted", func() {
		deleteProgramCallback(nil)
		Expect(testutil.ToFloat64(numPrograms)).To(Equal(1.0))
	})

	It("should decrement the programs_active metric when another Program is deleted", func() {
		deleteProgramCallback(nil)
		Expect(testutil.ToFloat64(numPrograms)).To(Equal(0.0))
	})

	It("should not decrement the programs_active metric when a Program is deleted and the metric is already at 0", func() {
		deleteProgramCallback(nil)
		Expect(testutil.ToFloat64(numPrograms)).To(Equal(0.0))
	})
})
