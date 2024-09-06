// Copyright 2016-2020, Pulumi Corporation.
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
	"path/filepath"
	"strings"
	"sync"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
)

// func (sess *StackReconcilerSession) SetupWorkspaceFromGitSource(ctx context.Context, gitAuth *auto.GitAuth, source *shared.GitSource) (string, error) {
// 	repo := auto.GitRepo{
// 		URL:         source.ProjectRepo,
// 		ProjectPath: source.RepoDir,
// 		CommitHash:  source.Commit,
// 		Branch:      source.Branch,
// 		Auth:        gitAuth,
// 	}
// 	homeDir := sess.getPulumiHome()
// 	workspaceDir := sess.getWorkspaceDir()

// 	sess.logger.Debug("Setting up pulumi workspace for stack", "stack", sess.stack, "workspace", workspaceDir)
// 	// Create a new workspace.

// 	secretsProvider := auto.SecretsProvider(sess.stack.SecretsProvider)

// 	w, err := auto.NewLocalWorkspace(
// 		ctx,
// 		auto.PulumiHome(homeDir),
// 		auto.WorkDir(workspaceDir),
// 		auto.Repo(repo),
// 		secretsProvider)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to create local workspace: %w", err)
// 	}

// 	revision, err := revisionAtWorkingDir(w.WorkDir())
// 	if err != nil {
// 		return "", err
// 	}

// 	return revision, sess.setupWorkspace(ctx, w)
// }

// SetupGitAuth sets up the authentication option to use for the git source
// repository of the stack. If neither gitAuth or gitAuthSecret are set,
// a pointer to a zero value of GitAuth is returned — representing
// unauthenticated git access.
func (sess *StackReconcilerSession) SetupGitAuth(ctx context.Context) (*auto.GitAuth, error) {
	return nil, nil
	// 	gitAuth := &auto.GitAuth{}

	// 	// check that the URL is valid (and we'll use it later to check we got appropriate auth)
	// 	u, err := giturls.Parse(sess.stack.ProjectRepo)
	// 	if err != nil {
	// 		return gitAuth, err
	// 	}

	// 	if sess.stack.GitAuth != nil {

	// 		if sess.stack.GitAuth.SSHAuth != nil {
	// 			privateKey, err := sess.resolveResourceRef(ctx, &sess.stack.GitAuth.SSHAuth.SSHPrivateKey)
	// 			if err != nil {
	// 				return nil, fmt.Errorf("resolving gitAuth SSH private key: %w", err)
	// 			}
	// 			gitAuth.SSHPrivateKey = privateKey

	// 			if sess.stack.GitAuth.SSHAuth.Password != nil {
	// 				password, err := sess.resolveResourceRef(ctx, sess.stack.GitAuth.SSHAuth.Password)
	// 				if err != nil {
	// 					return nil, fmt.Errorf("resolving gitAuth SSH password: %w", err)
	// 				}
	// 				gitAuth.Password = password
	// 			}

	// 			return gitAuth, nil
	// 		}

	// 		if sess.stack.GitAuth.PersonalAccessToken != nil {
	// 			accessToken, err := sess.resolveResourceRef(ctx, sess.stack.GitAuth.PersonalAccessToken)
	// 			if err != nil {
	// 				return nil, fmt.Errorf("resolving gitAuth personal access token: %w", err)
	// 			}
	// 			gitAuth.PersonalAccessToken = accessToken
	// 			return gitAuth, nil
	// 		}

	// 		if sess.stack.GitAuth.BasicAuth == nil {
	// 			return nil, errors.New("gitAuth config must specify exactly one of " +
	// 				"'personalAccessToken', 'sshPrivateKey' or 'basicAuth'")
	// 		}

	// 		userName, err := sess.resolveResourceRef(ctx, &sess.stack.GitAuth.BasicAuth.UserName)
	// 		if err != nil {
	// 			return nil, fmt.Errorf("resolving gitAuth username: %w", err)
	// 		}

	// 		password, err := sess.resolveResourceRef(ctx, &sess.stack.GitAuth.BasicAuth.Password)
	// 		if err != nil {
	// 			return nil, fmt.Errorf("resolving gitAuth password: %w", err)
	// 		}

	// 		gitAuth.Username = userName
	// 		gitAuth.Password = password
	// 	} else if sess.stack.GitAuthSecret != "" {
	// 		namespacedName := types.NamespacedName{Name: sess.stack.GitAuthSecret, Namespace: sess.namespace}

	// 		// Fetch the named secret.
	// 		secret := &corev1.Secret{}
	// 		if err := sess.kubeClient.Get(ctx, namespacedName, secret); err != nil {
	// 			sess.logger.Error(err, "Could not find secret for access to the git repository",
	// 				"Namespace", sess.namespace, "Stack.GitAuthSecret", sess.stack.GitAuthSecret)
	// 			return nil, err
	// 		}

	// 		// First check if an SSH private key has been specified.
	// 		if sshPrivateKey, exists := secret.Data["sshPrivateKey"]; exists {
	// 			gitAuth = &auto.GitAuth{
	// 				SSHPrivateKey: string(sshPrivateKey),
	// 			}

	// 			if password, exists := secret.Data["password"]; exists {
	// 				gitAuth.Password = string(password)
	// 			}
	// 			// Then check if a personal access token has been specified.
	// 		} else if accessToken, exists := secret.Data["accessToken"]; exists {
	// 			gitAuth = &auto.GitAuth{
	// 				PersonalAccessToken: string(accessToken),
	// 			}
	// 			// Then check if basic authentication has been specified.
	// 		} else if username, exists := secret.Data["username"]; exists {
	// 			if password, exists := secret.Data["password"]; exists {
	// 				gitAuth = &auto.GitAuth{
	// 					Username: string(username),
	// 					Password: string(password),
	// 				}
	// 			} else {
	// 				return nil, errors.New("creating gitAuth: missing 'password' secret entry")
	// 			}
	// 		}
	// 	}

	// 	if u.Scheme == "ssh" && gitAuth.SSHPrivateKey == "" {
	// 		return gitAuth, fmt.Errorf("a private key must be provided for SSH")
	// 	}

	// 	return gitAuth, nil

}

