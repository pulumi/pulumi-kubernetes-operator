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
	"errors"
	"fmt"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/pulumi/pulumi-kubernetes-operator/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi-kubernetes-operator/operator/api/pulumi/shared"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	v1 "k8s.io/api/core/v1"
)

// Source represents a source of commits.
type Source interface {
	CurrentCommit(context.Context) (string, error)
}

// NewGitSource creates a new Git source. URL is the https location of the
// repository. Ref is typically the branch to fetch.
func NewGitSource(gs shared.GitSource, auth *auto.GitAuth) (Source, error) {
	if gs.ProjectRepo == "" {
		return nil, fmt.Errorf(`missing "projectRepo"`)
	}
	if gs.Commit == "" && gs.Branch == "" {
		return nil, fmt.Errorf(`missing "commit" or "branch"`)
	}
	if gs.Commit != "" && gs.Branch != "" {
		return nil, fmt.Errorf(`only one of "commit" or "branch" should be specified`)
	}

	ref := gs.Commit
	if gs.Branch != "" {
		ref = gs.Branch
	}
	url := gs.ProjectRepo

	return newGitSource(url, ref, auth)
}

func newGitSource(rawURL string, ref string, auth *auto.GitAuth) (Source, error) {
	url, _, err := gitutil.ParseGitRepoURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parsing %q: %w", rawURL, err)
	}
	refSpec := config.RefSpec(fmt.Sprintf("+%s:%s", ref, ref))

	fs := memory.NewStorage()
	remote := git.NewRemote(fs, &config.RemoteConfig{
		Name:  "origin",
		URLs:  []string{url},
		Fetch: []config.RefSpec{refSpec},
	})

	return &gitSource{
		fs:     fs,
		ref:    refSpec,
		remote: remote,
		auth:   auth,
	}, nil
}

type gitSource struct {
	fs     *memory.Storage
	ref    config.RefSpec
	remote *git.Remote
	auth   *auto.GitAuth
}

func (gs gitSource) CurrentCommit(ctx context.Context) (string, error) {
	// If our ref is already a commit then use it directly.
	if gs.ref.IsExactSHA1() {
		return gs.ref.Src(), nil
	}

	// Otherwise fetch the most recent commit for the ref (branch) we care
	// about.
	auth, err := gs.authMethod()
	if err != nil {
		return "", fmt.Errorf("getting auth methodL: %w", err)
	}
	err = gs.remote.FetchContext(ctx, &git.FetchOptions{
		Depth: 1, // Don't fetch the entire history.
		Auth:  auth,
	})
	if err != nil {
		return "", fmt.Errorf("fetching: %w", err)
	}
	for commit := range gs.fs.Commits {
		return commit.String(), nil
	}
	return "", fmt.Errorf("no commits found for ref %q", gs.ref.Src())
}

// authMethod translates auto.GitAuth into a go-git transport.AuthMethod.
func (gs gitSource) authMethod() (transport.AuthMethod, error) {
	if gs.auth == nil {
		return nil, nil
	}

	if gs.auth.Username != "" {
		return &http.BasicAuth{Username: gs.auth.Username, Password: gs.auth.Password}, nil
	}

	if gs.auth.PersonalAccessToken != "" {
		return &http.BasicAuth{Username: "git", Password: gs.auth.PersonalAccessToken}, nil
	}

	if gs.auth.SSHPrivateKey != "" {
		return ssh.NewPublicKeys("git", []byte(gs.auth.SSHPrivateKey), gs.auth.Password)
	}

	return nil, nil
}

