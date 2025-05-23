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
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
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
