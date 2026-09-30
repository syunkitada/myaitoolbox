package automation

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

func TestCronEventsBetweenReturnsScheduledOccurrences(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Tokyo")
	require.NoError(t, err)
	def := domain.TriggerDefinition{
		ID:       "daily_report",
		Type:     domain.TriggerTypeCron,
		Cron:     "0 9 * * 1-5",
		Timezone: "Asia/Tokyo",
	}
	after := time.Date(2026, 9, 25, 8, 59, 0, 0, loc)
	now := time.Date(2026, 9, 25, 9, 1, 0, 0, loc)
	events, err := CronEventsBetween(context.Background(), def, after, now)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "daily_report@2026-09-25T09:00:00+09:00", events[0].ID)
	assert.Equal(t, time.Date(2026, 9, 25, 9, 0, 0, 0, loc), events[0].ScheduledAt)
}

func TestScanFileEventsFiltersAndRequiresSettledFiles(t *testing.T) {
	root := t.TempDir()
	watch := filepath.Join(root, "incoming")
	require.NoError(t, os.MkdirAll(watch, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(watch, "report.md"), []byte("report"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(watch, "ignore.txt"), []byte("ignore"), 0o644))
	def := domain.TriggerDefinition{
		ID:          "watch_reports",
		ProjectRoot: root,
		Enabled:     true,
		Type:        domain.TriggerTypeFileCreated,
		WatchPath:   watch,
		Pattern:     "*.md",
		SettleFor:   0,
	}

	events, err := ScanFileEvents(context.Background(), def, time.Now())
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "incoming/report.md", events[0].SourcePath)
	assert.Contains(t, events[0].ID, "file:watch_reports:incoming/report.md:")
}

func TestFileWatcherEmitsCreatedFile(t *testing.T) {
	root := t.TempDir()
	watch := filepath.Join(root, "incoming")
	require.NoError(t, os.MkdirAll(watch, 0o755))
	def := domain.TriggerDefinition{
		ID:          "watch_reports",
		ProjectRoot: root,
		Enabled:     true,
		Type:        domain.TriggerTypeFileCreated,
		WatchPath:   watch,
		Pattern:     "*.md",
		SettleFor:   0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := NewFileWatcher([]domain.TriggerDefinition{def}).Events(ctx)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(watch, "new.md"), []byte("new"), 0o644))

	select {
	case event := <-events:
		assert.Equal(t, "incoming/new.md", event.SourcePath)
		assert.Equal(t, def.ID, event.TriggerID)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for file event")
	}
}

func TestFileWatcherEmitsCreatedFileForEachDefinitionSharingWatchPath(t *testing.T) {
	root := t.TempDir()
	watch := filepath.Join(root, "incoming")
	require.NoError(t, os.MkdirAll(watch, 0o755))
	definitions := []domain.TriggerDefinition{
		{
			ID:          "watch_reports",
			ProjectRoot: root,
			Enabled:     true,
			Type:        domain.TriggerTypeFileCreated,
			WatchPath:   watch,
			Pattern:     "*.md",
			SettleFor:   0,
		},
		{
			ID:          "watch_logs",
			ProjectRoot: root,
			Enabled:     true,
			Type:        domain.TriggerTypeFileCreated,
			WatchPath:   watch,
			Pattern:     "*.txt",
			SettleFor:   0,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := NewFileWatcher(definitions).Events(ctx)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(watch, "report.md"), []byte("report"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(watch, "worker.txt"), []byte("log"), 0o644))

	got := make(map[string]bool)
	deadline := time.After(3 * time.Second)
	for len(got) < len(definitions) {
		select {
		case event := <-events:
			got[event.TriggerID] = true
		case <-deadline:
			t.Fatalf("timed out waiting for events, got %v", got)
		}
	}
	assert.True(t, got["watch_reports"])
	assert.True(t, got["watch_logs"])
}