// resolveGitAuth sets up the authentication option to use for the git source
// repository of the stack. If neither gitAuth or gitAuthSecret are set,
// a pointer to a zero value of GitAuth is returned — representing
// unauthenticated git access.
//
// The workspace pod is also mutated to mount these references at some
// well-known paths.
func (sess *StackReconcilerSession) resolveGitAuth(ctx context.Context) (*auto.GitAuth, error) {
	if sess.stack.GitSource == nil {
		return nil, nil
	}
	if sess.stack.GitAuthSecret != "" {
		return nil, errors.New("gitAuthSecret was deprecated and is no longer supported in v2")
	}
	if sess.stack.GitAuth == nil {
		return nil, nil
	}

	auth := &auto.GitAuth{}
	config := sess.stack.GitSource

	if config.GitAuth.SSHAuth != nil {
		privateKey, err := sess.resolveResourceRef(ctx, &config.GitAuth.SSHAuth.SSHPrivateKey)
		if err != nil {
			return auth, fmt.Errorf("resolving gitAuth SSH private key: %w", err)
		}
		auth.SSHPrivateKey = privateKey

		if config.GitAuth.SSHAuth.Password != nil {
			password, err := sess.resolveResourceRef(ctx, config.GitAuth.SSHAuth.Password)
			if err != nil {
				return auth, fmt.Errorf("resolving gitAuth SSH password: %w", err)
			}
			auth.Password = password
		}

		return auth, nil
	}

	if config.GitAuth.PersonalAccessToken != nil {
		accessToken, err := sess.resolveResourceRef(ctx, sess.stack.GitAuth.PersonalAccessToken)
		if err != nil {
			return auth, fmt.Errorf("resolving gitAuth personal access token: %w", err)
		}
		auth.PersonalAccessToken = accessToken
		return auth, nil
	}

	if config.GitAuth.BasicAuth == nil {
		return auth, errors.New("gitAuth config must specify exactly one of " +
			"'personalAccessToken', 'sshPrivateKey' or 'basicAuth'")
	}

	username, err := sess.resolveResourceRef(ctx, &sess.stack.GitAuth.BasicAuth.UserName)
	if err != nil {
		return auth, fmt.Errorf("resolving gitAuth username: %w", err)
	}

	password, err := sess.resolveResourceRef(ctx, &sess.stack.GitAuth.BasicAuth.Password)
	if err != nil {
		return auth, fmt.Errorf("resolving gitAuth password: %w", err)
	}

	auth.Username = username
	auth.Password = password
	return auth, nil
}

func (sess *StackReconcilerSession) setupWorkspaceFromGitSource(ctx context.Context, gs shared.GitSource, commit string) error {
	sess.ws.Spec.Git = &v1alpha1.GitSource{
		Ref:     commit,
		URL:     gs.ProjectRepo,
		Dir:     gs.RepoDir,
		Shallow: gs.Shallow,
	}

	if gs.GitAuth != nil {
		auth := &v1alpha1.GitAuth{}
		if gs.GitAuth.SSHAuth != nil {
			auth.SSHPrivateKey = &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: gs.GitAuth.SSHAuth.SSHPrivateKey.SecretRef.Name,
				},
				Key: gs.GitAuth.SSHAuth.SSHPrivateKey.SecretRef.Key,
			}
			if gs.GitAuth.SSHAuth.Password != nil {
				auth.Password = &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: gs.GitAuth.SSHAuth.Password.SecretRef.Name,
					},
					Key: gs.GitAuth.SSHAuth.Password.SecretRef.Key,
				}
			}
		}
		if gs.GitAuth.BasicAuth != nil {
			auth.Username = &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: gs.GitAuth.BasicAuth.UserName.SecretRef.Name,
				},
				Key: gs.GitAuth.BasicAuth.UserName.SecretRef.Key,
			}
			auth.Password = &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: gs.GitAuth.BasicAuth.Password.SecretRef.Name,
				},
				Key: gs.GitAuth.BasicAuth.Password.SecretRef.Key,
			}
		}
		if gs.GitAuth.PersonalAccessToken != nil {
			auth.Token = &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: gs.GitAuth.PersonalAccessToken.SecretRef.Name,
				},
				Key: gs.GitAuth.PersonalAccessToken.SecretRef.Key,
			}
		}
		sess.ws.Spec.Git.Auth = auth
	}

	return sess.setupWorkspace(ctx)
}
