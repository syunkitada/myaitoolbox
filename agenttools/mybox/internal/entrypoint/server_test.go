package entrypoint

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/application"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/entrypoint/api"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/markdown"
)

func newTestServer(t *testing.T) (*Server, *App) {
	t.Helper()
	root := t.TempDir()
	project := &domain.Project{Name: "test", Path: root}
	app := &App{
		Config:   &domain.Config{DefaultProject: "test", Projects: []domain.Project{*project}},
		Project:  project,
		Projects: application.NewProjectUseCase(&fakeConfigStore{}),
		Tasks: application.NewTaskUseCase(
			markdown.NewTaskRepository(root),
			markdown.NewTemplateRenderer(root, root),
			markdown.NewPromptRepository(root, root),
			"test",
			root,
		),
		Files: application.NewFileUseCase(markdown.NewFileRepository(root)),
		State: application.NewStateUseCase(&fakeStateStore{}),
	}
	s := NewServer(app.Config, "test", "")
	s.apps["test"] = app
	s.projects = application.NewProjectUseCase(&fakeConfigStore{})
	return s, app
}

func newTestServerWithBase(t *testing.T, basePath string) (*Server, *App) {
	t.Helper()
	s, app := newTestServer(t)
	s.basePath = normalizeBasePath(basePath)
	return s, app
}

type fakeConfigStore struct{}

func (f *fakeConfigStore) Load(ctx context.Context) (*domain.Config, error) {
	return &domain.Config{DefaultProject: "test", Projects: []domain.Project{{Name: "test", Path: "/tmp"}}}, nil
}

func (f *fakeConfigStore) Save(ctx context.Context, cfg *domain.Config) error { return nil }

func (f *fakeConfigStore) Update(ctx context.Context, fn func(*domain.Config) error) error {
	return fn(&domain.Config{DefaultProject: "test", Projects: []domain.Project{{Name: "test", Path: "/tmp"}}})
}

type fakeStateStore struct {
	state domain.State
}

func (f *fakeStateStore) Load(ctx context.Context) (*domain.State, error) {
	return &f.state, nil
}

func (f *fakeStateStore) Save(ctx context.Context, st *domain.State) error {
	f.state = *st
	return nil
}

func (f *fakeStateStore) Update(ctx context.Context, fn func(*domain.State) error) error {
	st := &domain.State{Favorites: append([]string(nil), f.state.Favorites...), RecentFiles: append([]string(nil), f.state.RecentFiles...)}
	if err := fn(st); err != nil {
		return err
	}
	f.state = *st
	return nil
}

func do(t *testing.T, s *Server, method, target string, body any, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, target, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &v))
	return v
}

