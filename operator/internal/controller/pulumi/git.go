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
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

// gitRepoCache provides a simple in-memory cache for cloned git repositories
// to avoid repeated full clones when checking path-based changes.
type gitRepoCache struct {
	mu    sync.RWMutex
	repos map[string]*cachedRepo
}

type cachedRepo struct {
	repo      *git.Repository
	expiresAt time.Time
}

var (
	repoCache     *gitRepoCache
	repoCacheOnce sync.Once
	// Cache TTL of 5 minutes - balances memory usage with avoiding repeated clones
	repoCacheTTL = 5 * time.Minute
)

func getRepoCache() *gitRepoCache {
	repoCacheOnce.Do(func() {
		repoCache = &gitRepoCache{
			repos: make(map[string]*cachedRepo),
		}
		// Start cleanup goroutine
		go repoCache.cleanup()
	})
	return repoCache
}

func (c *gitRepoCache) get(key string) (*git.Repository, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, ok := c.repos[key]
	if !ok {
		return nil, false
	}

	// Check if expired
	if time.Now().After(cached.expiresAt) {
		return nil, false
	}

	return cached.repo, true
}

func (c *gitRepoCache) set(key string, repo *git.Repository) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.repos[key] = &cachedRepo{
		repo:      repo,
		expiresAt: time.Now().Add(repoCacheTTL),
	}
}

func (c *gitRepoCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, cached := range c.repos {
			if now.After(cached.expiresAt) {
				delete(c.repos, key)
			}
		}
		c.mu.Unlock()
	}
}

// Source represents a source of commits.
type Source interface {
	CurrentCommit(context.Context) (string, error)
	// HasChangesInPath checks if there are changes in the specified path between two commits.
	// If path is empty, returns true (no filtering).
	HasChangesInPath(ctx context.Context, oldCommit, newCommit, path string) (bool, error)
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

// HasChangesInPath checks if there are changes in the specified path between two commits.
// If path is empty or oldCommit is empty, returns true (no filtering).
func (gs gitSource) HasChangesInPath(ctx context.Context, oldCommit, newCommit, path string) (bool, error) {
	// If no path filter or no old commit to compare, assume changes exist
	if path == "" || oldCommit == "" {
		return true, nil
	}

	// If commits are the same, no changes
	if oldCommit == newCommit {
		return false, nil
	}

	auth, err := gs.authMethod()
	if err != nil {
		return false, fmt.Errorf("getting auth method: %w", err)
	}

	// Fetch the references to get commit objects
	refs, err := gs.remote.ListContext(ctx, &git.ListOptions{
		Auth: auth,
	})
	if err != nil {
		return false, fmt.Errorf("listing references: %w", err)
	}

	// We need to fetch the commit objects, but we don't need to clone the whole repo
	// Use git log to compare the commits with path filtering
	// For now, we'll use a simpler approach: fetch both commits and compare their trees

	// Parse commit hashes
	oldHash := plumbing.NewHash(oldCommit)
	newHash := plumbing.NewHash(newCommit)

	// Fetch the specific commits using the reference list to verify they exist
	oldExists := false
	newExists := false
	for _, ref := range refs {
		if ref.Hash() == oldHash {
			oldExists = true
		}
		if ref.Hash() == newHash {
			newExists = true
		}
	}

	if !oldExists || !newExists {
		// If we can't verify both commits exist, assume changes to be safe
		return true, nil
	}

	// Now we need to actually fetch and compare the commits
	// Try to get from cache first
	repoURL := gs.remote.Config().URLs[0]
	cache := getRepoCache()
	cacheKey := repoURL // Use URL as cache key (could enhance with auth hash if needed)

	repo, cached := cache.get(cacheKey)
	if !cached {
		// Cache miss - initialize empty repository and fetch only the specific commits we need
		fs := memfs.New()
		storer := memory.NewStorage()

		// Initialize empty repository
		repo, err = git.Init(storer, fs)
		if err != nil {
			return false, fmt.Errorf("initializing repository: %w", err)
		}

		// Create remote
		_, err = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{repoURL},
		})
		if err != nil {
			return false, fmt.Errorf("creating remote: %w", err)
		}

		// Fetch only the specific commits we need with shallow depth
		// This is much more efficient than cloning the entire repo
		err = repo.FetchContext(ctx, &git.FetchOptions{
			Auth: auth,
			RefSpecs: []config.RefSpec{
				config.RefSpec(fmt.Sprintf("%s:%s", oldHash, oldHash)),
				config.RefSpec(fmt.Sprintf("%s:%s", newHash, newHash)),
			},
			Depth: 1,
			Tags:  git.NoTags,
		})
		if err != nil {
			return false, fmt.Errorf("fetching commits: %w", err)
		}

		// Store in cache for future use
		cache.set(cacheKey, repo)
	} else {
		// Cache hit - try to fetch the specific commits if not already present
		err = repo.FetchContext(ctx, &git.FetchOptions{
			Auth: auth,
			RefSpecs: []config.RefSpec{
				config.RefSpec(fmt.Sprintf("%s:%s", oldHash, oldHash)),
				config.RefSpec(fmt.Sprintf("%s:%s", newHash, newHash)),
			},
			Depth: 1,
			Tags:  git.NoTags,
		})
		// Ignore errors from fetch - we may already have the commits we need
		// The subsequent CommitObject calls will fail if commits are truly missing
	}

	// Get commit objects
	oldCommitObj, err := repo.CommitObject(oldHash)
	if err != nil {
		return false, fmt.Errorf("getting old commit %s: %w", oldCommit, err)
	}

	newCommitObj, err := repo.CommitObject(newHash)
	if err != nil {
		return false, fmt.Errorf("getting new commit %s: %w", newCommit, err)
	}

	// Get trees for both commits
	oldTree, err := oldCommitObj.Tree()
	if err != nil {
		return false, fmt.Errorf("getting old tree: %w", err)
	}

	newTree, err := newCommitObj.Tree()
	if err != nil {
		return false, fmt.Errorf("getting new tree: %w", err)
	}

	// Find changes between the trees
	changes, err := object.DiffTree(oldTree, newTree)
	if err != nil {
		return false, fmt.Errorf("computing diff: %w", err)
	}

	// Normalize the path for comparison
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")

	// Check if any changes are in the specified path
	for _, change := range changes {
		// Get the file paths from the change
		// Check both From and To in case of renames or modifications
		if change.From.Name != "" && isUnderPath(change.From.Name, path) {
			return true, nil
		}
		if change.To.Name != "" && isUnderPath(change.To.Name, path) {
			return true, nil
		}
	}

	// No changes found in the specified path
	return false, nil
}

