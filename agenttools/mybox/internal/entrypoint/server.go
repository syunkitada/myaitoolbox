package entrypoint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/syunkitada/myaitoolbox/mybox/internal/application"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/entrypoint/api"
	automationinfra "github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/automation"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/config"
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
	projects       *application.ProjectUseCase
	mu             sync.RWMutex
	defaultProject string
	basePath       string
	newApp         func(context.Context, string) (*App, error)
	herdrRun       herdrRunFunc
	terminals      *terminalHub
	fileExecutions *fileExecutionHub
	scheduled      *promptScheduler
}

const maxJSONBodySize = 16 << 20

func NewServer(cfg *domain.Config, defaultProject string, basePath string) *Server {
	if defaultProject == "" {
		defaultProject = cfg.DefaultProject
	}
	basePath = normalizeBasePath(basePath)
	s := &Server{
		config:         cfg,
		apps:           make(map[string]*App),
		projects:       NewProjectApp(),
		defaultProject: defaultProject,
		basePath:       basePath,
		newApp:         NewApp,
		terminals:      newTerminalHub(),
		fileExecutions: newFileExecutionHub(),
	}
	s.scheduled = newPromptScheduler(config.NewScheduledPromptStore(), s.runScheduledPrompt)
	return s
}

// StartPromptScheduler starts the server-owned worker that delivers persisted
// prompts even when no browser window is open.
func (s *Server) StartPromptScheduler(ctx context.Context) {
	if s.scheduled != nil {
		s.scheduled.Start(ctx)
	}
}

