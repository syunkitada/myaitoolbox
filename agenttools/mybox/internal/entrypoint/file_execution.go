package entrypoint

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

const (
	fileExecutionOutputLimit     = 1 << 20
	fileExecutionHistoryLimit    = 20
	fileExecutionConcurrentLimit = 16
)

const fileExecutionOutputTruncationMarker = "\n[output truncated; showing latest output]\n"

func appendFileExecutionOutput(current string, chunk []byte) string {
	if len(chunk) == 0 {
		return current
	}
	if len(current)+len(chunk) <= fileExecutionOutputLimit {
		return current + string(chunk)
	}
	retainedLength := fileExecutionOutputLimit - len(fileExecutionOutputTruncationMarker)
	if len(chunk) >= retainedLength {
		return fileExecutionOutputTruncationMarker + string(chunk[len(chunk)-retainedLength:])
	}
	return fileExecutionOutputTruncationMarker + current[len(current)-(retainedLength-len(chunk)):] + string(chunk)
}

type fileExecutionStatus string

const (
	fileExecutionRunning   fileExecutionStatus = "running"
	fileExecutionCompleted fileExecutionStatus = "completed"
	fileExecutionFailed    fileExecutionStatus = "failed"
	fileExecutionStopped   fileExecutionStatus = "stopped"
)

type fileExecutionSnapshot struct {
	ID        string              `json:"id"`
	Path      string              `json:"path"`
	Status    fileExecutionStatus `json:"status"`
	Output    string              `json:"output"`
	ExitCode  *int                `json:"exit_code,omitempty"`
	TimedOut  bool                `json:"timed_out,omitempty"`
	Error     string              `json:"error,omitempty"`
	StartedAt time.Time           `json:"started_at"`
}

type fileExecutionStreamMessage struct {
	Type      string              `json:"type"`
	ID        string              `json:"id,omitempty"`
	Path      string              `json:"path,omitempty"`
	Status    fileExecutionStatus `json:"status,omitempty"`
	Data      string              `json:"data,omitempty"`
	Output    string              `json:"output,omitempty"`
	ExitCode  *int                `json:"exit_code,omitempty"`
	TimedOut  bool                `json:"timed_out,omitempty"`
	Error     string              `json:"error,omitempty"`
	StartedAt time.Time           `json:"started_at,omitempty"`
}

type fileExecutionClient struct {
	send      chan fileExecutionStreamMessage
	done      chan struct{}
	connClose func()
	once      sync.Once
}

func (c *fileExecutionClient) halt() {
	c.once.Do(func() {
		close(c.done)
		if c.connClose != nil {
			c.connClose()
		}
	})
}

type fileExecutionJob struct {
	hub     *fileExecutionHub
	project string
	id      string
	path    string
	started time.Time
	cancel  context.CancelFunc

	mu       sync.Mutex
	status   fileExecutionStatus
	output   string
	result   *domain.FileExecResult
	err      string
	finished time.Time
	clients  map[*fileExecutionClient]struct{}
}

func (j *fileExecutionJob) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status != fileExecutionRunning {
		return len(p), nil
	}
	j.output = appendFileExecutionOutput(j.output, p)
	j.broadcastLocked(fileExecutionStreamMessage{Type: "output", ID: j.id, Data: string(p)})
	return len(p), nil
}

func (j *fileExecutionJob) snapshot() fileExecutionSnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.snapshotLocked()
}

func (j *fileExecutionJob) snapshotLocked() fileExecutionSnapshot {
	snapshot := fileExecutionSnapshot{
		ID:        j.id,
		Path:      j.path,
		Status:    j.status,
		Output:    j.output,
		StartedAt: j.started,
	}
	if j.result != nil {
		code := j.result.ExitCode
		snapshot.ExitCode = &code
		snapshot.TimedOut = j.result.TimedOut
	}
	snapshot.Error = j.err
	return snapshot
}

func (j *fileExecutionJob) stateMessageLocked() fileExecutionStreamMessage {
	snapshot := j.snapshotLocked()
	return fileExecutionStreamMessage{
		Type:      "state",
		ID:        snapshot.ID,
		Path:      snapshot.Path,
		Status:    snapshot.Status,
		Output:    snapshot.Output,
		ExitCode:  snapshot.ExitCode,
		TimedOut:  snapshot.TimedOut,
		Error:     snapshot.Error,
		StartedAt: snapshot.StartedAt,
	}
}

