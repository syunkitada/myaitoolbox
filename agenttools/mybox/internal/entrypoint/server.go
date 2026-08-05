package entrypoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/syunkitada/myaitoolbox/mybox/internal/application"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/entrypoint/api"
	"github.com/syunkitada/myaitoolbox/mybox/internal/webui"
)

var webDist = func() fs.FS {
	sub, err := fs.Sub(webui.FS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}()

type Server struct {
	config         *domain.Config
	apps           map[string]*App
	mu             sync.RWMutex
	readOnly       bool
	defaultProject string
	basePath       string
}

func NewServer(cfg *domain.Config, defaultProject string, readOnly bool, basePath string) *Server {
	if defaultProject == "" {
		defaultProject = cfg.DefaultProject
	}
	basePath = normalizeBasePath(basePath)
	return &Server{
		config:         cfg,
		apps:           make(map[string]*App),
		readOnly:       readOnly,
		defaultProject: defaultProject,
		basePath:       basePath,
	}
}

func normalizeBasePath(basePath string) string {
	basePath = strings.TrimSpace(basePath)
	basePath = strings.TrimSuffix(basePath, "/")
	if basePath != "" && !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	return basePath
}

func (s *Server) getApp(r *http.Request) (*App, error) {
	project := r.Header.Get("X-Project")
	if project == "" {
		project = s.defaultProject
	}
	s.mu.RLock()
	app, ok := s.apps[project]
	s.mu.RUnlock()
	if ok {
		return app, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if app, ok := s.apps[project]; ok {
		return app, nil
	}
	app, err := NewApp(r.Context(), project)
	if err != nil {
		return nil, err
	}
	s.apps[project] = app
	return app, nil
}

func (s *Server) Handler() http.Handler {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Recover())

	apiHandler := api.HandlerWithOptions(s, api.StdHTTPServerOptions{BaseURL: s.basePath})
	if s.basePath == "" {
		e.Any("/api/*", echo.WrapHandler(apiHandler))
		e.Any("/api", echo.WrapHandler(apiHandler))
		e.GET("/*", s.handleIndex)
		return e
	}
	g := e.Group(s.basePath)
	g.Any("/api/*", echo.WrapHandler(apiHandler))
	g.Any("/api", echo.WrapHandler(apiHandler))
	g.GET("", s.handleIndex)
	g.GET("/*", s.handleIndex)
	e.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusFound, s.basePath+"/")
	})
	return e
}

func (s *Server) handleIndex(c echo.Context) error {
	path := c.Request().URL.Path
	if s.basePath != "" {
		path = strings.TrimPrefix(path, s.basePath)
	}
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return c.Redirect(http.StatusFound, s.basePath+"/"+s.defaultProject+"/")
	}
	f, err := webDist.Open(path)
	// The SPA is served under /{project}/ (or /{basePath}/{project}/) and Vite
	// emits relative asset paths (./assets/...), so those requests arrive as
	// /{project}/assets/... — strip the leading project segment and retry.
	if err != nil {
		if i := strings.Index(path, "/"); i > 0 {
			f, err = webDist.Open(path[i+1:])
		}
	}
	if err == nil {
		info, statErr := f.Stat()
		if statErr == nil && !info.IsDir() {
			data, readErr := io.ReadAll(f)
			_ = f.Close()
			if readErr != nil {
				return readErr
			}
			ctype := mime.TypeByExtension(filepath.Ext(path))
			if ctype == "" {
				ctype = "application/octet-stream"
			}
			c.Response().Header().Set("Content-Type", ctype)
			_, _ = c.Response().Write(data)
			return nil
		}
		_ = f.Close()
	}
	return s.serveIndex(c)
}

func (s *Server) serveIndex(c echo.Context) error {
	data, readErr := fs.ReadFile(webDist, "index.html")
	if readErr != nil {
		return readErr
	}
	html := string(data)
	if s.basePath != "" {
		inject := fmt.Sprintf(
			"<base href=\"%s/\">\n<script>window.__MYBOX_BASE__=%q;</script>",
			s.basePath, s.basePath,
		)
		html = strings.Replace(html, "<head>", "<head>\n"+inject, 1)
	}
	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = c.Response().Write([]byte(html))
	return nil
}

func (s *Server) GetMeta(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	state, err := app.State.Get(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	projects := make([]string, 0, len(app.Config.Projects))
	for _, p := range app.Config.Projects {
		projects = append(projects, p.Name)
	}
	tags, err := s.collectTags(r.Context(), app)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.Meta{
		Project:        app.Project.Name,
		Projects:       projects,
		DefaultProject: app.Config.DefaultProject,
		Tags:           tags,
		Favorites:      state.Favorites,
		RecentFiles:    state.RecentFiles,
	})
}

func (s *Server) collectTags(ctx context.Context, app *App) ([]string, error) {
	set := map[string]struct{}{}
	tasks, err := app.Tasks.List(ctx, application.TaskFilter{})
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		for _, tag := range t.Tags {
			set[tag] = struct{}{}
		}
	}
	knowledge, err := app.Knowledge.List(ctx, application.KnowledgeFilter{})
	if err != nil {
		return nil, err
	}
	for _, k := range knowledge {
		for _, tag := range k.Tags {
			set[tag] = struct{}{}
		}
	}
	tags := make([]string, 0, len(set))
	for tag := range set {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags, nil
}

