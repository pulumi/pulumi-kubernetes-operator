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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestInitFluxSource(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	log := zap.L().Named(t.Name()).Sugar()

	url := "https://github.com/pulumi/examples.git"
	digest := "sha256:bcbed45526b241ab3366707b5a58c900e9d60a1d5c385cdfe976b1306584b454"

	ctrl := gomock.NewController(t)
	f := NewMockfetchWithContexter(ctrl)
	f.EXPECT().URL().Return(url).AnyTimes()
	f.EXPECT().Digest().Return(digest).AnyTimes()
	f.EXPECT().FetchWithContext(gomock.Any(), url, digest, dir).Return(nil)

	err := runInit(t.Context(), log, dir, f, nil)
	assert.NoError(t, err)
}

func TestFluxUntarOptions(t *testing.T) {
	t.Run("unset does not override the default", func(t *testing.T) {
		t.Setenv("FLUX_MAX_UNTAR_SIZE_BYTES", "")
		opts, err := fluxUntarOptions()
		assert.NoError(t, err)
		assert.Nil(t, opts)
	})

	t.Run("a valid size is accepted", func(t *testing.T) {
		t.Setenv("FLUX_MAX_UNTAR_SIZE_BYTES", "524288000")
		opts, err := fluxUntarOptions()
		assert.NoError(t, err)
		assert.NotEmpty(t, opts)
	})

	t.Run("a non-numeric value errors", func(t *testing.T) {
		t.Setenv("FLUX_MAX_UNTAR_SIZE_BYTES", "big")
		_, err := fluxUntarOptions()
		assert.Error(t, err)
	})
}

func TestInitFluxSource_UntarSizeLimit(t *testing.T) {
	const extractedSize = 2048

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	content := bytes.Repeat([]byte("a"), extractedSize)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "data.txt",
		Mode:     0o600,
		Size:     int64(len(content)),
	}))
	_, err := tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	archive := buf.Bytes()

	sum := sha256.Sum256(archive)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	}))
	t.Cleanup(srv.Close)

	fetchInto := func(t *testing.T, targetDir string) error {
		f, err := newFluxFetcher(srv.URL, digest)
		require.NoError(t, err)
		return runInit(t.Context(), zap.L().Named(t.Name()).Sugar(), targetDir, f, nil)
	}

	t.Run("fails when the artifact exceeds the configured limit", func(t *testing.T) {
		t.Setenv("FLUX_MAX_UNTAR_SIZE_BYTES", "1024")
		err := fetchInto(t, t.TempDir())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bigger than max archive size")
	})

	t.Run("succeeds when the limit is raised above the artifact size", func(t *testing.T) {
		t.Setenv("FLUX_MAX_UNTAR_SIZE_BYTES", "1048576")
		dir := t.TempDir()
		require.NoError(t, fetchInto(t, dir))
		extracted, err := os.ReadFile(filepath.Join(dir, "data.txt"))
		require.NoError(t, err)
		assert.Len(t, extracted, extractedSize)
	})
}

func TestWriteProjectFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	err := writeProjectFile(dir, "myproject", "yaml")
	assert.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "Pulumi.yaml"))
	assert.NoError(t, err)
	assert.Contains(t, string(data), "name: myproject")
	assert.Contains(t, string(data), "runtime: yaml")
}

func TestInitGitSource(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	log := zap.L().Named(t.Name()).Sugar()

	ctrl := gomock.NewController(t)
	g := NewMocknewLocalWorkspacer(ctrl)
	g.EXPECT().URL().Return("https://github.com/pulumi/examples.git").AnyTimes()
	g.EXPECT().Revision().Return("f143bd369afcb5455edb54c2b90ad7aaac719339").AnyTimes()

	// Simulate a successful pull, followed by a second unnecessary pull.
	// TODO: Check auth etc.
	gomock.InOrder(
		g.EXPECT().NewLocalWorkspace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil),
		g.EXPECT().NewLocalWorkspace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, git.ErrRepositoryAlreadyExists),
	)

	err := runInit(t.Context(), log, dir, nil, g)
	assert.NoError(t, err)

	err = runInit(t.Context(), log, dir, nil, g)
	assert.NoError(t, err)
}

func TestInitGitSourceE2E(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()

	// Copy the command so we don't mutate it.
	root := cobra.Command(*rootCmd) //nolint:unconvert // We want to copy.
	root.SetArgs([]string{
		"init",
		"--git-url=https://github.com/git-fixtures/basic",
		"--git-revision=6ecf0ef2c2dffb796033e5a02219af86ec6584e5",
		"--target-dir=" + t.TempDir(),
	})
	assert.NoError(t, root.Execute())
}
