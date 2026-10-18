package config

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

func TestBrowserAuditStoreAppendsOwnerOnlyJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "browser-audit.jsonl")
	store := NewBrowserAuditStoreAt(path)
	timestamp := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	events := []domain.BrowserAuditEvent{
		{Timestamp: timestamp, Event: "session_created", Project: "demo", SessionID: "session-1", Actor: "user", Result: "success"},
		{Timestamp: timestamp.Add(time.Second), Event: "browser_command", Project: "demo", SessionID: "session-1", Actor: "agent", Command: "observe", Result: "success"},
	}
	for _, event := range events {
		require.NoError(t, store.Append(context.Background(), event))
	}

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var got []domain.BrowserAuditEvent
	for scanner.Scan() {
		var event domain.BrowserAuditEvent
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &event))
		got = append(got, event)
	}
	require.NoError(t, scanner.Err())
	require.Equal(t, events, got)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestBrowserAuditStoreRejectsIncompleteEvents(t *testing.T) {
	store := NewBrowserAuditStoreAt(filepath.Join(t.TempDir(), "audit.jsonl"))
	require.EqualError(t, store.Append(context.Background(), domain.BrowserAuditEvent{}), "browser audit event identity is incomplete")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, store.Append(ctx, domain.BrowserAuditEvent{Event: "test", Project: "demo", Result: "success"}), context.Canceled)
}

func TestNewBrowserAuditStoreUsesConfiguredPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configured", "audit.jsonl")
	t.Setenv("MYBOX_BROWSER_AUDIT_LOG", path)

	store := NewBrowserAuditStore()
	require.NoError(t, store.Append(context.Background(), domain.BrowserAuditEvent{
		Event:   "test",
		Project: "demo",
		Result:  "success",
	}))
	_, err := os.Stat(path)
	require.NoError(t, err)
}

func TestBrowserAuditStoreListsFilteredEventsNewestFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	store := NewBrowserAuditStoreAt(path)
	base := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	for _, event := range []domain.BrowserAuditEvent{
		{Timestamp: base, Event: "session_created", Project: "demo", SessionID: "session-1", Result: "success"},
		{Timestamp: base.Add(time.Minute), Event: "browser_command", Project: "demo", SessionID: "session-1", Command: "navigate", Result: "blocked"},
		{Timestamp: base.Add(2 * time.Minute), Event: "browser_command", Project: "demo", SessionID: "session-2", Command: "observe", Result: "success"},
		{Timestamp: base.Add(3 * time.Minute), Event: "browser_command", Project: "other", SessionID: "session-3", Command: "observe", Result: "blocked"},
		{Timestamp: base.Add(4 * time.Minute), Event: "browser_command", Project: "demo", SessionID: "session-1", Command: "press", Result: "blocked"},
	} {
		require.NoError(t, store.Append(context.Background(), event))
	}

	page, err := store.List(context.Background(), domain.BrowserAuditQuery{
		Project:   "demo",
		SessionID: "session-1",
		Result:    "blocked",
		From:      base.Add(time.Minute),
		To:        base.Add(4 * time.Minute),
		Limit:     1,
	})
	require.NoError(t, err)
	require.Equal(t, 2, page.Total)
	require.Equal(t, 1, page.Limit)
	require.True(t, page.HasMore)
	require.Equal(t, base.Add(4*time.Minute), page.Events[0].Timestamp)

	page, err = store.List(context.Background(), domain.BrowserAuditQuery{
		Project:   "demo",
		SessionID: "session-1",
		Result:    "blocked",
		Limit:     1,
		Offset:    1,
	})
	require.NoError(t, err)
	require.False(t, page.HasMore)
	require.Equal(t, base.Add(time.Minute), page.Events[0].Timestamp)
}

