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
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

const (
	// maxDiagnostics and maxDiagnosticBytes bound the failure message. A
	// diagnostic is kept whole or dropped whole, so the message never ends
	// mid-sentence.
	maxDiagnostics     = 10
	maxDiagnosticBytes = 32 << 10

	// drainTimeout bounds the wait for the event stream to finish. The stream
	// is closed before the Pulumi operation returns, so the wait is normally
	// over at once. It only expires if the operation failed before the engine
	// produced any events at all.
	drainTimeout = 5 * time.Second
)

// errorDiagnostics collects the error-severity diagnostics from an engine
// event stream.
type errorDiagnostics struct {
	mu      sync.Mutex
	lines   []string
	size    int
	omitted int
	drained chan struct{}
}

func newErrorDiagnostics() *errorDiagnostics {
	return &errorDiagnostics{drained: make(chan struct{})}
}

func (d *errorDiagnostics) observe(event apitype.EngineEvent) {
	diag := event.DiagnosticEvent
	if diag == nil || diag.Severity != "error" {
		return
	}
	line := strings.TrimSpace(diag.Prefix + diag.Message)
	if line == "" {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.lines) >= maxDiagnostics || d.size+len(line) > maxDiagnosticBytes {
		d.omitted++
		return
	}
	d.lines = append(d.lines, line)
	d.size += len(line) + 1
}

// close marks the event stream as fully processed.
func (d *errorDiagnostics) close() {
	close(d.drained)
}

// failureMessage renders the diagnostics as a gRPC status message. The whole
// output stays in the workspace pod log either way.
func (d *errorDiagnostics) failureMessage(operation string) string {
	select {
	case <-d.drained:
	case <-time.After(drainTimeout):
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.lines) == 0 {
		return fmt.Sprintf("%s failed; see the workspace pod logs", operation)
	}
	msg := fmt.Sprintf("%s failed: %s", operation, strings.Join(d.lines, "\n"))
	if d.omitted > 0 {
		msg += fmt.Sprintf("\n(%d further diagnostics; see the workspace pod logs)", d.omitted)
	}
	return msg
}
