package automation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

func TestRunStoreSaveFindAndList(t *testing.T) {
	store := NewRunStore(t.TempDir())
	ctx := context.Background()
	run := domain.AutomationRun{
		ID:         "run-1",
		Project:    "demo",
		TriggerID:  "daily_report",
		EventID:    "daily_report@2026-09-27T09:00:00+09:00",
		Status:     domain.RunDispatched,
		TaskID:     "20260927_daily-report",
		StartedAt:  time.Date(2026, 9, 27, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
		SourcePath: "incoming/report.md",
	}
	require.NoError(t, store.Save(ctx, run))

	got, err := store.Find(ctx, run.Project, run.TriggerID, run.EventID)
	require.NoError(t, err)
	assert.Equal(t, run.ID, got.ID)
	assert.Equal(t, run.Status, got.Status)
	assert.Equal(t, run.SourcePath, got.SourcePath)
	assert.True(t, run.StartedAt.Equal(got.StartedAt))

	list, err := store.List(ctx, run.Project, run.TriggerID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, run.ID, list[0].ID)
}

func TestRunStoreFindMissing(t *testing.T) {
	_, err := NewRunStore(t.TempDir()).Find(context.Background(), "demo", "trigger", "event")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
