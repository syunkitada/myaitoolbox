package entrypoint

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/entrypoint/api"
)

func TestScheduledPromptAPI(t *testing.T) {
	s, _ := newTestServer(t)
	scheduledAt := time.Now().Add(10 * time.Minute).UTC().Truncate(time.Millisecond)

	rec := do(t, s, http.MethodPost, "/api/herdr/agents/scheduled-prompts", map[string]any{
		"target":       "agent-a",
		"text":         "send this later",
		"scheduled_at": scheduledAt,
	})
	require.Equal(t, http.StatusCreated, rec.Code)
	created := decode[api.ScheduledPrompt](t, rec)
	assert.NotEmpty(t, created.Id)
	assert.Equal(t, "agent-a", created.Target)
	assert.Equal(t, "send this later", created.Text)
	assert.Equal(t, scheduledAt, created.ScheduledAt)

	rec = do(t, s, http.MethodGet, "/api/herdr/agents/scheduled-prompts", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	listed := decode[[]api.ScheduledPrompt](t, rec)
	require.Len(t, listed, 1)
	assert.Equal(t, created.Id, listed[0].Id)

	rec = do(t, s, http.MethodDelete, "/api/herdr/agents/scheduled-prompts/"+created.Id, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	rec = do(t, s, http.MethodGet, "/api/herdr/agents/scheduled-prompts", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, decode[[]api.ScheduledPrompt](t, rec))
}

func TestScheduledPromptAPIRejectsPastTime(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/herdr/agents/scheduled-prompts", map[string]any{
		"target":       "agent-a",
		"text":         "too late",
		"scheduled_at": time.Now().Add(-time.Minute),
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestScheduledPromptWorkerUsesPersistedPrompt(t *testing.T) {
	s, _ := newTestServer(t)
	now := time.Now()
	prompt := domain.ScheduledPrompt{
		ID:          "persisted",
		Project:     "test",
		Target:      "agent-a",
		Text:        "send after restart",
		ScheduledAt: now.Add(-time.Minute),
	}
	require.NoError(t, s.scheduled.store.Create(context.Background(), prompt))
	var calls [][]string
	s.herdrRun = func(context.Context, ...string) ([]byte, error) {
		return nil, nil
	}
	s.scheduled.run = func(_ context.Context, prompt domain.ScheduledPrompt) error {
		calls = append(calls, []string{"agent", "prompt", prompt.Target, prompt.Text})
		return nil
	}
	s.scheduled.processDue(context.Background(), now)
	assert.Equal(t, [][]string{{"agent", "prompt", "agent-a", "send after restart"}}, calls)
	remaining, err := s.scheduled.store.List(context.Background(), "test")
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

func TestRunScheduledPromptResolvesLegacyAgentNameWithinProject(t *testing.T) {
	s, app := newTestServer(t)
	listOutput, err := json.Marshal(map[string]any{
		"result": map[string]any{
			"agents": []map[string]string{
				{"agent": "codex", "cwd": app.Project.Path, "pane_id": "w1:p1"},
				{"agent": "codex", "cwd": "/other/project", "pane_id": "w2:p1"},
			},
		},
	})
	require.NoError(t, err)
	var promptArgs []string
	s.herdrRun = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "agent" && args[1] == "list" {
			return listOutput, nil
		}
		promptArgs = args
		return nil, nil
	}

	err = s.runScheduledPrompt(context.Background(), domain.ScheduledPrompt{
		Project: "test",
		Target:  "codex",
		Text:    "send later",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"agent", "prompt", "w1:p1", "send later"}, promptArgs)
}
