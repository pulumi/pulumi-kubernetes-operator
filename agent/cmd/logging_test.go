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

package cmd

import (
	"testing"

	"go.uber.org/zap"
)

func TestApplyJSONLogMode_DefaultsToConsole(t *testing.T) {
	// AGENT_JSON_LOG unset: helper must not change the encoding.
	t.Setenv("AGENT_JSON_LOG", "")
	zc := zap.NewDevelopmentConfig() // Encoding == "console" by default.
	got := applyJSONLogMode(zc)
	if got.Encoding != "console" {
		t.Fatalf("Encoding = %q; want \"console\" (default)", got.Encoding)
	}
}

func TestApplyJSONLogMode_OffValuesAreIgnored(t *testing.T) {
	// Anything other than the literal string "true" must be ignored,
	// so misconfigurations on the workspace pod spec don't accidentally
	// flip the encoder and break downstream log scrapers.
	for _, val := range []string{"false", "0", "no", "TRUE ", "yes", "True"} {
		t.Run(val, func(t *testing.T) {
			t.Setenv("AGENT_JSON_LOG", val)
			zc := zap.NewDevelopmentConfig()
			got := applyJSONLogMode(zc)
			if got.Encoding != "console" {
				t.Fatalf("AGENT_JSON_LOG=%q: Encoding = %q; want \"console\"",
					val, got.Encoding)
			}
		})
	}
}

func TestApplyJSONLogMode_TrueSwitchesToJSON(t *testing.T) {
	t.Setenv("AGENT_JSON_LOG", "true")
	zc := zap.NewDevelopmentConfig()
	got := applyJSONLogMode(zc)
	if got.Encoding != "json" {
		t.Fatalf("Encoding = %q; want \"json\" when AGENT_JSON_LOG=true",
			got.Encoding)
	}
}