func (s *Server) UpdateFavorite(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if !s.ensureWritable(w) {
		return
	}
	var req api.UpdateFavoriteRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.State.ToggleFavorite(r.Context(), req.Path, req.Enabled); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) RecordRecent(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if !s.ensureWritable(w) {
		return
	}
	var req api.RecordRecentRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.State.RecordRecent(r.Context(), req.Path); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) Search(w http.ResponseWriter, r *http.Request, params api.SearchParams) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	option := domain.SearchOption{}
	if params.Type != nil {
		switch *params.Type {
		case api.SearchParamsType(domain.SearchTypeTask), api.SearchParamsType(domain.SearchTypeKnowledge):
			option.Type = domain.SearchType(*params.Type)
		default:
			writeError(w, fmt.Errorf("%w: invalid type", domain.ErrInvalidArgument))
			return
		}
	}
	results, err := app.Search.Search(r.Context(), params.Q, option)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]api.SearchResult, 0, len(results))
	for _, res := range results {
		out = append(out, api.SearchResult{
			Id:      strPtr(res.ID),
			Path:    res.Path,
			Title:   res.Title,
			Snippet: strPtr(res.Snippet),
			Type:    api.SearchResultType(res.Type),
		})
	}
	writeJSONResponse(w, http.StatusOK, out)
}

func (s *Server) ListTasks(w http.ResponseWriter, r *http.Request, params api.ListTasksParams) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	filter := application.TaskFilter{}
	if params.All != nil {
		filter.All = *params.All
	}
	if params.Status != nil {
		filter.Status = *params.Status
	}
	if params.Tag != nil {
		filter.Tag = *params.Tag
	}
	tasks, err := app.Tasks.List(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, toAPITasks(tasks))
}

func (s *Server) CreateTask(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if !s.ensureWritable(w) {
		return
	}
	var req api.CreateTaskRequest
	if !decodeBody(w, r, &req) {
		return
	}
	input := application.TaskInput{Name: req.Name}
	if req.Status != nil {
		input.Status = string(*req.Status)
	}
	if req.Priority != nil {
		input.Priority = string(*req.Priority)
	}
	if req.Assignee != nil {
		input.Assignee = *req.Assignee
	}
	if req.Due != nil {
		input.Due = *req.Due
	}
	if req.Tags != nil {
		input.Tags = *req.Tags
	}
	task, err := app.Tasks.Create(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusCreated, toAPITask(*task))
}

func (s *Server) GetTask(w http.ResponseWriter, r *http.Request, id string) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	task, err := app.Tasks.Show(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, toAPITask(*task))
}

func (s *Server) UpdateTask(w http.ResponseWriter, r *http.Request, id string) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if !s.ensureWritable(w) {
		return
	}
	var req api.UpdateTaskRequest
	if !decodeBody(w, r, &req) {
		return
	}
	input := application.TaskInput{}
	if req.Name != nil {
		input.Name = *req.Name
	}
	if req.Status != nil {
		input.Status = string(*req.Status)
	}
	if req.Priority != nil {
		input.Priority = string(*req.Priority)
	}
	if req.Assignee != nil {
		input.Assignee = *req.Assignee
	}
	if req.Due != nil {
		input.Due = *req.Due
	}
	if req.Tags != nil {
		input.Tags = *req.Tags
	}
	task, err := app.Tasks.Update(r.Context(), id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, toAPITask(*task))
}

func (s *Server) ArchiveTask(w http.ResponseWriter, r *http.Request, id string) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if !s.ensureWritable(w) {
		return
	}
	if err := app.Tasks.Archive(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ListKnowledge(w http.ResponseWriter, r *http.Request, params api.ListKnowledgeParams) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	filter := application.KnowledgeFilter{}
	if params.Tag != nil {
		filter.Tag = *params.Tag
	}
	list, err := app.Knowledge.List(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	if params.Path == nil || *params.Path == "" {
		writeJSONResponse(w, http.StatusOK, toAPIKnowledge(list))
		return
	}
	var out []api.Knowledge
	prefix := *params.Path
	for _, k := range list {
		if k.Path == prefix || strings.HasPrefix(k.Path, prefix+"/") {
			out = append(out, toAPIKnowledgeItem(k))
		}
	}
	writeJSONResponse(w, http.StatusOK, out)
}

func (s *Server) CreateKnowledge(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if !s.ensureWritable(w) {
		return
	}
	var req api.CreateKnowledgeRequest
	if !decodeBody(w, r, &req) {
		return
	}
	k, err := app.Knowledge.Create(r.Context(), req.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusCreated, toAPIKnowledgeItem(*k))
}

func (s *Server) GetKnowledgeContent(w http.ResponseWriter, r *http.Request, params api.GetKnowledgeContentParams) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	content, err := app.Knowledge.Content(r.Context(), params.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.KnowledgeContent{Path: params.Path, Content: content})
}