func (j *fileExecutionJob) attach(client *fileExecutionClient) {
	j.mu.Lock()
	client.send <- j.stateMessageLocked()
	if j.status == fileExecutionRunning {
		j.clients[client] = struct{}{}
	}
	j.mu.Unlock()
}

func (j *fileExecutionJob) detach(client *fileExecutionClient) {
	j.mu.Lock()
	delete(j.clients, client)
	j.mu.Unlock()
	client.halt()
	j.hub.prune(j.project)
}

func (j *fileExecutionJob) broadcastLocked(message fileExecutionStreamMessage) {
	for client := range j.clients {
		select {
		case client.send <- message:
		default:
			delete(j.clients, client)
			client.halt()
		}
	}
}

func (j *fileExecutionJob) finish(result domain.FileExecResult, err error) {
	j.mu.Lock()
	if j.status != fileExecutionRunning {
		j.mu.Unlock()
		return
	}
	if err != nil {
		j.status = fileExecutionFailed
		j.err = err.Error()
	} else {
		j.result = &domain.FileExecResult{
			ExitCode: result.ExitCode,
			Output:   j.output,
			TimedOut: result.TimedOut,
		}
		if result.ExitCode == 0 && !result.TimedOut {
			j.status = fileExecutionCompleted
		} else {
			j.status = fileExecutionFailed
		}
	}
	j.finished = time.Now()
	j.broadcastLocked(j.stateMessageLocked())
	j.mu.Unlock()
	j.hub.prune(j.project)
}

func (j *fileExecutionJob) stop() fileExecutionSnapshot {
	j.mu.Lock()
	if j.status != fileExecutionRunning {
		snapshot := j.snapshotLocked()
		j.mu.Unlock()
		return snapshot
	}
	j.status = fileExecutionStopped
	j.finished = time.Now()
	j.broadcastLocked(j.stateMessageLocked())
	snapshot := j.snapshotLocked()
	j.mu.Unlock()
	j.cancel()
	j.hub.prune(j.project)
	return snapshot
}

type fileExecutionHub struct {
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	jobs     map[string]*fileExecutionJob
	sequence atomic.Uint64
	once     sync.Once
}

func newFileExecutionHub() *fileExecutionHub {
	ctx, cancel := context.WithCancel(context.Background())
	return &fileExecutionHub{ctx: ctx, cancel: cancel, jobs: make(map[string]*fileExecutionJob)}
}

func (h *fileExecutionHub) start(project, path string, execute func(context.Context, io.Writer) (domain.FileExecResult, error)) (*fileExecutionJob, bool) {
	sequence := h.sequence.Add(1)
	job := &fileExecutionJob{
		hub:     h,
		project: project,
		id:      fmt.Sprintf("file-execution-%d", sequence),
		path:    path,
		started: time.Now(),
		status:  fileExecutionRunning,
		clients: make(map[*fileExecutionClient]struct{}),
	}
	ctx, cancel := context.WithCancel(h.ctx)
	job.cancel = cancel
	h.mu.Lock()
	active := 0
	for _, existing := range h.jobs {
		existing.mu.Lock()
		if existing.status == fileExecutionRunning {
			active++
		}
		existing.mu.Unlock()
	}
	if active >= fileExecutionConcurrentLimit {
		h.mu.Unlock()
		cancel()
		return nil, false
	}
	h.jobs[job.id] = job
	h.mu.Unlock()
	go func() {
		result, err := execute(ctx, job)
		job.finish(result, err)
	}()
	return job, true
}

func (h *fileExecutionHub) get(project, id string) *fileExecutionJob {
	h.mu.Lock()
	defer h.mu.Unlock()
	job := h.jobs[id]
	if job == nil || job.project != project {
		return nil
	}
	return job
}

func (h *fileExecutionHub) dismiss(project, id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	job := h.jobs[id]
	if job == nil || job.project != project {
		return false
	}
	job.mu.Lock()
	running := job.status == fileExecutionRunning
	job.mu.Unlock()
	if running {
		return false
	}
	delete(h.jobs, id)
	return true
}

func (h *fileExecutionHub) list(project string) []fileExecutionSnapshot {
	h.mu.Lock()
	jobs := make([]*fileExecutionJob, 0, len(h.jobs))
	for _, job := range h.jobs {
		if job.project == project {
			jobs = append(jobs, job)
		}
	}
	h.mu.Unlock()
	snapshots := make([]fileExecutionSnapshot, 0, len(jobs))
	for _, job := range jobs {
		snapshots = append(snapshots, job.snapshot())
	}
	sort.SliceStable(snapshots, func(i, j int) bool {
		return snapshots[i].StartedAt.After(snapshots[j].StartedAt)
	})
	return snapshots
}

