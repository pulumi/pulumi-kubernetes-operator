// Copyright 2026, Pulumi Corporation.
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

package pathfilter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		filterStrings []string
		path          string
		want          bool
	}{
		{
			name:          "no filters includes everything",
			filterStrings: nil,
			path:          "tenants/acme/index.ts",
			want:          true,
		},
		{
			name:          "include matches",
			filterStrings: []string{"tenants/acme/**"},
			path:          "tenants/acme/index.ts",
			want:          true,
		},
		{
			name:          "include does not match another tenant",
			filterStrings: []string{"tenants/acme/**"},
			path:          "tenants/globex/index.ts",
			want:          false,
		},
		{
			name:          "second include matches",
			filterStrings: []string{"tenants/acme/**", "lib/**"},
			path:          "lib/util.ts",
			want:          true,
		},
		{
			name:          "neither of two includes matches",
			filterStrings: []string{"tenants/acme/**", "lib/**"},
			path:          "docs/readme.md",
			want:          false,
		},
		{
			name:          "exclude only includes everything else",
			filterStrings: []string{"!docs/**"},
			path:          "tenants/acme/index.ts",
			want:          true,
		},
		{
			name:          "exclude only excludes its own match",
			filterStrings: []string{"!docs/**"},
			path:          "docs/readme.md",
			want:          false,
		},
		{
			name:          "exclude beats include",
			filterStrings: []string{"tenants/acme/**", "!**/*.md"},
			path:          "tenants/acme/README.md",
			want:          false,
		},
		{
			name:          "exclude beats include whatever the order",
			filterStrings: []string{"!**/*.md", "tenants/acme/**"},
			path:          "tenants/acme/README.md",
			want:          false,
		},
		{
			name:          "include survives an exclude that does not match",
			filterStrings: []string{"tenants/acme/**", "!**/*.md"},
			path:          "tenants/acme/index.ts",
			want:          true,
		},
		{
			name:          "a single star crosses a separator",
			filterStrings: []string{"tenants/*"},
			path:          "tenants/acme/index.ts",
			want:          true,
		},
		{
			name:          "an exact path matches itself",
			filterStrings: []string{"Pulumi.yaml"},
			path:          "Pulumi.yaml",
			want:          true,
		},
		{
			name:          "an exact path does not match a nested file",
			filterStrings: []string{"Pulumi.yaml"},
			path:          "tenants/acme/Pulumi.yaml",
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			filters, allExcludes, err := Compile(tt.filterStrings)
			require.NoError(t, err)
			assert.Equal(t, tt.want, Matches(filters, allExcludes, tt.path))
		})
	}
}

func TestCompile_ReportsAllExcludes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		filterStrings []string
		want          bool
	}{
		{name: "no filters", filterStrings: nil, want: true},
		{name: "only excludes", filterStrings: []string{"!docs/**", "!**/*.md"}, want: true},
		{name: "one include", filterStrings: []string{"lib/**"}, want: false},
		{name: "mixed", filterStrings: []string{"lib/**", "!**/*.md"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, allExcludes, err := Compile(tt.filterStrings)
			require.NoError(t, err)
			assert.Equal(t, tt.want, allExcludes)
		})
	}
}

func TestCompile_RejectsAnInvalidFilter(t *testing.T) {
	t.Parallel()

	_, _, err := Compile([]string{"tenants/[acme"})
	require.Error(t, err)
}