var transportMutex sync.Mutex

func setupGitRepo(ctx context.Context, workDir string, repoArgs *auto.GitRepo) (string, error) {
	cloneOptions := &git.CloneOptions{
		RemoteName: "origin", // be explicit so we can require it in remote refs
		URL:        repoArgs.URL,
	}

	if repoArgs.Shallow {
		cloneOptions.Depth = 1
		cloneOptions.SingleBranch = true
	}

	if repoArgs.Auth != nil {
		authDetails := repoArgs.Auth
		// Each of the authentication options are mutually exclusive so let's check that only 1 is specified
		if authDetails.SSHPrivateKeyPath != "" && authDetails.Username != "" ||
			authDetails.PersonalAccessToken != "" && authDetails.Username != "" ||
			authDetails.PersonalAccessToken != "" && authDetails.SSHPrivateKeyPath != "" ||
			authDetails.Username != "" && authDetails.SSHPrivateKey != "" {
			return "", errors.New("please specify one authentication option of `Personal Access Token`, " +
				"`Username\\Password`, `SSH Private Key Path` or `SSH Private Key`")
		}

		// Firstly we will try to check that an SSH Private Key Path has been specified
		if authDetails.SSHPrivateKeyPath != "" {
			publicKeys, err := ssh.NewPublicKeysFromFile("git", repoArgs.Auth.SSHPrivateKeyPath, repoArgs.Auth.Password)
			if err != nil {
				return "", fmt.Errorf("unable to use SSH Private Key Path: %w", err)
			}

			cloneOptions.Auth = publicKeys
		}

		// Then we check if the details of a SSH Private Key as passed
		if authDetails.SSHPrivateKey != "" {
			publicKeys, err := ssh.NewPublicKeys("git", []byte(repoArgs.Auth.SSHPrivateKey), repoArgs.Auth.Password)
			if err != nil {
				return "", fmt.Errorf("unable to use SSH Private Key: %w", err)
			}

			cloneOptions.Auth = publicKeys
		}

		// Then we check to see if a Personal Access Token has been specified
		// the username for use with a PAT can be *anything* but an empty string
		// so we are setting this to `git`
		if authDetails.PersonalAccessToken != "" {
			cloneOptions.Auth = &http.BasicAuth{
				Username: "git",
				Password: repoArgs.Auth.PersonalAccessToken,
			}
		}

		// then we check to see if a username and a password has been specified
		if authDetails.Password != "" && authDetails.Username != "" {
			cloneOptions.Auth = &http.BasicAuth{
				Username: repoArgs.Auth.Username,
				Password: repoArgs.Auth.Password,
			}
		}
	}

	// *Repository.Clone() will do appropriate fetching given a branch name. We must deal with
	// different varieties, since people have been advised to use these as a workaround while only
	// "refs/heads/<default>" worked.
	//
	// If a reference name is not supplied, then .Clone will fetch all refs (and all objects
	// referenced by those), and checking out a commit later will work as expected.
	if repoArgs.Branch != "" {
		refName := plumbing.ReferenceName(repoArgs.Branch)
		switch {
		case refName.IsRemote(): // e.g., refs/remotes/origin/branch
			shorter := refName.Short() // this gives "origin/branch"
			parts := strings.SplitN(shorter, "/", 2)
			if len(parts) == 2 && parts[0] == "origin" {
				refName = plumbing.NewBranchReferenceName(parts[1])
			} else {
				return "", fmt.Errorf("a remote ref must begin with 'refs/remote/origin/', but got %q", repoArgs.Branch)
			}
		case refName.IsTag(): // looks like `refs/tags/v1.0.0` -- respect this even though the field is `.Branch`
			// nothing to do
		case !refName.IsBranch(): // not a remote, not refs/heads/branch; treat as a simple branch name
			refName = plumbing.NewBranchReferenceName(repoArgs.Branch)
		default:
			// already looks like a full branch name, so use as is
		}
		cloneOptions.ReferenceName = refName
	}

	// Azure DevOps requires multi_ack and multi_ack_detailed capabilities, which go-git doesn't implement.
	// But: it's possible to do a full clone by saying it's _not_ _un_supported, in which case the library
	// happily functions so long as it doesn't _actually_ get a multi_ack packet. See
	// https://github.com/go-git/go-git/blob/v5.5.1/_examples/azure_devops/main.go.
	repo, err := func() (*git.Repository, error) {
		// Because transport.UnsupportedCapabilities is a global variable, we need a global lock around the
		// use of this.
		transportMutex.Lock()
		defer transportMutex.Unlock()

		oldUnsupportedCaps := transport.UnsupportedCapabilities
		// This check is crude, but avoids having another dependency to parse the git URL.
		if strings.Contains(repoArgs.URL, "dev.azure.com") {
			transport.UnsupportedCapabilities = []capability.Capability{
				capability.ThinPack,
			}
		}

		// clone
		repo, err := git.PlainCloneContext(ctx, workDir, false, cloneOptions)

		// Regardless of error we need to restore the UnsupportedCapabilities
		transport.UnsupportedCapabilities = oldUnsupportedCaps
		return repo, err
	}()
	if err != nil {
		return "", fmt.Errorf("unable to clone repo: %w", err)
	}

	if repoArgs.CommitHash != "" {
		// ensure that the commit has been fetched
		err := func() error {
			// repo.FetchContext ends up looking at the global transport.UnsupportedCapabilities, so we need a
			// global lock around the use of this.
			transportMutex.Lock()
			defer transportMutex.Unlock()
			return repo.FetchContext(ctx, &git.FetchOptions{
				RemoteName: "origin",
				Auth:       cloneOptions.Auth,
				Depth:      cloneOptions.Depth,
				RefSpecs:   []config.RefSpec{config.RefSpec(repoArgs.CommitHash + ":" + repoArgs.CommitHash)},
			})
		}()
		if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) && !errors.Is(err, git.ErrExactSHA1NotSupported) {
			return "", fmt.Errorf("fetching commit: %w", err)
		}

		// checkout commit if specified
		w, err := repo.Worktree()
		if err != nil {
			return "", err
		}

		hash := repoArgs.CommitHash
		err = w.Checkout(&git.CheckoutOptions{
			Hash:  plumbing.NewHash(hash),
			Force: true,
		})
		if err != nil {
			return "", fmt.Errorf("unable to checkout commit: %w", err)
		}
	}

	var relPath string
	if repoArgs.ProjectPath != "" {
		relPath = repoArgs.ProjectPath
	}

	workDir = filepath.Join(workDir, relPath)
	return workDir, nil
}