// Shutdown stops server-owned file executions before the HTTP server exits.
func (s *Server) Shutdown() {
	if s.fileExecutions != nil {
		s.fileExecutions.stopAll()
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
	s.mu.RLock()
	if project == "" {
		project = s.defaultProject
	}
	app, ok := s.apps[project]
	s.mu.RUnlock()
	if ok {
		return app, nil
	}
	return s.loadAndCacheApp(r.Context(), project)
}

// loadAndCacheApp deliberately constructs the app before taking s.mu. NewApp
// loads project configuration, which uses the config store mutex. Keeping
// that work outside the server lock avoids an inverse lock order with
// refreshDefaultProject (config store -> s.mu).
func (s *Server) loadAndCacheApp(ctx context.Context, project string) (*App, error) {
	loader := s.newApp
	if loader == nil {
		loader = NewApp
	}
	app, err := loader(ctx, project)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if cached, ok := s.apps[project]; ok {
		return cached, nil
	}
	s.apps[project] = app
	return app, nil
}

func (s *Server) refreshDefaultProject(ctx context.Context) {
	cfg, err := s.projects.Config.Load(ctx)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.config = cfg
	s.defaultProject = cfg.DefaultProject
	s.mu.Unlock()
}

func (s *Server) Handler() http.Handler {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Recover())

	apiHandler := api.HandlerWithOptions(s, api.StdHTTPServerOptions{BaseURL: s.basePath})
	rawFile := echo.WrapHandler(http.HandlerFunc(s.GetFileRaw))
	uploadFiles := echo.WrapHandler(http.HandlerFunc(s.UploadFiles))
	getTaskTemplate := echo.WrapHandler(http.HandlerFunc(s.GetTaskTemplate))
	createTaskTrigger := echo.WrapHandler(http.HandlerFunc(s.CreateTaskTrigger))
	runTaskTrigger := func(c echo.Context) error {
		s.RunTaskTrigger(c.Response(), c.Request(), c.Param("id"))
		return nil
	}
	if s.basePath == "" {
		e.GET("/api/files/raw", rawFile)
		e.POST("/api/files/upload", uploadFiles)
		e.GET("/api/task-template", getTaskTemplate)
		e.POST("/api/task-triggers", createTaskTrigger)
		e.POST("/api/task-triggers/:id/run", runTaskTrigger)
		e.POST("/api/files/execute/runs", s.StartFileExecution)
		e.GET("/api/files/execute/runs", s.ListFileExecutions)
		e.POST("/api/files/execute/runs/:id/stop", s.StopFileExecution)
		e.DELETE("/api/files/execute/runs/:id", s.DismissFileExecution)
		e.GET("/api/files/execute/stream", s.ExecuteFileStream)
		e.GET("/api/terminal", s.Terminal)
		e.DELETE("/api/terminal/destroy", s.DestroyTerminal)
		e.GET("/api/stats", echo.WrapHandler(http.HandlerFunc(s.GetStats)))
		s.registerGitRoutes(e, "")
		s.registerHerdrRoutes(e, "")
		e.Any("/api/*", echo.WrapHandler(apiHandler))
		e.Any("/api", echo.WrapHandler(apiHandler))
		e.GET("/*", s.handleIndex)
		return e
	}
	g := e.Group(s.basePath)
	g.GET("/api/files/raw", rawFile)
	g.POST("/api/files/upload", uploadFiles)
	g.GET("/api/task-template", getTaskTemplate)
	g.POST("/api/task-triggers", createTaskTrigger)
	g.POST("/api/task-triggers/:id/run", runTaskTrigger)
	g.POST("/api/files/execute/runs", s.StartFileExecution)
	g.GET("/api/files/execute/runs", s.ListFileExecutions)
	g.POST("/api/files/execute/runs/:id/stop", s.StopFileExecution)
	g.DELETE("/api/files/execute/runs/:id", s.DismissFileExecution)
	g.GET("/api/files/execute/stream", s.ExecuteFileStream)
	g.GET("/api/terminal", s.Terminal)
	g.DELETE("/api/terminal/destroy", s.DestroyTerminal)
	g.GET("/api/stats", echo.WrapHandler(http.HandlerFunc(s.GetStats)))
	s.registerGitRoutes(e, s.basePath)
	s.registerHerdrRoutes(e, s.basePath)
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
	// Vite emits relative asset paths (./assets/...), so a request from a
	// project route can arrive as /projects/{project}/.../assets/... . Only
	// resolve the SPA entrypoint and compiled assets here. Stripping arbitrary
	// path prefixes would make a project route ending in README.md resolve to
	// the documentation file embedded at webDist/README.md instead of serving
	// index.html.
	assetPath := path
	if path != "index.html" && !strings.HasPrefix(path, "assets/") {
		if i := strings.Index(path, "/assets/"); i >= 0 {
			assetPath = path[i+1:]
		} else {
			assetPath = ""
		}
	}
	var f fs.File
	err := fs.ErrNotExist
	if assetPath != "" {
		f, err = webDist.Open(assetPath)
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

func (s *Server) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.projects.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]api.Project, 0, len(projects))
	for _, p := range projects {
		out = append(out, api.Project{Name: p.Name, Path: p.Path})
	}
	writeJSONResponse(w, http.StatusOK, out)
}

func (s *Server) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req api.CreateProjectRequest
	if !decodeBody(w, r, &req) {
		return
	}
	project, err := s.projects.Add(r.Context(), req.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	s.mu.Lock()
	delete(s.apps, project.Name)
	s.mu.Unlock()
	s.refreshDefaultProject(r.Context())
	writeJSONResponse(w, http.StatusCreated, api.Project{Name: project.Name, Path: project.Path})
}