func (h *fileExecutionHub) prune(project string) {
	type historyCandidate struct {
		job      *fileExecutionJob
		finished time.Time
	}

	h.mu.Lock()
	candidates := make([]historyCandidate, 0, len(h.jobs))
	for _, job := range h.jobs {
		if job.project != project {
			continue
		}
		job.mu.Lock()
		terminal := job.status != fileExecutionRunning && len(job.clients) == 0
		finished := job.finished
		job.mu.Unlock()
		if terminal {
			candidates = append(candidates, historyCandidate{job: job, finished: finished})
		}
	}
	if len(candidates) <= fileExecutionHistoryLimit {
		h.mu.Unlock()
		return
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].finished.Before(candidates[j].finished)
	})
	remove := len(candidates) - fileExecutionHistoryLimit
	for _, candidate := range candidates {
		if remove == 0 {
			break
		}
		if h.jobs[candidate.job.id] == candidate.job {
			delete(h.jobs, candidate.job.id)
			remove--
		}
	}
	h.mu.Unlock()
}

func (h *fileExecutionHub) stopAll() {
	h.once.Do(func() {
		h.mu.Lock()
		jobs := make([]*fileExecutionJob, 0, len(h.jobs))
		for _, job := range h.jobs {
			jobs = append(jobs, job)
		}
		h.mu.Unlock()
		for _, job := range jobs {
			job.stop()
		}
		h.cancel()
	})
}

type fileExecutionStartRequest struct {
	Path string `json:"path"`
}

func (s *Server) fileExecutionProject(r *http.Request) string {
	project := r.Header.Get("X-Project")
	if project == "" {
		project = r.URL.Query().Get("project")
	}
	if project == "" {
		s.mu.RLock()
		project = s.defaultProject
		s.mu.RUnlock()
	}
	return project
}

func (s *Server) StartFileExecution(c echo.Context) error {
	project := s.fileExecutionProject(c.Request())
	app, err := s.getAppByProject(c.Request().Context(), project)
	if err != nil {
		writeError(c.Response(), err)
		return nil
	}
	var req fileExecutionStartRequest
	if !decodeBody(c.Response(), c.Request(), &req) {
		return nil
	}
	if req.Path == "" {
		writeError(c.Response(), fmt.Errorf("%w: path is required", domain.ErrInvalidArgument))
		return nil
	}
	if err := app.Files.ValidateExecutable(c.Request().Context(), req.Path); err != nil {
		writeError(c.Response(), err)
		return nil
	}
	job, started := s.fileExecutions.start(app.Project.Name, req.Path, func(ctx context.Context, output io.Writer) (domain.FileExecResult, error) {
		return app.Files.ExecuteStream(ctx, req.Path, output)
	})
	if !started {
		return echo.NewHTTPError(http.StatusTooManyRequests, "too many file executions are running")
	}
	return c.JSON(http.StatusCreated, job.snapshot())
}

func (s *Server) ListFileExecutions(c echo.Context) error {
	project := s.fileExecutionProject(c.Request())
	if _, err := s.getAppByProject(c.Request().Context(), project); err != nil {
		writeError(c.Response(), err)
		return nil
	}
	writeJSONResponse(c.Response(), http.StatusOK, s.fileExecutions.list(project))
	return nil
}

func (s *Server) StopFileExecution(c echo.Context) error {
	project := s.fileExecutionProject(c.Request())
	if _, err := s.getAppByProject(c.Request().Context(), project); err != nil {
		writeError(c.Response(), err)
		return nil
	}
	job := s.fileExecutions.get(project, c.Param("id"))
	if job == nil {
		writeError(c.Response(), fmt.Errorf("%w: execution %s", domain.ErrNotFound, c.Param("id")))
		return nil
	}
	return c.JSON(http.StatusOK, job.stop())
}

func (s *Server) DismissFileExecution(c echo.Context) error {
	project := s.fileExecutionProject(c.Request())
	if _, err := s.getAppByProject(c.Request().Context(), project); err != nil {
		writeError(c.Response(), err)
		return nil
	}
	if !s.fileExecutions.dismiss(project, c.Param("id")) {
		writeError(c.Response(), fmt.Errorf("%w: execution %s", domain.ErrNotFound, c.Param("id")))
		return nil
	}
	return c.NoContent(http.StatusNoContent)
}
