package markdown

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

func TestKnowledgeRepositoryLifecycle(t *testing.T) {
	root := t.TempDir()
	repo := NewKnowledgeRepository(root)
	ctx := context.Background()

	content := "---\ntitle: Hexagonal Architecture\ntags: [golang]\n---\n\n# Hexagonal\n\nSee [[port]]"
	require.NoError(t, repo.Create(ctx, "architecture/hexagonal", content))

	k, err := repo.Find(ctx, "architecture/hexagonal")
	require.NoError(t, err)
	assert.Equal(t, "Hexagonal Architecture", k.Title)
	assert.Equal(t, "architecture/hexagonal", k.Path)
	assert.Equal(t, []string{"golang"}, k.Tags)
	assert.Equal(t, []string{"port"}, k.WikiLinks)

	list, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, repo.SaveContent(ctx, "architecture/hexagonal", "# New content"))
	got, err := repo.Find(ctx, "architecture/hexagonal")
	require.NoError(t, err)
	assert.Equal(t, "# New content", got.Body)

	require.NoError(t, repo.Move(ctx, "architecture/hexagonal", "design/hexagonal"))
	_, err = repo.Find(ctx, "architecture/hexagonal")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = repo.Find(ctx, "design/hexagonal")
	require.NoError(t, err)

	require.NoError(t, repo.Rename(ctx, "design/hexagonal", "hexagon"))
	_, err = repo.Find(ctx, "design/hexagon")
	require.NoError(t, err)
}

func TestKnowledgeRepositoryRejectPathTraversal(t *testing.T) {
	repo := NewKnowledgeRepository(t.TempDir())
	ctx := context.Background()
	_, err := repo.Find(ctx, "../secret")
	assert.ErrorIs(t, err, domain.ErrInvalidPath)
	err = repo.Create(ctx, "a/../b", "content")
	assert.ErrorIs(t, err, domain.ErrInvalidPath)
	err = repo.Move(ctx, "a", "b/../../c")
	assert.ErrorIs(t, err, domain.ErrInvalidPath)
	err = repo.Rename(ctx, "a", "b/c")
	assert.ErrorIs(t, err, domain.ErrInvalidPath)
}

func TestKnowledgeRepositoryCreateDuplicate(t *testing.T) {
	repo := NewKnowledgeRepository(t.TempDir())
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, "a", "content"))
	err := repo.Create(ctx, "a", "content")
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}

func TestKnowledgeWithoutFrontMatter(t *testing.T) {
	root := t.TempDir()
	repo := NewKnowledgeRepository(root)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, "note", "# My Title\n\nbody"))
	k, err := repo.Find(ctx, "note")
	require.NoError(t, err)
	assert.Equal(t, "My Title", k.Title)
}

func TestKnowledgeExtractsMarkdownLinksRelativeToNote(t *testing.T) {
	root := t.TempDir()
	repo := NewKnowledgeRepository(root)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, "notes/guide", "see [Detail](sub/detail.md) and [Other](../other.md) and [Web](https://example.com)"))
	require.NoError(t, repo.Create(ctx, "notes/other", "body"))

	k, err := repo.Find(ctx, "notes/guide")
	require.NoError(t, err)
	assert.Equal(t, []string{"notes/sub/detail", "other"}, k.MarkdownLinks)
}

func TestKnowledgeExtractsDirectoryLinks(t *testing.T) {
	root := t.TempDir()
	repo := NewKnowledgeRepository(root)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, "docs/index", "see [config](./xdgconfig/) and [sub](../sub/)"))
	require.NoError(t, repo.Create(ctx, "docs/xdgconfig/README", "body"))

	k, err := repo.Find(ctx, "docs/index")
	require.NoError(t, err)
	assert.Equal(t, []string{"docs/xdgconfig", "sub"}, k.MarkdownLinks)
}
