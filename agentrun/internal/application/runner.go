package application

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/syunkitada/myaitoolbox/agentrun/internal/domain"
)

// Runner orchestrates task processing in oneshot mode.
type Runner struct {
	taskStore   domain.TaskStore
	lockManager domain.LockManager
	history     domain.HistoryRecorder
	spawner     domain.AgentSpawner
	detector    *Detector
	handoff     *HandoffProcessor
	agentsRoot  string
	logger      *slog.Logger
}

// NewRunner creates a Runner.
func NewRunner(
	taskStore domain.TaskStore,
	lockManager domain.LockManager,
	history domain.HistoryRecorder,
	spawner domain.AgentSpawner,
	detector *Detector,
	handoff *HandoffProcessor,
	agentsRoot string,
	logger *slog.Logger,
) *Runner {
	return &Runner{
		taskStore:   taskStore,
		lockManager: lockManager,
		history:     history,
		spawner:     spawner,
		detector:    detector,
		handoff:     handoff,
		agentsRoot:  agentsRoot,
		logger:      logger,
	}
}

// RunOneshot processes all inbox tasks and retries failed tasks, then exits.
func (r *Runner) RunOneshot() error {
	// Ensure workspace structure
	if err := r.taskStore.EnsureWorkspace(); err != nil {
		return fmt.Errorf("workspace setup failed: %w", err)
	}

	// Recover crashed tasks first (tasks with stale locks)
	if err := r.detector.RecoverCrashedTasks(); err != nil {
		r.logger.Error("recovery failed", "error", err)
	}

	// Detect inbox tasks
	inboxTasks, err := r.detector.DetectInboxTasks()
	if err != nil {
		return fmt.Errorf("failed to detect inbox tasks: %w", err)
	}

	// Detect retryable tasks (inprogress, no lock)
	retryableTasks, err := r.detector.DetectRetryableTasks()
	if err != nil {
		return fmt.Errorf("failed to detect retryable tasks: %w", err)
	}

	// Combine both lists
	allTasks := append(inboxTasks, retryableTasks...)

	if len(allTasks) == 0 {
		r.logger.Info("no tasks to process")
		return nil
	}

	r.logger.Info("found tasks to process", "count", len(allTasks), "inbox", len(inboxTasks), "retryable", len(retryableTasks))

	for _, task := range allTasks {
		if err := r.processTask(task); err != nil {
			r.logger.Error("failed to process task",
				"task_id", task.TaskID,
				"error", err,
			)
		}
	}

	return nil
}

// RunDaemon watches the workspace for changes and processes tasks continuously.
func (r *Runner) RunDaemon(ctx context.Context, watcher domain.Watcher) error {
	// Ensure workspace structure
	if err := r.taskStore.EnsureWorkspace(); err != nil {
		return fmt.Errorf("workspace setup failed: %w", err)
	}

	// Process existing tasks on startup
	r.RunOneshot()

	// Watch for new tasks
	tasksDir := filepath.Join(".", "tasks")
	events, err := watcher.Watch(tasksDir)
	if err != nil {
		return fmt.Errorf("failed to start watcher: %w", err)
	}
	defer watcher.Close()

	r.logger.Info("daemon started, watching for changes")

	// Periodic recovery check
	recoveryTicker := time.NewTicker(5 * time.Minute)
	defer recoveryTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("daemon shutting down")
			return nil
		case event, ok := <-events:
			if !ok {
				return fmt.Errorf("watcher channel closed")
			}
			r.logger.Info("filesystem event", "path", event.Path, "type", event.Type)
			// Small delay to allow batch writes
			time.Sleep(100 * time.Millisecond)
			if err := r.processEvent(event); err != nil {
				r.logger.Error("failed to process event", "error", err)
			}
		case <-recoveryTicker.C:
			if err := r.detector.RecoverCrashedTasks(); err != nil {
				r.logger.Error("periodic recovery failed", "error", err)
			}
		}
	}
}

func (r *Runner) processEvent(event domain.WatchEvent) error {
	// Determine the task directory from the event path
	taskDir := filepath.Dir(event.Path)
	taskID := filepath.Base(taskDir)

	// Read metadata
	meta, err := r.taskStore.ReadMetadata(taskDir)
	if err != nil {
		r.logger.Debug("skipping event (no metadata)", "path", event.Path)
		return nil
	}

	task := &domain.TaskInfo{
		TaskDir:  taskDir,
		TaskID:   taskID,
		Metadata: meta,
	}

	// Only process inbox tasks or retryable tasks
	switch meta.Status {
	case domain.StatusInbox:
		return r.processTask(task)
	case domain.StatusInProgress:
		// Check if it's retryable
		locked, err := r.lockManager.IsLocked(taskDir)
		if err != nil {
			return err
		}
		if !locked && meta.CurrentAssignee != "human" {
			return r.processTask(task)
		}
	}

	return nil
}

