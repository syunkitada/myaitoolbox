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
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "task.md"),
		[]byte("---\nstatus: doing\n---\n\n# Task\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.txt"),
		[]byte("---\nstatus: done\n---\n\nplain"), 0o644))

	entries, err := NewFileRepository(root).Tree(context.Background(), true)
	require.NoError(t, err)

	byPath := map[string]domain.FileEntry{}
	for _, e := range entries {
		byPath[e.Path] = e
	}

	assert.Equal(t, "", byPath["README.md"].Status)
	assert.Equal(t, "doing", byPath["docs/task.md"].Status)
	assert.Equal(t, "", byPath["notes.txt"].Status)
	assert.Equal(t, "", byPath["docs"].Status)
}

func TestFileRepositoryTreeStatusInvalidFrontMatter(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "broken.md"),
		[]byte("---\nstatus: [unclosed\n---\n\nbody"), 0o644))

	entries, err := NewFileRepository(root).Tree(context.Background(), true)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "", entries[0].Status)
}

func TestFileRepositoryMarkdownTags(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "nested"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".hidden"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"),
		[]byte("---\ntags: [go, docs]\n---\n\n# README\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "nested", "guide.markdown"),
		[]byte("---\ntags:\n  - docs\n  - guide\n---\n\n# Guide\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".hidden", "secret.md"),
		[]byte("---\ntags: [secret]\n---\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.txt"),
		[]byte("---\ntags: [ignored]\n---\n"), 0o644))

	tags, err := NewFileRepository(root).MarkdownTags(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"docs", "go", "guide"}, tags)
}

func TestFileRepositoryMarkdownTagsSkipsBrokenFrontMatter(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "valid.md"),
		[]byte("---\ntags: [keep]\n---\n\n# Valid\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "broken.md"),
		[]byte("---\nname: senpai\ndescription: multi line with Examples:\n<example>\nContext: not allowed\n</example>\n---\n"), 0o644))

	tags, err := NewFileRepository(root).MarkdownTags(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"keep"}, tags)
}

func TestFileRepositoryExecutable(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "run.sh"), []byte("#!/bin/sh\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "plain.txt"), []byte("text\n"), 0o644))

	repo := NewFileRepository(root)
	entries, err := repo.Tree(context.Background(), true)
	require.NoError(t, err)
	byPath := map[string]domain.FileEntry{}
	for _, e := range entries {
		byPath[e.Path] = e
	}
	assert.True(t, byPath["scripts/run.sh"].Executable)
	assert.False(t, byPath["plain.txt"].Executable)
	assert.False(t, byPath["scripts"].Executable)
}

func TestFileRepositoryExecute(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "hello.sh"),
		[]byte("#!/bin/sh\nprintf 'hi %s\\n' you\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "plain.txt"), []byte("text\n"), 0o644))

	repo := NewFileRepository(root)
	res, err := repo.Execute(context.Background(), "hello.sh")
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
	assert.Equal(t, "hi you\n", res.Output)
	assert.False(t, res.TimedOut)

	_, err = repo.Execute(context.Background(), "plain.txt")
	assert.ErrorIs(t, err, domain.ErrInvalidPath)

	_, err = repo.Execute(context.Background(), "missing.sh")
	assert.ErrorIs(t, err, domain.ErrNotFound)

	_, err = repo.Execute(context.Background(), "../escape.sh")
	assert.ErrorIs(t, err, domain.ErrInvalidPath)
}

