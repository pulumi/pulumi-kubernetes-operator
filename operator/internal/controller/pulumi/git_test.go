// Copyright 2024, Pulumi Corporation.
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
	"context"
	"testing"

	"github.com/pulumi/pulumi-kubernetes-operator/operator/api/pulumi/shared"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _pko = "https://github.com/pulumi/pulumi-kubernetes-operator.git"

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		source  shared.GitSource
		wantErr string
	}{
		{
			name: "missing projectRepo",
			source: shared.GitSource{
				Branch: "$$$",
			},
			wantErr: `missing "projectRepo"`,
		},
		{
			name: "missing branch and commit",
			source: shared.GitSource{
				ProjectRepo: _pko,
			},
			wantErr: `missing "commit" or "branch"`,
		},
		{
			name: "branch and commit",
			source: shared.GitSource{
				ProjectRepo: _pko,
				Branch:      "master",
				Commit:      "55d7ace59a14b8ac7d4def0065040f1c31c90cd3",
			},
			wantErr: `only one of "commit" or "branch"`,
		},
		{
			name: "invalid url",
			source: shared.GitSource{
				ProjectRepo: "hhtp://github.com/pulumi",
				Branch:      "master",
			},
			wantErr: "invalid URL scheme: hhtp",
		},
	}

	for _, tt := range tests {
		_, err := NewGitSource(tt.source, nil /* auth */)
		assert.ErrorContains(t, err, tt.wantErr)

	}
}

func TestCurrentCommit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		source  shared.GitSource
		auth    *auto.GitAuth
		wantErr string
		want    string
		eq      assert.ValueAssertionFunc
	}{
		{
			name: "branch",
			source: shared.GitSource{
				ProjectRepo: "https://github.com/git-fixtures/basic.git",
				Branch:      "master",
			},
			want: "6ecf0ef2c2dffb796033e5a02219af86ec6584e5",
		},
		{
			name: "tag",
			source: shared.GitSource{
				ProjectRepo: _pko,
				Branch:      "refs/tags/v1.16.0",
			},
			want: "2ca775387e522fd5c29668a85bfba2f8fd791848",
		},
		{
			name: "commit",
			source: shared.GitSource{
				ProjectRepo: _pko,
				Commit:      "55d7ace59a14b8ac7d4def0065040f1c31c90cd3",
			},
			want: "55d7ace59a14b8ac7d4def0065040f1c31c90cd3",
		},
		{
			name: "non-existent ref",
			source: shared.GitSource{
				ProjectRepo: _pko,
				Branch:      "doesntexist",
			},
			wantErr: "no commits found for ref",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs, err := NewGitSource(tt.source, nil /* auth */)
			require.NoError(t, err)

			commit, err := gs.CurrentCommit(context.Background())
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}

			assert.Equal(t, tt.want, commit)
		})
	}
}
