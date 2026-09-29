package entrypoint

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/entrypoint/api"
)

var scheduledPromptSequence uint64

// runScheduledPrompt uses the pane ID stored by the current UI. For prompts
// created by the earlier browser implementation, Target may still be an
// agent name such as "codex"; resolve that name within the scheduled project
// before sending so multiple agents of the same kind cannot make it ambiguous.
func (s *Server) runScheduledPrompt(ctx context.Context, prompt domain.ScheduledPrompt) error {
	target := prompt.Target
	if !strings.Contains(target, ":") {
		resolved, err := s.resolveScheduledPromptTarget(ctx, prompt.Project, target)
		if err != nil {
			return err
		}
		target = resolved
	}
	_, err := s.runHerdr(ctx, "agent", "prompt", target, prompt.Text)
	return err
}

func (s *Server) resolveScheduledPromptTarget(ctx context.Context, project, target string) (string, error) {
	out, err := s.runHerdr(ctx, "agent", "list")
	if err != nil {
		return "", err
	}
	var envelope herdrEnvelope
	if err := json.Unmarshal(out, &envelope); err != nil {
		return "", fmt.Errorf("invalid herdr agent list output: %w", err)
	}
	var list herdrAgentListResult
	if err := json.Unmarshal(envelope.Result, &list); err != nil {
		return "", fmt.Errorf("invalid herdr agent list result: %w", err)
	}
	projectPath := s.projectPath(project)
	candidates := make([]string, 0, len(list.Agents))
	for _, agent := range list.Agents {
		if agent.Agent != target && agent.Name != target {
			continue
		}
		if projectPath != "" && !pathIsWithin(projectPath, agent.Cwd) {
			continue
		}
		if agent.PaneID != "" {
			candidates = append(candidates, agent.PaneID)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("scheduled prompt target %q was not found in project %q", target, project)
	}
	return "", fmt.Errorf("scheduled prompt target %q is ambiguous in project %q", target, project)
}

func (s *Server) projectPath(project string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if app := s.apps[project]; app != nil && app.Project != nil {
		return app.Project.Path
	}
	for _, candidate := range s.config.Projects {
		if candidate.Name == project {
			return candidate.Path
		}
	}
	return ""
}

func pathIsWithin(root, candidate string) bool {
	if root == "" || candidate == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func (s *Server) ListHerdrScheduledPrompts(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	prompts, err := s.scheduled.store.List(r.Context(), app.Project.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	result := make([]api.ScheduledPrompt, 0, len(prompts))
	for _, prompt := range prompts {
		result = append(result, toAPIScheduledPrompt(prompt))
	}
	writeJSONResponse(w, http.StatusOK, result)
}

func (s *Server) CreateHerdrScheduledPrompt(w http.ResponseWriter, r *http.Request) {
	var req api.CreateHerdrScheduledPromptRequest
	if !decodeBody(w, r, &req) {
		return
	}
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	target := strings.TrimSpace(req.Target)
	text := strings.TrimSpace(req.Text)
	if !herdrTargetPattern.MatchString(target) {
		writeError(w, fmt.Errorf("%w: invalid agent target", domain.ErrInvalidArgument))
		return
	}
	if text == "" || len(text) > 4096 {
		writeError(w, fmt.Errorf("%w: prompt text must be between 1 and 4096 characters", domain.ErrInvalidArgument))
		return
	}
	if req.ScheduledAt.IsZero() || !req.ScheduledAt.After(time.Now()) {
		writeError(w, fmt.Errorf("%w: schedule time must be in the future", domain.ErrInvalidArgument))
		return
	}
	prompt := domain.ScheduledPrompt{
		ID:          newScheduledPromptID(),
		Project:     app.Project.Name,
		Target:      target,
		Text:        text,
		ScheduledAt: req.ScheduledAt,
	}
	if err := s.scheduled.store.Create(r.Context(), prompt); err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusCreated, toAPIScheduledPrompt(prompt))
}

func (s *Server) DeleteHerdrScheduledPrompt(w http.ResponseWriter, r *http.Request, id string) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.scheduled.store.Delete(r.Context(), app.Project.Name, id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func newScheduledPromptID() string {
	sequence := atomic.AddUint64(&scheduledPromptSequence, 1)
	return fmt.Sprintf("sp-%d-%d", time.Now().UnixNano(), sequence)
}

func toAPIScheduledPrompt(prompt domain.ScheduledPrompt) api.ScheduledPrompt {
	return api.ScheduledPrompt{
		Id:          prompt.ID,
		Target:      prompt.Target,
		Text:        prompt.Text,
		ScheduledAt: prompt.ScheduledAt,
	}
}