func (s *Server) DeleteProject(w http.ResponseWriter, r *http.Request, name string) {
	if err := s.projects.Remove(r.Context(), name); err != nil {
		writeError(w, err)
		return
	}
	s.mu.Lock()
	delete(s.apps, name)
	s.mu.Unlock()
	s.refreshDefaultProject(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ReorderProjects(w http.ResponseWriter, r *http.Request) {
	var req api.ReorderProjectsRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.projects.Reorder(r.Context(), req.Names); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) GetProjectPaths(w http.ResponseWriter, r *http.Request, params api.GetProjectPathsParams) {
	var prefix string
	if params.Prefix != nil {
		prefix = *params.Prefix
	}
	paths, err := s.projects.PathCandidates(r.Context(), prefix)
	if err != nil {
		writeError(w, err)
		return
	}
	if paths == nil {
		paths = []string{}
	}
	writeJSONResponse(w, http.StatusOK, paths)
}

func (s *Server) GetProjectGitStatus(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.projects.Config.Load(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	result := make(map[string]api.ProjectGitStatus, len(cfg.Projects))
	for _, p := range cfg.Projects {
		status := gitStatus(p.Path)
		result[p.Name] = status
	}
	writeJSONResponse(w, http.StatusOK, result)
}

func isGitDir(dir string) bool {
	return isInsideWorkTree(dir)
}

func gitStatus(dir string) api.ProjectGitStatus {
	if !isGitDir(dir) {
		return api.ProjectGitStatus{}
	}
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return api.ProjectGitStatus{}
	}
	var staged, modified, untracked int
	for _, line := range bytes.Split(out, []byte("\n")) {
		if len(line) < 2 {
			continue
		}
		x, y := line[0], line[1]
		if x == '?' && y == '?' {
			untracked++
		} else {
			if x != ' ' && x != '?' {
				staged++
			}
			if y != ' ' && y != '?' {
				modified++
			}
		}
	}
	dirty := staged+modified+untracked > 0
	return api.ProjectGitStatus{Dirty: dirty, Modified: modified, Staged: staged, Untracked: untracked}
}

func (s *Server) GetMeta(w http.ResponseWriter, r *http.Request) {
	project := r.Header.Get("X-Project")
	if project == "" {
		cfg, err := s.projects.Config.Load(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		projects := make([]string, 0, len(cfg.Projects))
		for _, p := range cfg.Projects {
			projects = append(projects, p.Name)
		}
		writeJSONResponse(w, http.StatusOK, api.Meta{
			Project:        "",
			Projects:       projects,
			DefaultProject: cfg.DefaultProject,
			Tags:           []string{},
			Favorites:      []api.Favorite{},
			RecentFiles:    []string{},
		})
		return
	}
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
	cfgProjects, err := s.projects.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	projects := make([]string, 0, len(cfgProjects))
	for _, p := range cfgProjects {
		projects = append(projects, p.Name)
	}
	defaultProject, err := s.projects.Default(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	favorites := s.resolveFavoriteProjects(r.Context(), state.Favorites, app.Project.Name, cfgProjects)
	resolutions := make(map[string]string)
	for i, favorite := range state.Favorites {
		if favorite.Project == "" && favorites[i].Project != "" {
			resolutions[favorite.Path] = favorites[i].Project
		}
	}
	if err := app.State.ResolveFavoriteProjects(r.Context(), resolutions); err != nil {
		writeError(w, err)
		return
	}
	tags, err := s.collectTags(r.Context(), app)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.Meta{
		Project:        app.Project.Name,
		Projects:       projects,
		DefaultProject: defaultProject,
		Tags:           tags,
		Favorites:      toAPIFavorites(favorites),
		RecentFiles:    state.RecentFiles,
	})
}

func (s *Server) resolveFavoriteProjects(ctx context.Context, favorites []domain.Favorite, currentProject string, projects []domain.Project) []domain.Favorite {
	resolved := append([]domain.Favorite(nil), favorites...)
	for i := range resolved {
		if resolved[i].Project != "" || resolved[i].Path == "" {
			continue
		}
		candidates := projects
		if currentProject != "" {
			candidates = make([]domain.Project, 0, len(projects))
			for _, project := range projects {
				if project.Name == currentProject {
					candidates = append(candidates, project)
					break
				}
			}
			for _, project := range projects {
				if project.Name != currentProject {
					candidates = append(candidates, project)
				}
			}
		}
		for _, project := range candidates {
			app, err := s.getAppByProject(ctx, project.Name)
			if err != nil || !favoritePathExists(ctx, app, resolved[i].Path) {
				continue
			}
			resolved[i].Project = project.Name
			break
		}
	}
	return resolved
}

func favoritePathExists(ctx context.Context, app *App, favoritePath string) bool {
	parent := pathpkg.Dir(favoritePath)
	if parent == "." {
		parent = ""
	}
	entries, err := app.Files.Children(ctx, parent, true)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Path == favoritePath {
			return true
		}
	}
	return false
}

func toAPIFavorites(favorites []domain.Favorite) []api.Favorite {
	out := make([]api.Favorite, 0, len(favorites))
	for _, favorite := range favorites {
		out = append(out, api.Favorite{Project: favorite.Project, Path: favorite.Path})
	}
	return out
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
	markdownTags, err := app.Files.MarkdownTags(ctx)
	if err != nil {
		return nil, err
	}
	for _, tag := range markdownTags {
		set[tag] = struct{}{}
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
	var req api.UpdateFavoriteRequest
	if !decodeBody(w, r, &req) {
		return
	}
	project := app.Project.Name
	if req.Project != nil && *req.Project != "" {
		project = *req.Project
	}
	if err := app.State.ToggleFavorite(r.Context(), req.Path, project, req.Enabled); err != nil {
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

func (s *Server) DeleteRecent(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.RecordRecentRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.State.RemoveRecent(r.Context(), req.Path); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ListTasks(w http.ResponseWriter, r *http.Request, params api.ListTasksParams) {
	project := r.Header.Get("X-Project")
	filter := application.TaskFilter{}
	if params.All != nil {
		filter.All = *params.All
	}
	if params.Status != nil {
		filter.Status = string(*params.Status)
	}
	if params.Tag != nil {
		filter.Tag = *params.Tag
	}

	// X-Project ヘッダーがない = プロジェクト未選択 → 全プロジェクトを横断
	if project == "" {
		tasks, err := s.listAllProjectsTasks(r.Context(), filter)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSONResponse(w, http.StatusOK, toAPITasks(tasks))
		return
	}

	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	tasks, err := app.Tasks.List(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, toAPITasks(tasks))
}

func (s *Server) listAllProjectsTasks(ctx context.Context, filter application.TaskFilter) ([]domain.Task, error) {
	projects, err := s.projects.List(ctx)
	if err != nil {
		return nil, err
	}
	var all []domain.Task
	for _, p := range projects {
		app, appErr := s.getAppByProject(ctx, p.Name)
		if appErr != nil {
			continue
		}
		tasks, taskErr := app.Tasks.List(ctx, filter)
		if taskErr != nil {
			continue
		}
		all = append(all, tasks...)
	}
	return all, nil
}

func (s *Server) getAppByProject(ctx context.Context, project string) (*App, error) {
	s.mu.RLock()
	app, ok := s.apps[project]
	s.mu.RUnlock()
	if ok {
		return app, nil
	}
	return s.loadAndCacheApp(ctx, project)
}

func (s *Server) CreateTask(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req createTaskRequest
	if !decodeBody(w, r, &req) {
		return
	}
	input := application.TaskInput{Name: req.Name}
	input.Content = req.Content
	if req.Description != nil {
		input.Description = *req.Description
	}
	if req.AgentKind != nil {
		input.AgentKind = *req.AgentKind
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
	task, err := app.Tasks.Create(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusCreated, toAPITask(*task))
}

type createTaskRequest struct {
	api.CreateTaskRequest
	Content *string `json:"content,omitempty"`
}

type taskTemplateResponse struct {
	Content string `json:"content"`
}

func (s *Server) GetTaskTemplate(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	content, err := app.Tasks.RenderTaskTemplate(r.URL.Query().Get("name"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, taskTemplateResponse{Content: content})
}

type taskTriggerCreateRequest struct {
	ID      string                      `json:"id"`
	Task    taskTriggerTaskRequest      `json:"task"`
	Trigger taskTriggerConditionRequest `json:"trigger"`
}

type taskTriggerTaskRequest struct {
	Name      string  `json:"name"`
	AgentKind string  `json:"agent_kind"`
	Prompt    string  `json:"prompt,omitempty"`
	Content   *string `json:"content,omitempty"`
}

type taskTriggerConditionRequest struct {
	Type     string `json:"type"`
	Cron     string `json:"cron,omitempty"`
	Timezone string `json:"timezone,omitempty"`
	Path     string `json:"path,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
}

type taskTriggerResponse struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Directory string `json:"directory"`
	TaskPath  string `json:"task_path"`
}

type taskTriggerRunResponse struct {
	ID           string    `json:"id"`
	Project      string    `json:"project"`
	TriggerID    string    `json:"trigger_id"`
	EventID      string    `json:"event_id"`
	Status       string    `json:"status"`
	TaskID       string    `json:"task_id,omitempty"`
	AgentName    string    `json:"agent_name,omitempty"`
	SourcePath   string    `json:"source_path,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

func (s *Server) CreateTaskTrigger(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if app.TaskTriggers == nil {
		writeError(w, fmt.Errorf("task trigger use case is unavailable"))
		return
	}
	var req taskTriggerCreateRequest
	if !decodeBody(w, r, &req) {
		return
	}
	triggerID := strings.TrimSpace(req.ID)
	err = app.TaskTriggers.Create(r.Context(), application.TaskTriggerInput{
		ID:        triggerID,
		Name:      req.Task.Name,
		Content:   req.Task.Content,
		Type:      domain.TriggerType(req.Trigger.Type),
		Cron:      req.Trigger.Cron,
		Timezone:  req.Trigger.Timezone,
		WatchPath: req.Trigger.Path,
		Pattern:   req.Trigger.Pattern,
		AgentKind: req.Task.AgentKind,
		Prompt:    req.Task.Prompt,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	triggerDir := filepath.ToSlash(filepath.Join("_task_triggers", triggerID))
	writeJSONResponse(w, http.StatusCreated, taskTriggerResponse{
		ID:        triggerID,
		Type:      req.Trigger.Type,
		Directory: triggerDir,
		TaskPath:  filepath.ToSlash(filepath.Join(triggerDir, "task.md")),
	})
}

func (s *Server) RunTaskTrigger(w http.ResponseWriter, r *http.Request, id string) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if app.TaskTriggers == nil || app.Automation == nil {
		writeError(w, fmt.Errorf("%w: automation is unavailable", domain.ErrInvalidArgument))
		return
	}
	def, err := app.TaskTriggers.Find(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	if !def.Enabled {
		writeError(w, fmt.Errorf("%w: trigger %s is disabled", domain.ErrInvalidArgument, def.ID))
		return
	}
	release, err := automationinfra.AcquireProjectLock(config.AutomationStateDir(), app.Project.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	defer release()
	run, err := app.Automation.RunManual(r.Context(), *def)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, taskTriggerRunResponse{
		ID:           run.ID,
		Project:      run.Project,
		TriggerID:    run.TriggerID,
		EventID:      run.EventID,
		Status:       string(run.Status),
		TaskID:       run.TaskID,
		AgentName:    run.AgentName,
		SourcePath:   run.SourcePath,
		StartedAt:    run.StartedAt,
		FinishedAt:   run.FinishedAt,
		ErrorMessage: run.ErrorMessage,
	})
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
	var req api.UpdateTaskRequest
	if !decodeBody(w, r, &req) {
		return
	}
	patch := application.TaskPatch{}
	if req.Name != nil {
		patch.Name = req.Name
	}
	if req.Description != nil {
		patch.Description = req.Description
	}
	if req.AgentKind != nil {
		patch.AgentKind = req.AgentKind
	}
	if req.Status != nil {
		status := string(*req.Status)
		patch.Status = &status
	}
	if req.Priority != nil {
		priority := string(*req.Priority)
		patch.Priority = &priority
	}
	if req.Assignee != nil {
		patch.Assignee = req.Assignee
	}
	if req.Due != nil {
		patch.Due = req.Due
	}
	if req.Tags != nil {
		patch.Tags = req.Tags
	}
	task, err := app.Tasks.Patch(r.Context(), id, patch)
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
	if err := app.Tasks.Archive(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) DeleteTask(w http.ResponseWriter, r *http.Request, id string) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := app.Tasks.Delete(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ListFiles(w http.ResponseWriter, r *http.Request, params api.ListFilesParams) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	showHidden := true
	if params.ShowHidden != nil {
		showHidden = *params.ShowHidden
	}
	var entries []domain.FileEntry
	if params.Path != nil {
		entries, err = app.Files.Children(r.Context(), *params.Path, showHidden)
	} else {
		entries, err = app.Files.Tree(r.Context(), showHidden)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]api.FileEntry, 0, len(entries))
	for _, e := range entries {
		entry := api.FileEntry{Path: e.Path, Name: e.Name, Kind: api.FileEntryKind(e.Kind)}
		if e.Status != "" {
			entry.Status = &e.Status
		}
		if e.Executable {
			exec := true
			entry.Executable = &exec
		}
		out = append(out, entry)
	}
	writeJSONResponse(w, http.StatusOK, out)
}

func (s *Server) SearchFiles(w http.ResponseWriter, r *http.Request, params api.SearchFilesParams) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	showHidden := true
	if params.ShowHidden != nil {
		showHidden = *params.ShowHidden
	}
	results, total, err := app.Files.Search(r.Context(), params.Q, showHidden)
	if err != nil {
		writeError(w, err)
		return
	}
	out := api.FileSearchResponse{
		Query:     params.Q,
		Results:   make([]api.FileSearchResult, 0, len(results)),
		Total:     total,
		Truncated: total > len(results),
	}
	for _, result := range results {
		out.Results = append(out.Results, api.FileSearchResult{
			Path:       result.Path,
			Line:       result.Line,
			Snippet:    result.Snippet,
			MatchCount: result.MatchCount,
		})
	}
	writeJSONResponse(w, http.StatusOK, out)
}

func (s *Server) ExecuteFile(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.FilePathRequest
	if !decodeBody(w, r, &req) {
		return
	}
	res, err := app.Files.Execute(r.Context(), req.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	out := api.FileExecuteResult{Path: req.Path, ExitCode: res.ExitCode, Output: res.Output}
	if res.TimedOut {
		timedOut := true
		out.TimedOut = &timedOut
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

// GetFileRaw streams a project file as raw bytes (used for image embeds in
// markdown, where the browser cannot send the X-Project header).
func (s *Server) GetFileRaw(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	path := q.Get("path")
	project := q.Get("project")
	var (
		app *App
		err error
	)
	if project != "" {
		app, err = s.getAppByProject(r.Context(), project)
	} else {
		app, err = s.getApp(r)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := app.Files.Raw(r.Context(), path)
	if err != nil {
		writeError(w, err)
		return
	}
	ctype := mime.TypeByExtension(filepath.Ext(path))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ctype)
	_, _ = w.Write(data)
}

func (s *Server) CreateFile(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.FilePathRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.Files.Create(r.Context(), req.Path); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) CreateDir(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.FilePathRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.Files.CreateDir(r.Context(), req.Path); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) SaveFileContent(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
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

const (
	// Keep a finite request limit to avoid unbounded resource usage while
	// allowing the large archives commonly stored in a workspace.
	maxUploadSize   = 1 << 30
	maxUploadMemory = 32 << 20
)

// UploadFiles stores one or more local files in a project directory. The
// multipart endpoint is intentionally kept outside the generated JSON API
// because file uploads need multipart parsing rather than JSON decoding.
func (s *Server) UploadFiles(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadMemory); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, &httpError{
				status: http.StatusRequestEntityTooLarge,
				err:    fmt.Errorf("upload exceeds the maximum size of %d GiB", maxUploadSize>>30),
			})
			return
		}
		writeError(w, fmt.Errorf("%w: invalid multipart form", domain.ErrInvalidArgument))
		return
	}
	if r.MultipartForm != nil {
		defer func() {
			// MultipartForm cleanup happens after the response is determined;
			// a cleanup failure cannot be reported as a new HTTP error here.
			_ = r.MultipartForm.RemoveAll()
		}()
	}

	directory := r.FormValue("directory")
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeError(w, fmt.Errorf("%w: no files supplied", domain.ErrInvalidArgument))
		return
	}

	for _, header := range files {
		name := filepath.Base(strings.ReplaceAll(header.Filename, string(rune(92)), "/"))
		if name == "" || name == "." || name == ".." || strings.ContainsRune(name, 0) {
			writeError(w, fmt.Errorf("%w: invalid file name", domain.ErrInvalidArgument))
			return
		}
		path := name
		if directory != "" {
			path = directory + "/" + name
		}

		file, err := header.Open()
		if err != nil {
			writeError(w, err)
			return
		}
		saveErr := app.Files.SaveReader(r.Context(), path, file)
		closeErr := file.Close()
		if saveErr != nil {
			writeError(w, saveErr)
			return
		}
		if closeErr != nil {
			writeError(w, closeErr)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) MoveFile(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.MoveFileRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.Files.Move(r.Context(), req.OldPath, req.NewPath); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) CopyFile(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.MoveFileRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.Files.Copy(r.Context(), req.OldPath, req.NewPath); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) DeleteFile(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.FilePathRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.Files.Delete(r.Context(), req.Path); err != nil {
		writeError(w, err)
		return
	}
	if err := app.State.RemoveFavorites(r.Context(), app.Project.Name, req.Path); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) GetFileGitStatus(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	result := fileGitStatus(app.Project.Path)
	writeJSONResponse(w, http.StatusOK, result)
}

func fileGitStatus(dir string) map[string]string {
	if !isGitDir(dir) {
		return map[string]string{}
	}
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return map[string]string{}
	}
	result := make(map[string]string)
	for _, line := range bytes.Split(out, []byte("\n")) {
		if len(line) < 3 {
			continue
		}
		x, y := line[0], line[1]
		path := string(bytes.TrimLeft(line[2:], " "))
		if x == '?' && y == '?' {
			result[path] = "untracked"
		} else if x == 'D' || y == 'D' {
			result[path] = "deleted"
		} else if x != ' ' && x != '?' {
			result[path] = "staged"
		} else if y != ' ' && y != '?' {
			result[path] = "modified"
		}
	}
	return result
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
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodySize)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, &httpError{
				status: http.StatusRequestEntityTooLarge,
				err:    fmt.Errorf("request body exceeds the maximum size of %d MiB", maxJSONBodySize>>20),
			})
			return false
		}
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
	case errors.Is(err, domain.ErrResourceTooLarge):
		status = http.StatusRequestEntityTooLarge
	case errors.Is(err, domain.ErrAlreadyExists):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrAutomationBusy):
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
		Id:            t.ID,
		Title:         t.Title,
		Description:   strPtr(t.Description),
		AgentKind:     strPtr(t.AgentKind),
		Status:        api.TaskStatus(t.Status),
		Priority:      api.TaskPriority(t.Priority),
		Assignee:      strPtr(t.Assignee),
		Due:           strPtr(t.Due),
		PendingUntil:  strPtr(t.PendingUntil),
		PendingReason: strPtr(t.PendingReason),
		Tags:          &t.Tags,
		Project:       strPtr(t.Project),
		Created:       &t.Created,
		Body:          strPtr(t.Body),
		Archived:      &t.Archived,
	}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
