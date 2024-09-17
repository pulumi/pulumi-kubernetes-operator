package cmd

import (
	"context"
	"testing"

	"github.com/go-git/go-git/v5"
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

	code := runInit(context.Background(), log, dir, f, nil)
	assert.Equal(t, 0, code)
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

	code := runInit(context.Background(), log, dir, nil, g)
	assert.Equal(t, 0, code)

	code = runInit(context.Background(), log, dir, nil, g)
	assert.Equal(t, 0, code)
}

func TestValidation(t *testing.T) {
	log := zap.L().Named(t.Name()).Sugar()
	code := runInit(context.Background(), log, t.TempDir(), nil, nil)
	assert.Equal(t, 1, code)
}