func TestFileRepositoryChildren(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "sub"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".hidden"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "tasks", "alpha"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "tasks", "beta"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("# Project\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.txt"), []byte("text\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "task.md"), []byte("---\nstatus: doing\n---\n\n# Task\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "sub", "deep.md"), []byte("# Deep\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".hidden", "secret.md"), []byte("# Secret\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "tasks", "alpha", "task.md"), []byte("---\nstatus: done\n---\n\n# Alpha\n"), 0o644))

	repo := NewFileRepository(root)

	rootEntries, err := repo.Children(context.Background(), "", true)
	require.NoError(t, err)
	names := []string{}
	for _, e := range rootEntries {
		names = append(names, e.Name)
	}
	assert.ElementsMatch(t, []string{"docs", ".hidden", "tasks", "README.md", "notes.txt"}, names)

	docs, err := repo.Children(context.Background(), "docs", true)
	require.NoError(t, err)
	require.Len(t, docs, 2)
	byName := map[string]domain.FileEntry{}
	for _, e := range docs {
		byName[e.Name] = e
	}
	assert.Equal(t, domain.FileKindDir, byName["sub"].Kind)
	assert.Equal(t, domain.FileKindFile, byName["task.md"].Kind)
	assert.Equal(t, "doing", byName["task.md"].Status)

	tasks, err := repo.Children(context.Background(), "tasks", true)
	require.NoError(t, err)
	byName = map[string]domain.FileEntry{}
	for _, e := range tasks {
		byName[e.Name] = e
	}
	assert.Equal(t, domain.FileKindDir, byName["alpha"].Kind)
	assert.Equal(t, "done", byName["alpha"].Status)
	assert.Equal(t, "", byName["beta"].Status)

	hidden, err := repo.Children(context.Background(), "", false)
	require.NoError(t, err)
	for _, e := range hidden {
		assert.NotContains(t, e.Name, ".hidden")
	}

	escaped, err := repo.Children(context.Background(), "../escape", true)
	assert.ErrorIs(t, err, domain.ErrInvalidPath)
	assert.Nil(t, escaped)
}

func TestFileRepositoryRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "tasks", "linked"), 0o755))
	require.NoError(t, os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "tasks", "linked", "task.md")))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "link")))

	repo := NewFileRepository(root)
	_, err := repo.Content(context.Background(), "link/secret.txt")
	assert.ErrorIs(t, err, domain.ErrInvalidPath)
	assert.ErrorIs(t, repo.Save(context.Background(), "link/new.txt", "blocked"), domain.ErrInvalidPath)
	_, err = repo.Children(context.Background(), "link", true)
	assert.ErrorIs(t, err, domain.ErrInvalidPath)
	entries, err := repo.Children(context.Background(), "tasks", true)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Empty(t, entries[0].Status)
}

func TestFileRepositoryDeleteDir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "guide.md"), []byte("# Guide\n"), 0o644))

	repo := NewFileRepository(root)
	require.NoError(t, repo.Delete(context.Background(), "docs"))
	_, err := os.Stat(filepath.Join(root, "docs"))
	assert.True(t, os.IsNotExist(err))
}

func TestFileRepositoryCreateDir(t *testing.T) {
	root := t.TempDir()
	repo := NewFileRepository(root)

	require.NoError(t, repo.CreateDir(context.Background(), "docs"))
	info, err := os.Stat(filepath.Join(root, "docs"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	require.NoError(t, repo.CreateDir(context.Background(), "docs/sub/deep"))
	info, err = os.Stat(filepath.Join(root, "docs/sub/deep"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	err = repo.CreateDir(context.Background(), "docs")
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)

	err = repo.CreateDir(context.Background(), "../escape")
	assert.ErrorIs(t, err, domain.ErrInvalidPath)
}

func TestFileRepositoryMoveDir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "guide.md"), []byte("# Guide\n"), 0o644))

	repo := NewFileRepository(root)
	require.NoError(t, repo.Move(context.Background(), "docs", "notes"))

	_, err := os.Stat(filepath.Join(root, "docs"))
	assert.True(t, os.IsNotExist(err))
	data, err := os.ReadFile(filepath.Join(root, "notes", "guide.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Guide\n", string(data))
}

func TestFileRepositoryCopyDir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "guide.md"), []byte("# Guide\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "sub", "deep.md"), []byte("# Deep\n"), 0o644))

	repo := NewFileRepository(root)
	require.NoError(t, repo.Copy(context.Background(), "docs", "docs-copy"))

	data, err := os.ReadFile(filepath.Join(root, "docs-copy", "guide.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Guide\n", string(data))
	data, err = os.ReadFile(filepath.Join(root, "docs-copy", "sub", "deep.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Deep\n", string(data))
}
