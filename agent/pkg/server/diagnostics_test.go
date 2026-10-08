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

package server

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
)

func diagnostic(severity, prefix, message string) apitype.EngineEvent {
	return apitype.EngineEvent{
		DiagnosticEvent: &apitype.DiagnosticEvent{
			Severity: severity,
			Prefix:   prefix,
			Message:  message,
		},
	}
}

func TestErrorDiagnostics_JoinsPrefixAndMessage(t *testing.T) {
	d := &errorDiagnostics{}
	d.observe(diagnostic("error", "error: ", "update canceled\n"))

	assert.Equal(t, "up failed: error: update canceled", d.failureMessage("up"))
}

func TestErrorDiagnostics_IgnoresSeveritiesThatAreNotFailures(t *testing.T) {
	d := &errorDiagnostics{}
	d.observe(diagnostic("info", "", "creating resource\n"))
	d.observe(diagnostic("debug", "debug: ", "registering resource\n"))
	d.observe(diagnostic("warning", "warning: ", "deprecated\n"))
	d.observe(apitype.EngineEvent{})

	assert.Equal(t, "refresh failed; see the workspace pod logs", d.failureMessage("refresh"))
}

func TestErrorDiagnostics_FallsBackWhenTheEngineReportedNothing(t *testing.T) {
	d := &errorDiagnostics{}

	assert.Equal(t, "destroy failed; see the workspace pod logs", d.failureMessage("destroy"))
}

func TestErrorDiagnostics_ReportsRelayedStderrWhenThereIsNoErrorDiagnostic(t *testing.T) {
	d := &errorDiagnostics{}
	d.observe(diagnostic("info#err", "", "Error: resource, variable, or config value \"x\" not found\n\n"))
	d.observe(diagnostic("info#err", "", "  on Pulumi.yaml line 8:\n\n"))

	assert.Equal(t, "preview failed: Error: resource, variable, or config value \"x\" not found\n"+
		"on Pulumi.yaml line 8:", d.failureMessage("preview"))
}

func TestErrorDiagnostics_PrefersErrorDiagnosticsOverRelayedStderr(t *testing.T) {
	d := &errorDiagnostics{}
	d.observe(diagnostic("info#err", "", "plugin chatter\n"))
	d.observe(diagnostic("error", "error: ", "the real failure\n"))
	d.observe(diagnostic("info#err", "", "more plugin chatter\n"))

	msg := d.failureMessage("up")
	assert.Equal(t, "up failed: error: the real failure", msg)
	assert.NotContains(t, msg, "chatter", "stderr must not crowd out an error diagnostic")
}

func TestErrorDiagnostics_ReportsEveryDiagnosticUpToTheLimit(t *testing.T) {
	d := &errorDiagnostics{}
	for i := 0; i < maxDiagnostics; i++ {
		d.observe(diagnostic("error", "error: ", fmt.Sprintf("failure %d\n", i)))
	}

	msg := d.failureMessage("up")
	assert.Equal(t, maxDiagnostics, strings.Count(msg, "error: "))
	assert.NotContains(t, msg, "further diagnostics")
}

func TestErrorDiagnostics_CountsDiagnosticsPastTheLimit(t *testing.T) {
	d := &errorDiagnostics{}
	for i := 0; i < maxDiagnostics+7; i++ {
		d.observe(diagnostic("error", "error: ", fmt.Sprintf("failure %d\n", i)))
	}

	msg := d.failureMessage("up")
	assert.Equal(t, maxDiagnostics, strings.Count(msg, "error: "))
	assert.Contains(t, msg, "(7 further diagnostics; see the workspace pod logs)")
}

func TestErrorDiagnostics_BoundsRelayedStderrToo(t *testing.T) {
	d := &errorDiagnostics{}
	for i := 0; i < maxDiagnostics+3; i++ {
		d.observe(diagnostic("info#err", "", fmt.Sprintf("stderr line %d\n", i)))
	}

	msg := d.failureMessage("up")
	assert.Equal(t, maxDiagnostics, strings.Count(msg, "stderr line "))
	assert.Contains(t, msg, "(3 further diagnostics; see the workspace pod logs)")
}

func TestErrorDiagnostics_KeepsTheFirstDiagnosticsNotTheLast(t *testing.T) {
	d := &errorDiagnostics{}
	for i := 0; i < maxDiagnostics+1; i++ {
		d.observe(diagnostic("error", "error: ", fmt.Sprintf("failure %d\n", i)))
	}

	msg := d.failureMessage("up")
	assert.Contains(t, msg, "failure 0", "the first failure is usually the cause")
	assert.NotContains(t, msg, fmt.Sprintf("failure %d", maxDiagnostics))
}

func TestErrorDiagnostics_StaysUnderTheByteBudget(t *testing.T) {
	d := &errorDiagnostics{}
	for i := 0; i < maxDiagnostics; i++ {
		d.observe(diagnostic("error", "error: ", strings.Repeat("x", maxDiagnosticBytes)+"\n"))
	}

	msg := d.failureMessage("up")
	assert.Less(t, len(msg), maxDiagnosticBytes+256)
	assert.Equal(t, "up failed; see the workspace pod logs", msg)
}

func TestErrorDiagnostics_NeverCutsADiagnosticInHalf(t *testing.T) {
	d := &errorDiagnostics{}
	d.observe(diagnostic("error", "error: ", "first\n"))
	d.observe(diagnostic("error", "error: ", strings.Repeat("y", maxDiagnosticBytes)+"\n"))

	msg := d.failureMessage("up")
	assert.Contains(t, msg, "error: first")
	assert.NotContains(t, msg, "yyyy", "an oversized diagnostic is dropped whole, not truncated")
}
