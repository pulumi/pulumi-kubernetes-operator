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
	"os"

	"go.uber.org/zap"
)

// applyJSONLogMode returns a zap.Config mutated so that its Encoding is set
// to "json" when the caller asked for structured logging via the
// AGENT_JSON_LOG environment variable, and unchanged otherwise.
//
// The config is passed by value (not pointer) so callers retain full control
// over the result. The decision is gated on the literal string "true" so
// unset, empty, "0", "false", or accidental typos all default to the
// historical console encoder — preserving existing log scrapers.
func applyJSONLogMode(zc zap.Config) zap.Config {
	if os.Getenv("AGENT_JSON_LOG") == "true" {
		zc.Encoding = "json"
	}
	return zc
}
