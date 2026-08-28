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

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// TestEnvDuration and TestEnvInt cover the environment overrides that used to discard their
// parse errors. A malformed MAX_CONCURRENT_RECONCILES silently became 0, which
// controller-runtime treats as 1 -- a silent 25x throughput drop with nothing in the logs.
// See https://github.com/pulumi/pulumi-kubernetes-operator/issues/1293.
func TestEnvDuration(t *testing.T) {
	const name = "TEST_DURATION"
	tests := []struct {
		name    string
		set     bool
		value   string
		want    time.Duration
		wantErr string
	}{
		{name: "unset leaves the default", want: time.Second},
		{name: "valid value overrides", set: true, value: "5m", want: 5 * time.Minute},
		{name: "zero is honored", set: true, value: "0s", want: 0},
		{name: "malformed is an error", set: true, value: "5", want: time.Second, wantErr: `invalid duration in TEST_DURATION="5"`},
		{name: "empty is an error", set: true, value: "", want: time.Second, wantErr: `invalid duration in TEST_DURATION=""`},
		{name: "trailing space is an error", set: true, value: "5m ", want: time.Second, wantErr: "invalid duration"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv(name, tt.value)
			}
			got := time.Second
			err := envDuration(name, &got)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got, "the target should be left alone on error")
		})
	}
}

func TestEnvInt(t *testing.T) {
	const name = "TEST_INT"
	tests := []struct {
		name    string
		set     bool
		value   string
		want    int
		wantErr string
	}{
		{name: "unset leaves the default", want: 25},
		{name: "valid value overrides", set: true, value: "50", want: 50},
		{name: "zero is parsed, and rejected later by validateConcurrency", set: true, value: "0", want: 0},
		{name: "malformed is an error", set: true, value: "abc", want: 25, wantErr: `invalid integer in TEST_INT="abc"`},
		{name: "empty is an error", set: true, value: "", want: 25, wantErr: "invalid integer"},
		{name: "trailing space is an error", set: true, value: "10 ", want: 25, wantErr: "invalid integer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv(name, tt.value)
			}
			got := 25
			err := envInt(name, &got)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got, "the target should be left alone on error")
		})
	}
}

func TestValidateConcurrency(t *testing.T) {
	tests := []struct {
		name        string
		maxRecon    int
		updateRecon int
		idle        time.Duration
		wantErr     string
	}{
		{name: "defaults are valid", maxRecon: 25, updateRecon: 0, idle: 30 * time.Minute},
		{name: "an explicit update budget is valid", maxRecon: 25, updateRecon: 10, idle: time.Minute},
		{name: "a disabled idle timeout is valid", maxRecon: 1, updateRecon: 0, idle: 0},
		{name: "zero reconciles is rejected", maxRecon: 0, wantErr: "max-concurrent-reconciles must be greater than zero"},
		{name: "negative reconciles is rejected", maxRecon: -1, wantErr: "max-concurrent-reconciles must be greater than zero"},
		{name: "a negative update budget is rejected", maxRecon: 25, updateRecon: -1, wantErr: "update-max-concurrent-reconciles must not be negative"},
		{name: "a negative idle timeout is rejected", maxRecon: 25, idle: -time.Second, wantErr: "update-idle-timeout must not be negative"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConcurrency(tt.maxRecon, tt.updateRecon, tt.idle)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestDetermineAdvAddr(t *testing.T) {
	const fakehostname = "fakehostname"
	t.Setenv("HOSTNAME", fakehostname)

	tests := []struct {
		addr string
		want string
	}{
		{
			addr: ":9090",
			want: "localhost:9090",
		},
		{
			addr: "localhost:1111",
			want: "localhost:1111",
		},
		{
			addr: "0.0.0.0:9090",
			want: fakehostname + ":9090",
		},
		{
			addr: "fake.default:9090",
			want: "fake.default:9090",
		},
		{
			addr: "fake.default.svc.cluster.local:9090",
			want: "fake.default.svc.cluster.local:9090",
		},
	}
	for _, tc := range tests {
		t.Run(tc.addr, func(t *testing.T) {
			if got := determineAdvAddr(tc.addr); got != tc.want {
				t.Errorf("determineAdvAddr() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEnvOrDefault(t *testing.T) {
	// Set up some ENV vars for testing.
	t.Setenv("TEST_ENV", "test")
	t.Setenv("EMPTY_ENV", "")

	tests := []struct {
		name         string
		envName      string
		defaultValue string
		want         string
	}{
		{
			name:         "env set, default ignored",
			envName:      "TEST_ENV",
			defaultValue: "default",
			want:         "test",
		},
		{
			name:         "env not set, default used",
			envName:      "EMPTY_ENV",
			defaultValue: "default",
			want:         "default",
		},
		{
			name:         "env not set, no default",
			envName:      "EMPTY_ENV",
			defaultValue: "",
			want:         "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := envOrDefault(tc.envName, tc.defaultValue); got != tc.want {
				t.Errorf("envOrDefault() = %v, want %v", got, tc.want)
			}
		})
	}
}
