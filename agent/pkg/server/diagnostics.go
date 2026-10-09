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
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

const (
	// maxDiagnostics and maxDiagnosticBytes bound the failure message. A
	// diagnostic is kept whole or dropped whole, so the message never ends
	// mid-sentence.
	maxDiagnostics     = 10
	maxDiagnosticBytes = 32 << 10
)

// boundedLines accumulates diagnostics up to the bounds and counts the rest.
type boundedLines struct {
	lines   []string
	size    int
	omitted int
}

func (b *boundedLines) add(line string) {
	if len(b.lines) >= maxDiagnostics || b.size+len(line) > maxDiagnosticBytes {
		b.omitted++
		return
	}
	b.lines = append(b.lines, line)
	b.size += len(line) + 1
}

// empty reports whether nothing was observed at all. A diagnostic dropped for
// being oversized still counts, so that it cannot be masked by a later source.
func (b *boundedLines) empty() bool {
	return len(b.lines) == 0 && b.omitted == 0
}

// errorDiagnostics collects the failure diagnostics from an engine event stream.
type errorDiagnostics struct {
	mu sync.Mutex
	// errs holds "error" severity diagnostics, which the engine raises when a
	// resource operation returns an error.
	errs boundedLines
	// stderr holds "info#err" severity diagnostics, which relay whatever a
	// language runtime or a provider plugin wrote to stderr. A program that
	// fails to evaluate, or a plugin that crashes, reports only through these.
	stderr boundedLines
}

func (d *errorDiagnostics) observe(event apitype.EngineEvent) {
	diag := event.DiagnosticEvent
	if diag == nil {
		return
	}
	line := strings.TrimSpace(diag.Prefix + diag.Message)
	if line == "" {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	switch diag.Severity {
	case "error":
		d.errs.add(line)
	case "info#err":
		d.stderr.add(line)
	}
}

func (d *errorDiagnostics) failureMessage(operation string) string {
	d.mu.Lock()
	defer d.mu.Unlock()

	reported := &d.errs
	if reported.empty() {
		reported = &d.stderr
	}

	switch {
	case reported.empty():
		return fmt.Sprintf("%s failed; see the workspace pod logs", operation)
	case len(reported.lines) == 0:
		return fmt.Sprintf("%s failed; %d diagnostics were too large to report, see the workspace pod logs",
			operation, reported.omitted)
	case reported.omitted > 0:
		return fmt.Sprintf("%s failed: %s\n(%d further diagnostics; see the workspace pod logs)",
			operation, strings.Join(reported.lines, "\n"), reported.omitted)
	default:
		return fmt.Sprintf("%s failed: %s", operation, strings.Join(reported.lines, "\n"))
	}
}