func TestBrowserAuditStoreRotatesRetainedGenerationsAndPaginatesAcrossThem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	store := NewBrowserAuditStoreAt(path)
	firstEvent := browserAuditRotationEvent(1)
	firstLine, err := json.Marshal(firstEvent)
	require.NoError(t, err)
	store.maxBytes = int64((len(firstLine) + 1) * 2)
	store.backups = 2

	for index := 1; index <= 7; index++ {
		require.NoError(t, store.Append(context.Background(), browserAuditRotationEvent(index)))
		if index == 6 {
			// Rotating this file must restore owner-only permissions on the backup.
			require.NoError(t, os.Chmod(path, 0o644))
		}
	}

	for _, pageCase := range []struct {
		offset   int
		commands []string
		hasMore  bool
	}{
		{offset: 0, commands: []string{"command-07", "command-06"}, hasMore: true},
		{offset: 2, commands: []string{"command-05", "command-04"}, hasMore: true},
		{offset: 4, commands: []string{"command-03"}, hasMore: false},
	} {
		page, err := store.List(context.Background(), domain.BrowserAuditQuery{
			Project: "demo",
			Limit:   2,
			Offset:  pageCase.offset,
		})
		require.NoError(t, err)
		require.Equal(t, 5, page.Total)
		require.Equal(t, pageCase.hasMore, page.HasMore)
		commands := make([]string, 0, len(page.Events))
		for _, event := range page.Events {
			commands = append(commands, event.Command)
		}
		require.Equal(t, pageCase.commands, commands)
	}

	for _, rotatedPath := range []string{path, browserAuditBackupPath(path, 1), browserAuditBackupPath(path, 2)} {
		info, err := os.Stat(rotatedPath)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		require.LessOrEqual(t, info.Size(), store.maxBytes)
	}
	_, err = os.Stat(browserAuditBackupPath(path, 3))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestBrowserAuditStoreWithZeroBackupsRetainsOnlyCurrentLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	store := NewBrowserAuditStoreAt(path)
	eventLine, err := json.Marshal(browserAuditRotationEvent(1))
	require.NoError(t, err)
	store.maxBytes = int64((len(eventLine) + 1) * 2)
	store.backups = 0

	for index := 1; index <= 5; index++ {
		require.NoError(t, store.Append(context.Background(), browserAuditRotationEvent(index)))
	}
	page, err := store.List(context.Background(), domain.BrowserAuditQuery{Project: "demo"})
	require.NoError(t, err)
	require.Equal(t, 1, page.Total)
	require.Equal(t, "command-05", page.Events[0].Command)
	_, err = os.Stat(browserAuditBackupPath(path, 1))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestBrowserAuditStoreSerializesConcurrentProcessesDuringRotation(t *testing.T) {
	const workerCount = 4
	const eventsPerWorker = 20
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	startPath := path + ".start"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	commands := make([]*exec.Cmd, 0, workerCount)
	outputs := make([]*bytes.Buffer, 0, workerCount)
	readyPaths := make([]string, 0, workerCount)
	t.Cleanup(func() {
		cancel()
		for _, cmd := range commands {
			if cmd.Process == nil || cmd.ProcessState != nil {
				continue
			}
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	readerStore := NewBrowserAuditStoreAt(path)
	readerStore.backups = 100
	readerContext, stopReader := context.WithCancel(ctx)
	readerDone := make(chan error, 1)
	t.Cleanup(stopReader)
	go func() {
		for {
			_, err := readerStore.List(readerContext, domain.BrowserAuditQuery{Project: "demo", Limit: 100})
			if err != nil {
				if errors.Is(err, context.Canceled) {
					readerDone <- nil
					return
				}
				readerDone <- err
				return
			}
			select {
			case <-readerContext.Done():
				readerDone <- nil
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}()
	for worker := 0; worker < workerCount; worker++ {
		readyPath := fmt.Sprintf("%s.ready-%02d", path, worker)
		readyPaths = append(readyPaths, readyPath)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBrowserAuditStoreProcessWorker$")
		cmd.Env = append(os.Environ(),
			"MYBOX_TEST_BROWSER_AUDIT_WORKER=1",
			"MYBOX_TEST_BROWSER_AUDIT_PATH="+path,
			"MYBOX_TEST_BROWSER_AUDIT_START="+startPath,
			"MYBOX_TEST_BROWSER_AUDIT_READY="+readyPath,
			fmt.Sprintf("MYBOX_TEST_BROWSER_AUDIT_WORKER_ID=%d", worker),
			fmt.Sprintf("MYBOX_TEST_BROWSER_AUDIT_EVENT_COUNT=%d", eventsPerWorker),
		)
		output := &bytes.Buffer{}
		cmd.Stdout = output
		cmd.Stderr = output
		require.NoError(t, cmd.Start())
		commands = append(commands, cmd)
		outputs = append(outputs, output)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		readyCount := 0
		for _, readyPath := range readyPaths {
			if _, err := os.Stat(readyPath); err == nil {
				readyCount++
			} else {
				require.ErrorIs(t, err, os.ErrNotExist)
			}
		}
		if readyCount == workerCount {
			break
		}
		if time.Now().After(deadline) {
			require.FailNow(t, "timed out waiting for audit log workers to become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.NoError(t, os.WriteFile(startPath, []byte("start"), 0o600))

	for index, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("audit log worker %d failed: %v\n%s", index, err, outputs[index].String())
		}
	}
	stopReader()
	require.NoError(t, <-readerDone)

	store := NewBrowserAuditStoreAt(path)
	store.backups = 100
	page, err := store.List(context.Background(), domain.BrowserAuditQuery{Project: "demo", Limit: 100})
	require.NoError(t, err)
	require.Equal(t, workerCount*eventsPerWorker, page.Total)
	require.Len(t, page.Events, workerCount*eventsPerWorker)

	commandsSeen := make(map[string]struct{}, len(page.Events))
	for _, event := range page.Events {
		commandsSeen[event.Command] = struct{}{}
	}
	for worker := 0; worker < workerCount; worker++ {
		for index := 0; index < eventsPerWorker; index++ {
			command := fmt.Sprintf("worker-%02d-command-%03d", worker, index)
			require.Contains(t, commandsSeen, command)
		}
	}
	lockInfo, err := os.Stat(path + ".lock")
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), lockInfo.Mode().Perm())
}

func TestBrowserAuditStoreHonorsContextWhileWaitingForProcessLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	heldLock, err := lockBrowserAuditFile(context.Background(), path, true)
	require.NoError(t, err)
	defer func() { _ = heldLock.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = lockBrowserAuditFile(ctx, path, true)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), time.Second)
}

func TestBrowserAuditStoreRejectsSymlinkLockFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	target := filepath.Join(dir, "unrelated-file")
	targetContents := []byte("must remain untouched")
	require.NoError(t, os.WriteFile(target, targetContents, 0o644))
	if err := os.Symlink(target, path+".lock"); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	store := NewBrowserAuditStoreAt(path)
	err := store.Append(context.Background(), browserAuditRotationEvent(1))
	require.Error(t, err)
	gotContents, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	require.Equal(t, targetContents, gotContents)
	_, statErr := os.Stat(path)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestBrowserAuditStoreProcessWorker(t *testing.T) {
	if os.Getenv("MYBOX_TEST_BROWSER_AUDIT_WORKER") != "1" {
		return
	}
	path := os.Getenv("MYBOX_TEST_BROWSER_AUDIT_PATH")
	startPath := os.Getenv("MYBOX_TEST_BROWSER_AUDIT_START")
	readyPath := os.Getenv("MYBOX_TEST_BROWSER_AUDIT_READY")
	workerID, err := strconv.Atoi(os.Getenv("MYBOX_TEST_BROWSER_AUDIT_WORKER_ID"))
	require.NoError(t, err)
	eventCount, err := strconv.Atoi(os.Getenv("MYBOX_TEST_BROWSER_AUDIT_EVENT_COUNT"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(readyPath, []byte("ready"), 0o600))

	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(startPath); err == nil {
			break
		} else {
			require.ErrorIs(t, err, os.ErrNotExist)
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for concurrent audit workers to start")
		}
		time.Sleep(10 * time.Millisecond)
	}

	store := NewBrowserAuditStoreAt(path)
	event, err := json.Marshal(browserAuditConcurrentProcessEvent(workerID, 0))
	require.NoError(t, err)
	store.maxBytes = int64((len(event) + 1) * 2)
	store.backups = 100
	for index := 0; index < eventCount; index++ {
		require.NoError(t, store.Append(context.Background(), browserAuditConcurrentProcessEvent(workerID, index)))
	}
}

func browserAuditConcurrentProcessEvent(worker, index int) domain.BrowserAuditEvent {
	return domain.BrowserAuditEvent{
		Timestamp: time.Date(2026, time.January, 2, 3, 4, index, 0, time.UTC),
		Event:     "browser_command",
		Project:   "demo",
		SessionID: fmt.Sprintf("worker-%02d", worker),
		Command:   fmt.Sprintf("worker-%02d-command-%03d", worker, index),
		Result:    "success",
	}
}

func TestNewBrowserAuditStoreReadsRetentionSettings(t *testing.T) {
	t.Setenv("MYBOX_BROWSER_AUDIT_LOG", filepath.Join(t.TempDir(), "audit.jsonl"))
	t.Setenv("MYBOX_BROWSER_AUDIT_MAX_BYTES", "0")
	t.Setenv("MYBOX_BROWSER_AUDIT_BACKUPS", "0")

	store := NewBrowserAuditStore()
	require.Zero(t, store.maxBytes)
	require.Zero(t, store.backups)
}

func TestNewBrowserAuditStoreUsesDefaultsForInvalidRetentionSettings(t *testing.T) {
	t.Setenv("MYBOX_BROWSER_AUDIT_MAX_BYTES", "not-a-size")
	t.Setenv("MYBOX_BROWSER_AUDIT_BACKUPS", "101")

	store := NewBrowserAuditStore()
	require.Equal(t, defaultBrowserAuditMaxBytes, store.maxBytes)
	require.Equal(t, defaultBrowserAuditBackups, store.backups)
}

func browserAuditRotationEvent(index int) domain.BrowserAuditEvent {
	return domain.BrowserAuditEvent{
		Timestamp: time.Date(2026, time.January, 2, 3, 4, index, 0, time.UTC),
		Event:     "browser_command",
		Project:   "demo",
		SessionID: "session-1",
		Command:   fmt.Sprintf("command-%02d", index),
		Result:    "success",
	}
}

func TestBrowserAuditStoreRequiresProjectForListing(t *testing.T) {
	store := NewBrowserAuditStoreAt(filepath.Join(t.TempDir(), "audit.jsonl"))
	_, err := store.List(context.Background(), domain.BrowserAuditQuery{})
	require.EqualError(t, err, "browser audit project is required")
}
