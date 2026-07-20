package application

import (
	"log/slog"

	"github.com/syunkitada/myaitoolbox/agentrun/internal/domain"
)

// Detector finds tasks that need attention and handles recovery of crashed tasks.
type Detector struct {
	taskStore   domain.TaskStore
	lockManager domain.LockManager
	history     domain.HistoryRecorder
	logger      *slog.Logger
}

// NewDetector creates a Detector.
func NewDetector(
	taskStore domain.TaskStore,
	lockManager domain.LockManager,
	history domain.HistoryRecorder,
	logger *slog.Logger,
) *Detector {
	return &Detector{
		taskStore:   taskStore,
		lockManager: lockManager,
		history:     history,
		logger:      logger,
	}
}

// DetectInboxTasks returns all tasks with status: inbox.
func (d *Detector) DetectInboxTasks() ([]*domain.TaskInfo, error) {
	return d.taskStore.ListTaskDirsByStatus(domain.StatusInbox)
}

// DetectWaitingTasks returns all tasks with status: waiting.
func (d *Detector) DetectWaitingTasks() ([]*domain.TaskInfo, error) {
	return d.taskStore.ListTaskDirsByStatus(domain.StatusWaiting)
}

// DetectRetryableTasks returns inprogress tasks that have no lock and can be retried.
// These are tasks that failed in a previous run and are waiting to be retried.
func (d *Detector) DetectRetryableTasks() ([]*domain.TaskInfo, error) {
	tasks, err := d.taskStore.ListTaskDirsByStatus(domain.StatusInProgress)
	if err != nil {
		return nil, err
	}

	var retryable []*domain.TaskInfo
	for _, task := range tasks {
		// Skip human-assigned tasks
		if task.Metadata.CurrentAssignee == "human" {
			continue
		}
		// Skip tasks that still have a lock (agent might still be running)
		locked, err := d.lockManager.IsLocked(task.TaskDir)
		if err != nil {
			continue
		}
		if locked {
			continue
		}
		// Task has no lock and is inprogress -> can be retried
		retryable = append(retryable, task)
	}
	return retryable, nil
}

// RecoverCrashedTasks finds inprogress tasks with stale locks and resets them.
// "Crashed" means the agent process died while holding the lock (stale lock detected).
func (d *Detector) RecoverCrashedTasks() error {
	inProgressTasks, err := d.taskStore.ListTaskDirsByStatus(domain.StatusInProgress)
	if err != nil {
		return err
	}

	for _, task := range inProgressTasks {
		// Skip human-assigned tasks
		if task.Metadata.CurrentAssignee == "human" {
			continue
		}

		recovered, err := d.recoverIfCrashed(task)
		if err != nil {
			d.logger.Error("recovery check failed",
				"task_id", task.TaskID,
				"error", err,
			)
			continue
		}
		if recovered {
			d.logger.Info("recovered crashed task",
				"task_id", task.TaskID,
				"previous_assignee", task.Metadata.CurrentAssignee,
			)
		}
	}
	return nil
}

// recoverIfCrashed only recovers tasks that have a stale lock.
// Tasks without any lock are NOT treated as crashed - they are retryable tasks.
func (d *Detector) recoverIfCrashed(task *domain.TaskInfo) (bool, error) {
	locked, err := d.lockManager.IsLocked(task.TaskDir)
	if err != nil {
		return false, err
	}

	if !locked {
		// No lock = not crashed, just failed and waiting for retry
		return false, nil
	}

	// Lock exists - check if it's stale (process crashed while holding lock)
	stale, err := d.lockManager.IsStale(task.TaskDir)
	if err != nil {
		return false, err
	}

	if stale {
		// Kill orphan process if running
		if err := d.lockManager.CleanupOrphan(task.TaskDir); err != nil {
			d.logger.Warn("failed to cleanup orphan",
				"task_id", task.TaskID,
				"error", err,
			)
		}
		if err := d.lockManager.ForceRelease(task.TaskDir); err != nil {
			return false, err
		}
		return d.resetToInbox(task)
	}

	return false, nil
}

func (d *Detector) resetToInbox(task *domain.TaskInfo) (bool, error) {
	// Reset metadata status, but preserve retry_count
	task.Metadata.Status = domain.StatusInbox
	if err := d.taskStore.WriteMetadata(task.TaskDir, task.Metadata); err != nil {
		return false, err
	}

	// Record history
	d.history.Record(task.TaskDir, &domain.HistoryEntry{
		Type:    "recovered",
		Content: "Task recovered from crashed state and reset to inbox.",
	})

	return true, nil
}
