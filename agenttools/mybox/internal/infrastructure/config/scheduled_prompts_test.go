package config

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

func TestScheduledPromptStorePersistsAndFiltersByProject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scheduled-prompts.yaml")
	store := NewScheduledPromptStoreAt(path)
	first := domain.ScheduledPrompt{
		ID:          "first",
		Project:     "alpha",
		Target:      "agent-a",
		Text:        "first prompt",
		ScheduledAt: time.Unix(200, 0).UTC(),
	}
	second := domain.ScheduledPrompt{
		ID:          "second",
		Project:     "beta",
		Target:      "agent-b",
		Text:        "second prompt",
		ScheduledAt: time.Unix(100, 0).UTC(),
	}
	require.NoError(t, store.Create(context.Background(), first))
	require.NoError(t, store.Create(context.Background(), second))

	got, err := NewScheduledPromptStoreAt(path).List(context.Background(), "alpha")
	require.NoError(t, err)
	assert.Equal(t, []domain.ScheduledPrompt{first}, got)

	all, err := store.List(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.Equal(t, "second", all[0].ID)
	assert.Equal(t, "first", all[1].ID)
}

func TestScheduledPromptStoreDelete(t *testing.T) {
	store := NewScheduledPromptStoreAt(filepath.Join(t.TempDir(), "scheduled-prompts.yaml"))
	prompt := domain.ScheduledPrompt{ID: "remove", Project: "alpha", Target: "agent", Text: "text", ScheduledAt: time.Now().Add(time.Hour)}
	require.NoError(t, store.Create(context.Background(), prompt))
	require.NoError(t, store.Delete(context.Background(), "alpha", "remove"))
	require.ErrorIs(t, store.Delete(context.Background(), "alpha", "remove"), domain.ErrNotFound)
	got, err := store.List(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, got)
}
