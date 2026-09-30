package markdown

import (
	"bytes"
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

	assert.Equal(t, "doing", byPath["README.md"].Status)
	assert.Equal(t, "doing", byPath["docs/task.md"].Status)
	assert.Equal(t, "", byPath["notes.txt"].Status)
	assert.Equal(t, "", byPath["docs"].Status)
}

func TestFileRepositorySearch(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".hidden"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "guide.md"), []byte("Deploy deployment\nNo match\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README"), []byte("Deployment notes\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".hidden", "secret.md"), []byte("deployment\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "data.log"), []byte("deployment\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "image.png"), []byte("deployment\x00\n"), 0o644))

	repo := NewFileRepository(root)
	results, err := repo.Search(context.Background(), "DEPLOY", false)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "README", results[0].Path)
	assert.Equal(t, 1, results[0].Line)
	assert.Equal(t, "Deployment notes", results[0].Snippet)
	assert.Equal(t, 1, results[0].MatchCount)
	assert.Equal(t, "docs/guide.md", results[1].Path)
	assert.Equal(t, 1, results[1].Line)
	assert.Equal(t, 2, results[1].MatchCount)

	results, err = repo.Search(context.Background(), "deployment", true)
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, ".hidden/secret.md", results[0].Path)
}

func TestFileRepositorySearchSkipsLargeFiles(t *testing.T) {
	root := t.TempDir()
	large := filepath.Join(root, "large.txt")
	require.NoError(t, os.WriteFile(large, []byte("deployment"), 0o644))
	require.NoError(t, os.Truncate(large, maxSearchFileBytes+1))

	results, err := NewFileRepository(root).Search(context.Background(), "deployment", true)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestFileRepositorySearchLimitedRetainsTotal(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte("deployment\n"), 0o644))
	}

	results, total, err := NewFileRepository(root).SearchLimited(context.Background(), "deployment", true, 2)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, 3, total)
	assert.Equal(t, "a.md", results[0].Path)
}

func TestFileRepositoryReadLimits(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "large.txt")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))
	require.NoError(t, os.Truncate(path, maxContentFileBytes+1))

	repo := NewFileRepository(root)
	_, err := repo.Content(context.Background(), "large.txt")
	assert.ErrorIs(t, err, domain.ErrResourceTooLarge)

	rawPath := filepath.Join(root, "large-image.bin")
	require.NoError(t, os.WriteFile(rawPath, []byte("x"), 0o644))
	require.NoError(t, os.Truncate(rawPath, maxRawFileBytes+1))
	_, err = repo.Raw(context.Background(), "large-image.bin")
	assert.ErrorIs(t, err, domain.ErrResourceTooLarge)
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
	require.NoError(t, os.MkdirAll(filepath.Join(root, "_tasks", "alpha"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "_tasks", "beta"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("# Project\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.txt"), []byte("text\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "task.md"), []byte("---\nstatus: doing\n---\n\n# Task\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "sub", "deep.md"), []byte("# Deep\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".hidden", "secret.md"), []byte("# Secret\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "_tasks", "alpha", "task.md"), []byte("---\nstatus: done\n---\n\n# Alpha\n"), 0o644))

	repo := NewFileRepository(root)

	rootEntries, err := repo.Children(context.Background(), "", true)
	require.NoError(t, err)
	names := []string{}
	for _, e := range rootEntries {
		names = append(names, e.Name)
	}
	assert.ElementsMatch(t, []string{"docs", ".hidden", "_tasks", "README.md", "notes.txt"}, names)

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

	tasks, err := repo.Children(context.Background(), "_tasks", true)
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
	require.NoError(t, os.MkdirAll(filepath.Join(root, "_tasks", "linked"), 0o755))
	require.NoError(t, os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "_tasks", "linked", "task.md")))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "link")))

	repo := NewFileRepository(root)
	_, err := repo.Content(context.Background(), "link/secret.txt")
	assert.ErrorIs(t, err, domain.ErrInvalidPath)
	assert.ErrorIs(t, repo.Save(context.Background(), "link/new.txt", "blocked"), domain.ErrInvalidPath)
	_, err = repo.Children(context.Background(), "link", true)
	assert.ErrorIs(t, err, domain.ErrInvalidPath)
	entries, err := repo.Children(context.Background(), "_tasks", true)
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

func TestFileRepositorySaveReader(t *testing.T) {
	root := t.TempDir()
	repo := NewFileRepository(root)

	require.NoError(t, repo.SaveReader(context.Background(), "archives/archive.zip", bytes.NewReader([]byte("zip data"))))
	data, err := os.ReadFile(filepath.Join(root, "archives", "archive.zip"))
	require.NoError(t, err)
	assert.Equal(t, []byte("zip data"), data)
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
