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

package pulumi

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"testing"

	cryptossh "golang.org/x/crypto/ssh"

	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
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

			commit, err := gs.CurrentCommit(t.Context())
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}

			assert.Equal(t, tt.want, commit)
		})
	}
}

func TestAuthMethod(t *testing.T) {
	t.Parallel()

	const (
		httpsURL = "https://github.com/pulumi/examples.git"
		sshURL   = "ssh://git@github.com/pulumi/examples.git"
	)

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := cryptossh.MarshalPrivateKey(priv, "")
	require.NoError(t, err)
	sshKey := string(pem.EncodeToMemory(block))

	tests := []struct {
		name        string
		url         string
		auth        *auto.GitAuth
		wantOptions int
		wantErr     string
	}{
		{
			name: "no auth",
			url:  httpsURL,
			auth: nil,
		},
		{
			name: "auth with no credential",
			url:  httpsURL,
			auth: &auto.GitAuth{},
		},
		{
			name:        "username and password",
			url:         httpsURL,
			auth:        &auto.GitAuth{Username: "u", Password: "p"},
			wantOptions: 1,
		},
		{
			name:        "personal access token",
			url:         httpsURL,
			auth:        &auto.GitAuth{PersonalAccessToken: "t"},
			wantOptions: 1,
		},
		{
			name:        "ssh private key",
			url:         sshURL,
			auth:        &auto.GitAuth{SSHPrivateKey: sshKey},
			wantOptions: 1,
		},
		{
			name:    "ssh private key that does not parse",
			url:     sshURL,
			auth:    &auto.GitAuth{SSHPrivateKey: "not a private key"},
			wantErr: "ssh:",
		},
		{
			name:        "username takes precedence over a token",
			url:         httpsURL,
			auth:        &auto.GitAuth{Username: "u", Password: "p", PersonalAccessToken: "t"},
			wantOptions: 1,
		},
		{
			name:        "a token takes precedence over an ssh key",
			url:         httpsURL,
			auth:        &auto.GitAuth{PersonalAccessToken: "t", SSHPrivateKey: sshKey},
			wantOptions: 1,
		},
		{
			name:    "an ssh key with an https url",
			url:     httpsURL,
			auth:    &auto.GitAuth{SSHPrivateKey: sshKey},
			wantErr: "an ssh private key requires a ssh:// repository URL",
		},
		{
			name:    "a token with an ssh url",
			url:     sshURL,
			auth:    &auto.GitAuth{PersonalAccessToken: "t"},
			wantErr: "an access token requires a http:// or https:// repository URL",
		},
		{
			name:    "a username with an ssh url",
			url:     sshURL,
			auth:    &auto.GitAuth{Username: "u", Password: "p"},
			wantErr: "a username and password requires a http:// or https:// repository URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts, err := gitSource{url: tt.url, auth: tt.auth}.authMethod()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, opts)
				return
			}
			require.NoError(t, err)
			assert.Len(t, opts, tt.wantOptions)
		})
	}
}

func TestLatestChange(t *testing.T) {
	t.Parallel()

	// git-fixtures/basic is a stable fixture repository: its master tip, that commit's first
	// parent, and the diff between them do not change.
	const (
		tip    = "6ecf0ef2c2dffb796033e5a02219af86ec6584e5"
		parent = "918c48b83bd081e863dbe1b80f8998f058cd8294"
	)

	gs, err := NewGitSource(shared.GitSource{
		ProjectRepo: "https://github.com/git-fixtures/basic.git",
		Branch:      "master",
	}, nil /* auth */)
	require.NoError(t, err)

	change, err := gs.LatestChange(t.Context())
	require.NoError(t, err)

	assert.Equal(t, tip, change.Commit)
	assert.Equal(t, parent, change.Parent)
	assert.ElementsMatch(t, []string{"vendor/foo.go"}, change.Paths)
}
