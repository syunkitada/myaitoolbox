package markdown

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

func TestFileRepositoryTreeStatus(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"),
		[]byte("---\nstatus: doing\n---\n\n# Project\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "guide.md"),
		[]byte("# Guide\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.txt"),
		[]byte("---\nstatus: done\n---\n\nplain"), 0o644))

	entries, err := NewFileRepository(root).Tree(context.Background())
	require.NoError(t, err)

	byPath := map[string]domain.FileEntry{}
	for _, e := range entries {
		byPath[e.Path] = e
	}

	assert.Equal(t, "doing", byPath["README.md"].Status)
	assert.Equal(t, "", byPath["docs/guide.md"].Status)
	assert.Equal(t, "", byPath["notes.txt"].Status)
	assert.Equal(t, "", byPath["docs"].Status)
}

func TestFileRepositoryTreeStatusInvalidFrontMatter(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "broken.md"),
		[]byte("---\nstatus: [unclosed\n---\n\nbody"), 0o644))

	entries, err := NewFileRepository(root).Tree(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "", entries[0].Status)
}
