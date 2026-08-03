package entrypoint

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/application"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/entrypoint/api"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/markdown"
)

func newTestServer(t *testing.T, readOnly bool) (*Server, *App) {
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
			"test",
		),
		Knowledge: application.NewKnowledgeUseCase(
			markdown.NewKnowledgeRepository(root),
			markdown.NewTemplateRenderer(root, root),
		),
		Search: application.NewSearchUseCase(markdown.NewSearcher(root)),
		State:  application.NewStateUseCase(&fakeStateStore{}),
	}
	s := NewServer(app.Config, "test", readOnly)
	s.apps["test"] = app
	return s, app
}

type fakeConfigStore struct{}

func (f *fakeConfigStore) Load(ctx context.Context) (*domain.Config, error) {
	return &domain.Config{DefaultProject: "test", Projects: []domain.Project{{Name: "test", Path: "/tmp"}}}, nil
}

func (f *fakeConfigStore) Save(ctx context.Context, cfg *domain.Config) error { return nil }

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

func do(t *testing.T, s *Server, method, target string, body any) *httptest.ResponseRecorder {
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
	s, _ := newTestServer(t, false)

	rec := do(t, s, http.MethodGet, "/api/meta", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	meta := decode[api.Meta](t, rec)
	assert.Equal(t, "test", meta.Project)
	assert.Equal(t, "test", meta.DefaultProject)

	rec = do(t, s, http.MethodPost, "/api/tasks", map[string]any{"name": "from api"})
	assert.Equal(t, http.StatusCreated, rec.Code)
	task := decode[api.Task](t, rec)
	assert.Equal(t, "from api", task.Title)
	require.NotNil(t, task.Project)
	assert.Equal(t, "test", *task.Project)

	rec = do(t, s, http.MethodPost, "/api/tasks", map[string]any{"name": ""})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestReadOnly(t *testing.T) {
	s, _ := newTestServer(t, true)

	rec := do(t, s, http.MethodPost, "/api/tasks", map[string]any{"name": "nope"})
	assert.Equal(t, http.StatusForbidden, rec.Code)

	rec = do(t, s, http.MethodGet, "/api/meta", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSearchAndKnowledge(t *testing.T) {
	s, app := newTestServer(t, false)
	ctx := context.Background()
	_, err := app.Tasks.Create(ctx, application.TaskInput{Name: "wire it up", Tags: []string{"api"}})
	require.NoError(t, err)
	_, err = app.Knowledge.Create(ctx, "notes/n1")
	require.NoError(t, err)

	rec := do(t, s, http.MethodGet, "/api/search?q=wire", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	results := decode[[]api.SearchResult](t, rec)
	require.Len(t, results, 1)
	assert.Equal(t, api.SearchResultType("task"), results[0].Type)

	rec = do(t, s, http.MethodGet, "/api/knowledge?tag=api", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestFavoritesAndRecent(t *testing.T) {
	s, _ := newTestServer(t, false)

	rec := do(t, s, http.MethodPut, "/api/meta/favorites", map[string]any{"path": "notes/n1", "enabled": true})
	assert.Equal(t, http.StatusNoContent, rec.Code)

	rec = do(t, s, http.MethodPost, "/api/meta/recent", map[string]any{"path": "notes/n1"})
	assert.Equal(t, http.StatusNoContent, rec.Code)

	rec = do(t, s, http.MethodGet, "/api/meta", nil)
	meta := decode[api.Meta](t, rec)
	assert.Contains(t, meta.Favorites, "notes/n1")
	assert.Contains(t, meta.RecentFiles, "notes/n1")
}

func TestGraphResolvesWikiLinksByAliasAndTitle(t *testing.T) {
	s, app := newTestServer(t, false)
	ctx := context.Background()
	_, err := app.Knowledge.Create(ctx, "notes/phase6")
	require.NoError(t, err)
	_, err = app.Knowledge.Create(ctx, "index")
	require.NoError(t, err)
	require.NoError(t, app.Knowledge.SaveContent(ctx, "notes/phase6", "---\ntitle: Phase 6\naliases: [P6]\n---\n\ncontent"))
	require.NoError(t, app.Knowledge.SaveContent(ctx, "index", "see [[Phase 6]], [[P6]], [[notes/phase6]], [[phase6]]"))

	rec := do(t, s, http.MethodGet, "/api/graph", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	graph := decode[api.GraphData](t, rec)
	require.Len(t, graph.Links, 4)
	for _, l := range graph.Links {
		assert.Equal(t, "notes/phase6", l.Target)
	}
}

func TestNotFoundAndTraversal(t *testing.T) {
	s, _ := newTestServer(t, false)

	rec := do(t, s, http.MethodGet, "/api/nope", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = do(t, s, http.MethodGet, "/api/knowledge/..%2f..%2fetc%2fpasswd", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
