package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/markdown"
)

func newKnowledgeUC(t *testing.T) *KnowledgeUseCase {
	t.Helper()
	root := t.TempDir()
	return NewKnowledgeUseCase(
		markdown.NewKnowledgeRepository(root),
		markdown.NewTemplateRenderer(root, ""),
	)
}

func TestKnowledgeCreateMoveRename(t *testing.T) {
	uc := newKnowledgeUC(t)
	ctx := context.Background()

	k, err := uc.Create(ctx, "golang/context")
	require.NoError(t, err)
	assert.Equal(t, "golang/context", k.Path)

	require.NoError(t, uc.Move(ctx, "golang/context", "golang/stdlib-context"))
	_, err = uc.Show(ctx, "golang/context")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	got, err := uc.Show(ctx, "golang/stdlib-context")
	require.NoError(t, err)
	assert.Equal(t, "golang/stdlib-context", got.Path)

	require.NoError(t, uc.Rename(ctx, "golang/stdlib-context", "ctx"))
	got, err = uc.Show(ctx, "golang/ctx")
	require.NoError(t, err)
	assert.Equal(t, "golang/ctx", got.Path)
}

func TestKnowledgeSaveContent(t *testing.T) {
	uc := newKnowledgeUC(t)
	ctx := context.Background()
	_, err := uc.Create(ctx, "note")
	require.NoError(t, err)

	require.NoError(t, uc.SaveContent(ctx, "note", "# Updated"))
	got, err := uc.Show(ctx, "note")
	require.NoError(t, err)
	assert.Equal(t, "# Updated", got.Body)
}

func TestKnowledgeCreateInvalidPath(t *testing.T) {
	uc := newKnowledgeUC(t)
	_, err := uc.Create(context.Background(), "../escape")
	assert.ErrorIs(t, err, domain.ErrInvalidPath)
}

func TestKnowledgeListTagFilter(t *testing.T) {
	uc := newKnowledgeUC(t)
	ctx := context.Background()
	_, err := uc.Create(ctx, "a")
	require.NoError(t, err)
	require.NoError(t, uc.SaveContent(ctx, "a", "---\ntags: [golang]\n---\n\nbody"))

	list, err := uc.List(ctx, KnowledgeFilter{Tag: "golang"})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "a", list[0].Path)

	list, err = uc.List(ctx, KnowledgeFilter{Tag: "nope"})
	require.NoError(t, err)
	assert.Empty(t, list)
}