func (s *Server) SaveKnowledgeContent(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if !s.ensureWritable(w) {
		return
	}
	var req api.KnowledgeContent
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.Knowledge.SaveContent(r.Context(), req.Path, req.Content); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) MoveKnowledge(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if !s.ensureWritable(w) {
		return
	}
	var req api.MoveKnowledgeRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.Knowledge.Move(r.Context(), req.OldPath, req.NewPath); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) RenameKnowledge(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if !s.ensureWritable(w) {
		return
	}
	var req api.RenameKnowledgeRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.Knowledge.Rename(r.Context(), req.OldPath, req.NewName); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ListFiles(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	entries, err := app.Files.Tree(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]api.FileEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, api.FileEntry{Path: e.Path, Name: e.Name, Kind: api.FileEntryKind(e.Kind)})
	}
	writeJSONResponse(w, http.StatusOK, out)
}

func (s *Server) GetFileContent(w http.ResponseWriter, r *http.Request, params api.GetFileContentParams) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	content, err := app.Files.Content(r.Context(), params.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.FileContent{Path: params.Path, Content: content})
}

func (s *Server) SaveFileContent(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if !s.ensureWritable(w) {
		return
	}
	var req api.FileContent
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.Files.Save(r.Context(), req.Path, req.Content); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) GetGraph(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	list, err := app.Knowledge.List(r.Context(), application.KnowledgeFilter{})
	if err != nil {
		writeError(w, err)
		return
	}
	nodes := make([]api.GraphNode, 0, len(list))
	index := make(map[string]string, len(list))
	for _, k := range list {
		index[strings.ToLower(k.Path)] = k.Path
		nodes = append(nodes, api.GraphNode{
			Id:    k.Path,
			Label: k.Title,
			Type:  strPtr("knowledge"),
		})
	}
	for _, k := range list {
		if t := strings.ToLower(k.Title); t != "" {
			if _, ok := index[t]; !ok {
				index[t] = k.Path
			}
		}
		for _, a := range k.Aliases {
			if key := strings.ToLower(a); key != "" {
				if _, ok := index[key]; !ok {
					index[key] = k.Path
				}
			}
		}
		if b := strings.ToLower(filepath.Base(k.Path)); b != "" {
			if _, ok := index[b]; !ok {
				index[b] = k.Path
			}
		}
	}
	var links []api.GraphLink
	for _, k := range list {
		for _, link := range k.WikiLinks {
			target := strings.ToLower(strings.TrimSuffix(link, ".md"))
			if target == strings.ToLower(k.Path) {
				continue
			}
			if resolved, ok := index[target]; ok {
				links = append(links, api.GraphLink{Source: k.Path, Target: resolved})
			}
		}
	}
	writeJSONResponse(w, http.StatusOK, api.GraphData{Nodes: nodes, Links: links})
}

func (s *Server) ensureWritable(w http.ResponseWriter) bool {
	if s.readOnly {
		writeError(w, &httpError{status: http.StatusForbidden, err: errors.New("server is read-only")})
		return false
	}
	return true
}

type httpError struct {
	status int
	err    error
}

func (e *httpError) Error() string {
	return e.err.Error()
}

func (e *httpError) Unwrap() error {
	return e.err
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		writeError(w, fmt.Errorf("%w: invalid request body", domain.ErrInvalidArgument))
		return false
	}
	return true
}

func writeJSONResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	var he *httpError
	switch {
	case errors.As(err, &he):
		status = he.status
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrInvalidArgument), errors.Is(err, domain.ErrInvalidPath):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrAlreadyExists):
		status = http.StatusConflict
	}
	writeJSONResponse(w, status, map[string]string{"error": err.Error()})
}

func toAPITasks(tasks []domain.Task) []api.Task {
	out := make([]api.Task, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, toAPITask(t))
	}
	return out
}

func toAPITask(t domain.Task) api.Task {
	return api.Task{
		Id:       t.ID,
		Title:    t.Title,
		Status:   api.TaskStatus(t.Status),
		Priority: api.TaskPriority(t.Priority),
		Assignee: strPtr(t.Assignee),
		Due:      strPtr(t.Due),
		Tags:     &t.Tags,
		Project:  strPtr(t.Project),
		Created:  &t.Created,
		Body:     strPtr(t.Body),
		Archived: &t.Archived,
	}
}

func toAPIKnowledge(list []domain.Knowledge) []api.Knowledge {
	out := make([]api.Knowledge, 0, len(list))
	for _, k := range list {
		out = append(out, toAPIKnowledgeItem(k))
	}
	return out
}

func toAPIKnowledgeItem(k domain.Knowledge) api.Knowledge {
	return api.Knowledge{
		Path:      k.Path,
		Title:     k.Title,
		Tags:      &k.Tags,
		Aliases:   &k.Aliases,
		Type:      strPtr(k.Type),
		Created:   &k.Created,
		Lastmod:   &k.LastMod,
		WikiLinks: &k.WikiLinks,
		Body:      strPtr(k.Body),
	}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
