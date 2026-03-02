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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewLoggerConfig(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		unset      bool
		wantErr    string
		wantJSON   bool
		wantLevel  string
		wantTime   string
		wantMsg    string
		wantSample bool
	}{
		{
			name:       "unset uses development config",
			unset:      true,
			wantJSON:   false,
			wantLevel:  "L",
			wantTime:   "T",
			wantMsg:    "M",
			wantSample: false,
		},
		{
			name:       "console uses development config",
			value:      "console",
			wantJSON:   false,
			wantLevel:  "L",
			wantTime:   "T",
			wantMsg:    "M",
			wantSample: false,
		},
		{
			name:       "json uses production config without sampling",
			value:      "json",
			wantJSON:   true,
			wantLevel:  "level",
			wantTime:   "ts",
			wantMsg:    "msg",
			wantSample: false,
		},
		{
			name:    "invalid value fails fast",
			value:   "bad",
			wantErr: "accepted values are",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(agentPulumiJSONOutputEnvVar, "")
			if tt.unset {
				require.NoError(t, os.Unsetenv(agentLogFormatEnvVar))
			} else {
				t.Setenv(agentLogFormatEnvVar, tt.value)
			}

			cfg, err := newLoggerConfig()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			if tt.wantJSON {
				require.Equal(t, "json", cfg.Encoding)
			} else {
				require.Equal(t, "console", cfg.Encoding)
			}
			require.Equal(t, tt.wantLevel, cfg.EncoderConfig.LevelKey)
			require.Equal(t, tt.wantTime, cfg.EncoderConfig.TimeKey)
			require.Equal(t, tt.wantMsg, cfg.EncoderConfig.MessageKey)
			if tt.wantSample {
				require.NotNil(t, cfg.Sampling)
			} else {
				require.Nil(t, cfg.Sampling)
			}
		})
	}
}

func TestGetAgentPulumiJSONOutput(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		unset   bool
		want    bool
		wantErr string
	}{
		{name: "unset defaults false", unset: true, want: false},
		{name: "true enables json output", value: "true", want: true},
		{name: "false disables json output", value: "false", want: false},
		{name: "invalid value fails fast", value: "nope", wantErr: "accepted values are"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.unset {
				require.NoError(t, os.Unsetenv(agentPulumiJSONOutputEnvVar))
			} else {
				t.Setenv(agentPulumiJSONOutputEnvVar, tt.value)
			}

			got, err := getAgentPulumiJSONOutput()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
