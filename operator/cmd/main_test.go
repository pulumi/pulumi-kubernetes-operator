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

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

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