func (r *Runner) processTask(task *domain.TaskInfo) error {
	r.logger.Info("processing task",
		"task_id", task.TaskID,
		"assignee", task.Metadata.CurrentAssignee,
	)

	// Skip human-assigned tasks
	if task.Metadata.CurrentAssignee == "human" {
		r.logger.Info("task assigned to human, skipping",
			"task_id", task.TaskID,
		)
		return nil
	}

	// Acquire lock
_hostname, _ := os.Hostname()
lockInfo := domain.NewLockInfo(os.Getpid(), 0, _hostname)
	if err := r.lockManager.Acquire(task.TaskDir, lockInfo); err != nil {
		r.logger.Warn("failed to acquire lock, skipping",
			"task_id", task.TaskID,
			"error", err,
		)
		return nil
	}
	defer r.lockManager.Release(task.TaskDir)

	// Update status to inprogress
	task.Metadata.Status = domain.StatusInProgress
	if err := r.taskStore.WriteMetadata(task.TaskDir, task.Metadata); err != nil {
		return err
	}

	// Record start
	r.history.Record(task.TaskDir, &domain.HistoryEntry{
		Type:    "started",
		Content: fmt.Sprintf("Agent: %s", task.Metadata.CurrentAssignee),
	})

	// Spawn agent
	agentDir := filepath.Join(r.agentsRoot, extractAgentClass(task.Metadata.CurrentAssignee))
	result, err := r.spawner.Spawn(task, agentDir)
	if err != nil {
		return r.handleAgentError(task, err)
	}

	// Check exit code
	if result.ExitCode != 0 {
		return r.handleAgentError(task, fmt.Errorf("agent exited with code %d: %s", result.ExitCode, result.Stderr))
	}

	// Validate artifacts
	if err := r.validateArtifacts(task); err != nil {
		r.logger.Warn("artifact validation warning",
			"task_id", task.TaskID,
			"error", err,
		)
	}

	// Check for handoff
	handoffProcessed, err := r.handoff.ProcessHandoff(task.TaskDir, task.Metadata)
	if err != nil {
		return err
	}

	if handoffProcessed {
		// Handoff was processed, task is now in inbox with new assignee
		return nil
	}

	// No handoff -> task is done
	task.Metadata.Status = domain.StatusDone
	if err := r.taskStore.WriteMetadata(task.TaskDir, task.Metadata); err != nil {
		return err
	}

	r.history.Record(task.TaskDir, &domain.HistoryEntry{
		Type:    "finished",
		Content: "Task completed successfully.",
	})

	// Move to archive
	if err := r.taskStore.MoveToArchive(task.TaskDir); err != nil {
		r.logger.Error("failed to archive task",
			"task_id", task.TaskID,
			"error", err,
		)
	}

	r.logger.Info("task completed", "task_id", task.TaskID)
	return nil
}

func (r *Runner) handleAgentError(task *domain.TaskInfo, err error) error {
	r.logger.Error("agent error",
		"task_id", task.TaskID,
		"error", err,
	)

	task.Metadata.RetryCount++
	if task.Metadata.RetryCount > task.Metadata.MaxRetry {
		// Max retry exceeded, hand off to human
		task.Metadata.CurrentAssignee = "human"
		r.history.Record(task.TaskDir, &domain.HistoryEntry{
			Type:    "error",
			Content: fmt.Sprintf("Max retry exceeded. Last error: %s", err.Error()),
		})
		r.logger.Warn("max retry exceeded, handing off to human",
			"task_id", task.TaskID,
			"retry_count", task.Metadata.RetryCount,
		)
	} else {
		// Retry: keep inprogress but update retry count
		r.history.Record(task.TaskDir, &domain.HistoryEntry{
			Type:    "error",
			Content: fmt.Sprintf("Retry %d/%d. Error: %s", task.Metadata.RetryCount, task.Metadata.MaxRetry, err.Error()),
		})
	}

	return r.taskStore.WriteMetadata(task.TaskDir, task.Metadata)
}

func (r *Runner) validateArtifacts(task *domain.TaskInfo) error {
	artifactsDir := filepath.Join(task.TaskDir, "artifacts")
	entries, err := os.ReadDir(artifactsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("artifacts directory does not exist")
		}
		return err
	}

	if len(entries) == 0 {
		return fmt.Errorf("artifacts directory is empty")
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Size() == 0 {
			return fmt.Errorf("artifact file is empty: %s", entry.Name())
		}
	}

	return nil
}

func extractAgentClass(assignee string) string {
	result := assignee
	for i := len(result) - 1; i >= 0; i-- {
		c := result[i]
		if c == '*' || (c >= '0' && c <= '9') {
			result = result[:i]
		} else {
			break
		}
	}
	return result
}
