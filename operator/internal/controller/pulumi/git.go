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
	"context"
	"errors"
	"fmt"
	"reflect"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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

	fs := memory.NewStorage()
	remote := git.NewRemote(fs, &config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})

	return &gitSource{
		fs:     fs,
		ref:    ref,
		remote: remote,
		auth:   auth,
	}, nil
}

type gitSource struct {
	fs     *memory.Storage
	ref    string
	remote *git.Remote
	auth   *auto.GitAuth
}

func (gs gitSource) CurrentCommit(ctx context.Context) (string, error) {
	// If our ref is already a commit then use it directly.
	if plumbing.IsHash(gs.ref) {
		return gs.ref, nil
	}

	// Otherwise fetch the most recent commit for the ref (branch) we care
	// about.
	auth, err := gs.authMethod()
	if err != nil {
		return "", fmt.Errorf("getting auth method: %w", err)
	}

	refs, err := gs.remote.ListContext(ctx, &git.ListOptions{
		Auth: auth,
	})
	if err != nil {
		return "", fmt.Errorf("listing: %w", err)
	}
	for _, r := range refs {
		if r.Name().String() == gs.ref || r.Name().Short() == gs.ref {
			return r.Hash().String(), nil
		}
	}

	return "", fmt.Errorf("no commits found for ref %q", gs.ref)
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
func (sess *stackReconcilerSession) resolveGitAuth(ctx context.Context) (*auto.GitAuth, error) {
	auth := &auto.GitAuth{}

	if sess.stack.GitSource == nil {
		return auth, nil // No git source to auth.
	}

	if sess.stack.GitAuthSecret == "" && sess.stack.GitAuth == nil {
		return auth, nil // No authentication.
	}

	if sess.stack.GitAuthSecret != "" {
		namespacedName := types.NamespacedName{Name: sess.stack.GitAuthSecret, Namespace: sess.namespace}

		// Fetch the named secret.
		secret := &corev1.Secret{}
		if err := sess.kubeClient.Get(ctx, namespacedName, secret); err != nil {
			sess.logger.Error(err, "Could not find secret for access to the git repository",
				"Namespace", sess.namespace, "Stack.GitAuthSecret", sess.stack.GitAuthSecret)
			return nil, err
		}

		// First check if an SSH private key has been specified.
		if sshPrivateKey, exists := secret.Data["sshPrivateKey"]; exists {
			auth.SSHPrivateKey = string(sshPrivateKey)

			if password, exists := secret.Data["password"]; exists {
				auth.Password = string(password)
			}
			// Then check if a personal access token has been specified.
		} else if accessToken, exists := secret.Data["accessToken"]; exists {
			auth.PersonalAccessToken = string(accessToken)
			// Then check if basic authentication has been specified.
		} else if username, exists := secret.Data["username"]; exists {
			if password, exists := secret.Data["password"]; exists {
				auth.Username = string(username)
				auth.Password = string(password)
			} else {
				return nil, fmt.Errorf(`creating gitAuth: no key "password" found in secret %s`, namespacedName)
			}
		}

		return auth, nil
	}

	stackAuth := sess.stack.GitAuth
	if stackAuth.SSHAuth != nil {
		privateKey, err := sess.resolveSecretResourceRef(ctx, &stackAuth.SSHAuth.SSHPrivateKey)
		if err != nil {
			return auth, fmt.Errorf("resolving gitAuth SSH private key: %w", err)
		}
		auth.SSHPrivateKey = privateKey

		if stackAuth.SSHAuth.Password != nil {
			password, err := sess.resolveSecretResourceRef(ctx, stackAuth.SSHAuth.Password)
			if err != nil {
				return auth, fmt.Errorf("resolving gitAuth SSH password: %w", err)
			}
			auth.Password = password
		}

		return auth, nil
	}

	if stackAuth.PersonalAccessToken != nil {
		accessToken, err := sess.resolveSecretResourceRef(ctx, sess.stack.GitAuth.PersonalAccessToken)
		if err != nil {
			return auth, fmt.Errorf("resolving gitAuth personal access token: %w", err)
		}
		auth.PersonalAccessToken = accessToken
		return auth, nil
	}

	if stackAuth.BasicAuth == nil {
		return auth, errors.New("gitAuth config must specify exactly one of " +
			"'personalAccessToken', 'sshPrivateKey' or 'basicAuth'")
	}

	username, err := sess.resolveSecretResourceRef(ctx, &sess.stack.GitAuth.BasicAuth.UserName)
	if err != nil {
		return auth, fmt.Errorf("resolving gitAuth username: %w", err)
	}

	password, err := sess.resolveSecretResourceRef(ctx, &sess.stack.GitAuth.BasicAuth.Password)
	if err != nil {
		return auth, fmt.Errorf("resolving gitAuth password: %w", err)
	}

	auth.Username = username
	auth.Password = password
	return auth, nil
}

func (sess *stackReconcilerSession) setupWorkspaceFromGitSource(_ context.Context, commit string) error {
	gs := sess.stack.GitSource
	if gs == nil {
		return fmt.Errorf("missing gitSource")
	}

	sess.ws.Spec.Git = &autov1alpha1.GitSource{
		Ref:     commit,
		URL:     gs.ProjectRepo,
		Dir:     gs.RepoDir,
		Shallow: gs.Shallow,
	}
	auth := &autov1alpha1.GitAuth{}

	if sess.stack.GitAuthSecret != "" {
		auth.SSHPrivateKey = &v1.SecretKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: sess.stack.GitAuthSecret,
			},
			Key:      "sshPrivateKey",
			Optional: ptr.To(true),
		}
		auth.Password = &v1.SecretKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: sess.stack.GitAuthSecret,
			},
			Key:      "password",
			Optional: ptr.To(true),
		}
		auth.Username = &v1.SecretKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: sess.stack.GitAuthSecret,
			},
			Key:      "username",
			Optional: ptr.To(true),
		}
		auth.Token = &v1.SecretKeySelector{
			LocalObjectReference: v1.LocalObjectReference{
				Name: sess.stack.GitAuthSecret,
			},
			Key:      "accessToken",
			Optional: ptr.To(true),
		}
	}

	if gs.GitAuth != nil {
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
	}

	if !reflect.DeepEqual(*auth, autov1alpha1.GitAuth{}) {
		sess.ws.Spec.Git.Auth = auth
	}

	return nil
}