// isUnderPath checks if filePath is under the given directory path
func isUnderPath(filePath, dirPath string) bool {
	if dirPath == "" {
		return true
	}
	// Normalize paths
	filePath = strings.TrimPrefix(filePath, "/")
	dirPath = strings.TrimPrefix(dirPath, "/")
	dirPath = strings.TrimSuffix(dirPath, "/")

	// Check if filePath starts with dirPath
	return filePath == dirPath || strings.HasPrefix(filePath, dirPath+"/")
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
		Ref:          commit,
		URL:          gs.ProjectRepo,
		Dir:          gs.RepoDir,
		Shallow:      gs.Shallow,
		Dependencies: gs.Dependencies,
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

// checkGitSources checks all git sources for new commits and updates the status.
// Returns true if any source has new commits, indicating the stack should be updated.
// Uses the same git authentication configured for the stack's projectRepo.
func (sess *stackReconcilerSession) checkGitSources(ctx context.Context, instance *pulumiv1.Stack) (bool, error) {
	if len(sess.stack.GitSources) == 0 {
		return false, nil
	}

	if instance.Status.GitSources == nil {
		instance.Status.GitSources = make(map[string]shared.GitSourceStatus)
	}

	// Resolve git authentication once - same credentials used for all sources
	auth, err := sess.resolveGitAuth(ctx)
	if err != nil {
		sess.logger.Error(err, "Failed to resolve git auth for git sources")
		// Mark all sources with auth error
		for _, src := range sess.stack.GitSources {
			instance.Status.GitSources[src.Name] = shared.GitSourceStatus{
				LastCheckedTime: metav1.Now(),
				Message:         fmt.Sprintf("Failed to resolve git auth: %v", err),
			}
		}
		return false, err
	}

	hasNewCommits := false

	for _, src := range sess.stack.GitSources {
		// Create a git source for this tracked repository using shared auth
		source, err := newGitSource(src.Repository, src.Branch, auth)
		if err != nil {
			sess.logger.Error(err, "Failed to create git source",
				"source", src.Name, "repository", src.Repository)
			instance.Status.GitSources[src.Name] = shared.GitSourceStatus{
				LastCheckedTime: metav1.Now(),
				Message:         fmt.Sprintf("Failed to create git source: %v", err),
			}
			continue
		}

		// Fetch the current commit
		currentCommit, err := source.CurrentCommit(ctx)
		if err != nil {
			sess.logger.Error(err, "Failed to fetch current commit for git source",
				"source", src.Name, "repository", src.Repository, "branch", src.Branch)
			instance.Status.GitSources[src.Name] = shared.GitSourceStatus{
				LastCheckedTime: metav1.Now(),
				Message:         fmt.Sprintf("Failed to fetch commit: %v", err),
			}
			continue
		}

		// Get the previous status
		previousStatus, exists := instance.Status.GitSources[src.Name]

		// Check if commit has changed
		if !exists || previousStatus.LastSeenCommit != currentCommit {
			sess.logger.Info("Git source has new commits",
				"source", src.Name,
				"repository", src.Repository,
				"branch", src.Branch,
				"previousCommit", previousStatus.LastSeenCommit,
				"currentCommit", currentCommit)
			hasNewCommits = true
		}

		// Update the status
		instance.Status.GitSources[src.Name] = shared.GitSourceStatus{
			LastSeenCommit:  currentCommit,
			LastCheckedTime: metav1.Now(),
			Message:         "",
		}
	}

	return hasNewCommits, nil
}