func TestMetaAndLifecycle(t *testing.T) {
	s, app := newTestServer(t)
	require.NoError(t, os.WriteFile(filepath.Join(app.Project.Path, "notes.md"), []byte("---\ntags: [docs]\n---\n\n# Notes\n"), 0o644))

	rec := do(t, s, http.MethodGet, "/api/meta", nil, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	meta := decode[api.Meta](t, rec)
	assert.Equal(t, "test", meta.Project)
	assert.Equal(t, "test", meta.DefaultProject)
	assert.Contains(t, meta.Tags, "docs")

	rec = do(t, s, http.MethodPost, "/api/tasks", map[string]any{"name": "from api"})
	assert.Equal(t, http.StatusCreated, rec.Code)
	task := decode[api.Task](t, rec)
	assert.Equal(t, "from api", task.Title)
	require.NotNil(t, task.Project)
	assert.Equal(t, "test", *task.Project)

	rec = do(t, s, http.MethodPost, "/api/tasks", map[string]any{"name": ""})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTaskAPILifecycle(t *testing.T) {
	s, _ := newTestServer(t)

	rec := do(t, s, http.MethodPost, "/api/tasks", map[string]any{"name": "review PR"})
	assert.Equal(t, http.StatusCreated, rec.Code)
	task := decode[api.Task](t, rec)
	assert.Equal(t, "review PR", task.Title)

	rec = do(t, s, http.MethodPatch, "/api/tasks/"+task.Id, map[string]any{
		"description": "details",
		"assignee":    "owner",
		"tags":        []string{"review"},
	})
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = do(t, s, http.MethodPatch, "/api/tasks/"+task.Id, map[string]any{
		"description": "",
		"assignee":    "",
		"tags":        []string{},
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	updated := decode[api.Task](t, rec)
	assert.Empty(t, updated.Description)
	assert.Empty(t, updated.Assignee)
	assert.Empty(t, updated.Tags)

	rec = do(t, s, http.MethodGet, "/api/tasks", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	list := decode[[]api.Task](t, rec)
	require.Len(t, list, 1)
	assert.Equal(t, task.Id, list[0].Id)

	rec = do(t, s, http.MethodPost, "/api/tasks/"+task.Id+"/archive", nil)
	assert.Equal(t, http.StatusNoContent, rec.Code) // 完了・全タスクともアーカイブ可能
	rec = do(t, s, http.MethodGet, "/api/tasks?all=true", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	list = decode[[]api.Task](t, rec)
	require.Len(t, list, 1)
	assert.True(t, *list[0].Archived)
}

func TestMetaUnselectedProject(t *testing.T) {
	s, _ := newTestServer(t)

	rec := do(t, s, http.MethodGet, "/api/meta", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	meta := decode[api.Meta](t, rec)
	assert.Equal(t, "", meta.Project)
	assert.Contains(t, meta.Projects, "test")
}

func TestProjectsAPI(t *testing.T) {
	s, _ := newTestServer(t)

	rec := do(t, s, http.MethodGet, "/api/projects", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	projects := decode[[]api.Project](t, rec)
	assert.Contains(t, projects, api.Project{Name: "test", Path: "/tmp"})

	// Path candidates only contain existing directories.
	dir := t.TempDir()
	rec = do(t, s, http.MethodGet, "/api/projects/paths?prefix="+dir, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	paths := decode[[]string](t, rec)
	require.Contains(t, paths, dir)

	rec = do(t, s, http.MethodGet, "/api/projects/paths?prefix="+filepath.Join(dir, "nope"), nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, decode[[]string](t, rec))

	// Creating a project from a non-existent path is rejected.
	rec = do(t, s, http.MethodPost, "/api/projects", map[string]any{"path": filepath.Join(dir, "missing")})
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = do(t, s, http.MethodPost, "/api/projects", map[string]any{"path": dir})
	assert.Equal(t, http.StatusCreated, rec.Code)
	project := decode[api.Project](t, rec)
	assert.Equal(t, filepath.Base(dir), project.Name)
	assert.Equal(t, dir, project.Path)

	rec = do(t, s, http.MethodDelete, "/api/projects/does-not-exist", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = do(t, s, http.MethodDelete, "/api/projects/test", nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestFavoritesAndRecent(t *testing.T) {
	s, _ := newTestServer(t)

	rec := do(t, s, http.MethodPut, "/api/meta/favorites", map[string]any{"path": "notes/n1", "enabled": true})
	assert.Equal(t, http.StatusNoContent, rec.Code)

	rec = do(t, s, http.MethodPost, "/api/meta/recent", map[string]any{"path": "notes/n1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)

	rec = do(t, s, http.MethodGet, "/api/meta", nil, "X-Project", "test")
	meta := decode[api.Meta](t, rec)
	assert.Contains(t, meta.Favorites, "notes/n1")
	assert.Contains(t, meta.RecentFiles, "notes/n1")

	rec = do(t, s, http.MethodPost, "/api/meta/recent/delete", map[string]any{"path": "notes/n1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)

	rec = do(t, s, http.MethodGet, "/api/meta", nil, "X-Project", "test")
	meta = decode[api.Meta](t, rec)
	assert.NotContains(t, meta.RecentFiles, "notes/n1")
}

func TestNotFoundAndTraversal(t *testing.T) {
	s, _ := newTestServer(t)

	rec := do(t, s, http.MethodGet, "/api/nope", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)

}

func TestFilesCreateDir(t *testing.T) {
	s, app := newTestServer(t)
	root := app.Project.Path

	rec := do(t, s, http.MethodPost, "/api/files/dir",
		api.FilePathRequest{Path: "docs/sub"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
	info, err := os.Stat(filepath.Join(root, "docs", "sub"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	rec = do(t, s, http.MethodPost, "/api/files/dir",
		api.FilePathRequest{Path: "docs"})
	assert.Equal(t, http.StatusConflict, rec.Code)

	rec = do(t, s, http.MethodPost, "/api/files/dir",
		api.FilePathRequest{Path: "../../etc/passwd"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)

}

func TestFilesUpload(t *testing.T) {
	s, app := newTestServer(t)
	root := app.Project.Path
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs"), 0o755))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("directory", "docs"))
	part, err := writer.CreateFormFile("files", "uploaded.bin")
	require.NoError(t, err)
	_, err = part.Write([]byte{0x00, 0x01, 0xff, 0x7f})
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/files/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	got, err := os.ReadFile(filepath.Join(root, "docs", "uploaded.bin"))
	require.NoError(t, err)
	assert.Equal(t, []byte{0x00, 0x01, 0xff, 0x7f}, got)

}

func TestFiles(t *testing.T) {
	s, app := newTestServer(t)
	root := app.Project.Path
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("# Project\n\nHello.\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "task.md"), []byte("---\nstatus: doing\n---\n\n# Task\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".hidden"), []byte("x"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git", "config"), []byte("x"), 0o644))

	rec := do(t, s, http.MethodGet, "/api/files?show_hidden=false", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	files := decode[[]api.FileEntry](t, rec)
	require.Len(t, files, 3)
	assert.Equal(t, api.FileEntry{Path: "docs", Name: "docs", Kind: api.FileEntryKind("dir")}, files[0])
	assert.Equal(t, api.FileEntry{Path: "README.md", Name: "README.md", Kind: api.FileEntryKind("file")}, files[1])
	status := "doing"
	assert.Equal(t, api.FileEntry{Path: "docs/task.md", Name: "task.md", Kind: api.FileEntryKind("file"), Status: &status}, files[2])

	rec = do(t, s, http.MethodGet, "/api/files", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	files = decode[[]api.FileEntry](t, rec)
	require.Len(t, files, 6)
	assert.Equal(t, api.FileEntry{Path: ".git", Name: ".git", Kind: api.FileEntryKind("dir")}, files[0])
	assert.Equal(t, api.FileEntry{Path: "docs", Name: "docs", Kind: api.FileEntryKind("dir")}, files[1])
	assert.Equal(t, api.FileEntry{Path: ".git/config", Name: "config", Kind: api.FileEntryKind("file")}, files[2])
	assert.Equal(t, api.FileEntry{Path: ".hidden", Name: ".hidden", Kind: api.FileEntryKind("file")}, files[3])
	assert.Equal(t, api.FileEntry{Path: "README.md", Name: "README.md", Kind: api.FileEntryKind("file")}, files[4])
	assert.Equal(t, api.FileEntry{Path: "docs/task.md", Name: "task.md", Kind: api.FileEntryKind("file"), Status: &status}, files[5])

	rec = do(t, s, http.MethodGet, "/api/files/content?path=README.md", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	content := decode[api.FileContent](t, rec)
	assert.Equal(t, "# Project\n\nHello.\n", content.Content)

	rec = do(t, s, http.MethodGet, "/api/files/content?path=nope.md", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = do(t, s, http.MethodGet, "/api/files/content?path=..%2f..%2fetc%2fpasswd", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = do(t, s, http.MethodGet, "/api/files/content?path=docs", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	require.NoError(t, os.MkdirAll(filepath.Join(root, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "assets", "logo.png"),
		[]byte("\x89PNG\r\n\x1a\nfakepng"), 0o644))
	rec = do(t, s, http.MethodGet, "/api/files/raw?path=assets%2Flogo.png", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	assert.Equal(t, "\x89PNG\r\n\x1a\nfakepng", rec.Body.String())

	rec = do(t, s, http.MethodGet, "/api/files/raw?path=assets%2Fmissing.png", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = do(t, s, http.MethodGet, "/api/files/raw?path=..%2f..%2fetc%2fpasswd", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = do(t, s, http.MethodGet, "/api/files/raw?path=docs", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = do(t, s, http.MethodPut, "/api/files/content",
		api.FileContent{Path: "docs/guide.md", Content: "# Guide\n\nUpdated.\n"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
	got, err := os.ReadFile(filepath.Join(root, "docs", "guide.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Guide\n\nUpdated.\n", string(got))

	rec = do(t, s, http.MethodPut, "/api/files/content",
		api.FileContent{Path: "docs/new.md", Content: "# New\n"})
	assert.Equal(t, http.StatusNoContent, rec.Code)

	rec = do(t, s, http.MethodPut, "/api/files/content",
		api.FileContent{Path: "../../etc/passwd", Content: "x"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = do(t, s, http.MethodPut, "/api/files/content",
		api.FileContent{Path: "docs", Content: "x"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = do(t, s, http.MethodPost, "/api/files/move",
		api.MoveFileRequest{OldPath: "docs/guide.md", NewPath: "notes/guide.md"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
	_, err = os.Stat(filepath.Join(root, "docs", "guide.md"))
	assert.True(t, os.IsNotExist(err))
	got, err = os.ReadFile(filepath.Join(root, "notes", "guide.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Guide\n\nUpdated.\n", string(got))

	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "sub"), 0o755))
	rec = do(t, s, http.MethodPost, "/api/files/move",
		api.MoveFileRequest{OldPath: "docs/new.md", NewPath: "docs/sub"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
	_, err = os.Stat(filepath.Join(root, "docs", "sub", "new.md"))
	require.NoError(t, err)

	rec = do(t, s, http.MethodPost, "/api/files/move",
		api.MoveFileRequest{OldPath: "README.md", NewPath: "../../etc/passwd"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = do(t, s, http.MethodPost, "/api/files/move",
		api.MoveFileRequest{OldPath: "nope.md", NewPath: "notes/x.md"})
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = do(t, s, http.MethodPost, "/api/files/copy",
		api.MoveFileRequest{OldPath: "notes/guide.md", NewPath: "notes/guide-copy.md"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
	got, err = os.ReadFile(filepath.Join(root, "notes", "guide-copy.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Guide\n\nUpdated.\n", string(got))

	rec = do(t, s, http.MethodPost, "/api/files/copy",
		api.MoveFileRequest{OldPath: "notes/guide.md", NewPath: "notes/guide-copy.md"})
	assert.Equal(t, http.StatusConflict, rec.Code)

	rec = do(t, s, http.MethodPost, "/api/files/delete",
		api.FilePathRequest{Path: "notes/guide-copy.md"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
	_, err = os.Stat(filepath.Join(root, "notes", "guide-copy.md"))
	assert.True(t, os.IsNotExist(err))

	rec = do(t, s, http.MethodPost, "/api/files/delete",
		api.FilePathRequest{Path: "notes/guide-copy.md"})
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = do(t, s, http.MethodPost, "/api/files/delete",
		api.FilePathRequest{Path: "../../etc/passwd"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = do(t, s, http.MethodPost, "/api/files/delete",
		api.FilePathRequest{Path: "notes"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
	_, err = os.Stat(filepath.Join(root, "notes"))
	assert.True(t, os.IsNotExist(err))
}

func TestFilesExecutableFlag(t *testing.T) {
	s, app := newTestServer(t)
	root := app.Project.Path
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "run.sh"), []byte("#!/bin/sh\necho hi\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("# Project\n"), 0o644))

	rec := do(t, s, http.MethodGet, "/api/files?show_hidden=false", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	files := decode[[]api.FileEntry](t, rec)
	byPath := map[string]api.FileEntry{}
	for _, f := range files {
		byPath[f.Path] = f
	}
	run, ok := byPath["scripts/run.sh"]
	require.True(t, ok, "scripts/run.sh should be listed")
	require.NotNil(t, run.Executable)
	assert.True(t, *run.Executable)

	readme, ok := byPath["README.md"]
	require.True(t, ok)
	assert.Nil(t, readme.Executable)
}

func TestFilesExecute(t *testing.T) {
	s, app := newTestServer(t)
	root := app.Project.Path
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "hello.sh"), []byte("#!/bin/sh\nprintf 'hello %s\\n' world\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("# Project\n"), 0o644))

	rec := do(t, s, http.MethodPost, "/api/files/execute", api.FilePathRequest{Path: "scripts/hello.sh"})
	assert.Equal(t, http.StatusOK, rec.Code)
	res := decode[api.FileExecuteResult](t, rec)
	assert.Equal(t, "scripts/hello.sh", res.Path)
	assert.Equal(t, 0, res.ExitCode)
	assert.Equal(t, "hello world\n", res.Output)
	assert.Nil(t, res.TimedOut)

	rec = do(t, s, http.MethodPost, "/api/files/execute", api.FilePathRequest{Path: "scripts/fail.sh"})
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = do(t, s, http.MethodPost, "/api/files/execute", api.FilePathRequest{Path: "../../etc/passwd"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = do(t, s, http.MethodPost, "/api/files/execute", api.FilePathRequest{Path: "README.md"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = do(t, s, http.MethodPost, "/api/files/execute", api.FilePathRequest{Path: "scripts"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateFile(t *testing.T) {
	s, app := newTestServer(t)
	root := app.Project.Path

	rec := do(t, s, http.MethodPost, "/api/files", api.FilePathRequest{Path: "notes/idea.md"})
	assert.Equal(t, http.StatusNoContent, rec.Code)
	_, err := os.Stat(filepath.Join(root, "notes", "idea.md"))
	require.NoError(t, err)

	rec = do(t, s, http.MethodPost, "/api/files", api.FilePathRequest{Path: "notes/idea.md"})
	assert.Equal(t, http.StatusConflict, rec.Code)

	rec = do(t, s, http.MethodPost, "/api/files", api.FilePathRequest{Path: "../../etc/passwd"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs"), 0o755))
	rec = do(t, s, http.MethodPost, "/api/files", api.FilePathRequest{Path: "docs"})
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestBasePath(t *testing.T) {
	s, _ := newTestServerWithBase(t, "/mybox")

	rec := do(t, s, http.MethodGet, "/mybox/api/meta", nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	root := s.apps["test"].Project.Path
	require.NoError(t, os.MkdirAll(filepath.Join(root, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "assets", "logo.png"), []byte("png"), 0o644))
	rec = do(t, s, http.MethodGet, "/mybox/api/files/raw?path=assets%2Flogo.png", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "image/png", rec.Header().Get("Content-Type"))

	rec = do(t, s, http.MethodGet, "/mybox/", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `window.__MYBOX_BASE__="/mybox"`)
	assert.Contains(t, rec.Body.String(), `<base href="/mybox/">`)

	rec = do(t, s, http.MethodGet, "/mybox/test/", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `window.__MYBOX_BASE__="/mybox"`)
	assert.Contains(t, rec.Body.String(), `<base href="/mybox/">`)

	rec = do(t, s, http.MethodGet, "/mybox/projects/test/dashboard/files/agenttools/README.md", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Body.String(), `<div id="root">`)

	assets, err := fs.ReadDir(webDist, "assets")
	require.NoError(t, err)
	require.NotEmpty(t, assets)
	rec = do(t, s, http.MethodGet, "/mybox/projects/test/assets/"+assets[0].Name(), nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = do(t, s, http.MethodGet, "/", nil)
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/mybox/", rec.Header().Get("Location"))

	rec = do(t, s, http.MethodGet, "/api/meta", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNormalizeBasePath(t *testing.T) {
	assert.Equal(t, "", normalizeBasePath(""))
	assert.Equal(t, "", normalizeBasePath("/"))
	assert.Equal(t, "/mybox", normalizeBasePath("mybox"))
	assert.Equal(t, "/mybox", normalizeBasePath("/mybox/"))
}

func TestSPAAssetResolution(t *testing.T) {
	s, _ := newTestServer(t)

	assets, err := fs.ReadDir(webDist, "assets")
	require.NoError(t, err)
	require.NotEmpty(t, assets)
	name := assets[0].Name()

	rec := do(t, s, http.MethodGet, "/projects/test/assets/"+name, nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = do(t, s, http.MethodGet, "/mybox/projects/test/assets/"+name, nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	// A project Markdown route must fall back to the SPA. It must not be
	// mistaken for the README.md tracked inside the embedded dist directory.
	rec = do(t, s, http.MethodGet, "/projects/test/dashboard/files/agenttools/README.md", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Body.String(), `<div id="root">`)
}