// checkGitDependencies checks all git dependencies for new commits and updates the status.
// Returns true if any dependency has new commits, indicating the stack should be updated.
func (sess *stackReconcilerSession) checkGitDependencies(ctx context.Context) (bool, error) {
	if len(sess.stack.GitDependencies) == 0 {
		return false, nil
	}

	if sess.instance.Status.GitDependencies == nil {
		sess.instance.Status.GitDependencies = make(map[string]shared.GitDependencyStatus)
	}

	hasNewCommits := false

	for _, dep := range sess.stack.GitDependencies {
		// Resolve authentication for this dependency
		var auth *auto.GitAuth
		if dep.GitAuth != nil {
			resolvedAuth, err := sess.resolveGitAuthFromConfig(ctx, dep.GitAuth)
			if err != nil {
				sess.logger.Error(err, "Failed to resolve git auth for dependency",
					"dependency", dep.Name, "repository", dep.Repository)
				// Update status with error
				sess.instance.Status.GitDependencies[dep.Name] = shared.GitDependencyStatus{
					LastCheckedTime: metav1.Now(),
					Message:         fmt.Sprintf("Failed to resolve auth: %v", err),
				}
				continue
			}
			auth = resolvedAuth
		}

		// Create a git source for this dependency
		source, err := newGitSource(dep.Repository, dep.Branch, auth)
		if err != nil {
			sess.logger.Error(err, "Failed to create git source for dependency",
				"dependency", dep.Name, "repository", dep.Repository)
			sess.instance.Status.GitDependencies[dep.Name] = shared.GitDependencyStatus{
				LastCheckedTime: metav1.Now(),
				Message:         fmt.Sprintf("Failed to create git source: %v", err),
			}
			continue
		}

		// Fetch the current commit
		currentCommit, err := source.CurrentCommit(ctx)
		if err != nil {
			sess.logger.Error(err, "Failed to fetch current commit for git dependency",
				"dependency", dep.Name, "repository", dep.Repository, "branch", dep.Branch)
			sess.instance.Status.GitDependencies[dep.Name] = shared.GitDependencyStatus{
				LastCheckedTime: metav1.Now(),
				Message:         fmt.Sprintf("Failed to fetch commit: %v", err),
			}
			continue
		}

		// Get the previous status
		previousStatus, exists := sess.instance.Status.GitDependencies[dep.Name]

		// Check if commit has changed
		if !exists || previousStatus.LastSeenCommit != currentCommit {
			sess.logger.Info("Git dependency has new commits",
				"dependency", dep.Name,
				"repository", dep.Repository,
				"branch", dep.Branch,
				"previousCommit", previousStatus.LastSeenCommit,
				"currentCommit", currentCommit)
			hasNewCommits = true
		}

		// Update the status
		sess.instance.Status.GitDependencies[dep.Name] = shared.GitDependencyStatus{
			LastSeenCommit:  currentCommit,
			LastCheckedTime: metav1.Now(),
			Message:         "",
		}
	}

	return hasNewCommits, nil
}

// resolveGitAuthFromConfig resolves GitAuth from a GitAuthConfig.
// This is similar to resolveGitAuth but works with GitAuthConfig directly.
func (sess *stackReconcilerSession) resolveGitAuthFromConfig(ctx context.Context, authConfig *shared.GitAuthConfig) (*auto.GitAuth, error) {
	auth := &auto.GitAuth{}

	if authConfig.SSHAuth != nil {
		privateKey, err := sess.resolveSecretResourceRef(ctx, &authConfig.SSHAuth.SSHPrivateKey)
		if err != nil {
			return auth, fmt.Errorf("resolving SSH private key: %w", err)
		}
		auth.SSHPrivateKey = privateKey

		if authConfig.SSHAuth.Password != nil {
			password, err := sess.resolveSecretResourceRef(ctx, authConfig.SSHAuth.Password)
			if err != nil {
				return auth, fmt.Errorf("resolving SSH password: %w", err)
			}
			auth.Password = password
		}

		return auth, nil
	}

	if authConfig.PersonalAccessToken != nil {
		accessToken, err := sess.resolveSecretResourceRef(ctx, authConfig.PersonalAccessToken)
		if err != nil {
			return auth, fmt.Errorf("resolving personal access token: %w", err)
		}
		auth.PersonalAccessToken = accessToken
		return auth, nil
	}

	if authConfig.BasicAuth != nil {
		username, err := sess.resolveSecretResourceRef(ctx, &authConfig.BasicAuth.UserName)
		if err != nil {
			return auth, fmt.Errorf("resolving username: %w", err)
		}

		password, err := sess.resolveSecretResourceRef(ctx, &authConfig.BasicAuth.Password)
		if err != nil {
			return auth, fmt.Errorf("resolving password: %w", err)
		}

		auth.Username = username
		auth.Password = password
		return auth, nil
	}

	return auth, nil
}
