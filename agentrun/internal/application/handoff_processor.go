package application

import (
	"log/slog"
	"os"

	"github.com/syunkitada/myaitoolbox/agentrun/internal/domain"
)

// HandoffProcessor reads handoff.yaml and updates task routing.
type HandoffProcessor struct {
	taskStore domain.TaskStore
	history   domain.HistoryRecorder
	logger    *slog.Logger
}

// NewHandoffProcessor creates a HandoffProcessor.
func NewHandoffProcessor(
	taskStore domain.TaskStore,
	history domain.HistoryRecorder,
	logger *slog.Logger,
) *HandoffProcessor {
	return &HandoffProcessor{
		taskStore: taskStore,
		history:   history,
		logger:    logger,
	}
}

// ProcessHandoff reads handoff.yaml from the task directory and routes accordingly.
// Returns true if a handoff was processed.
func (p *HandoffProcessor) ProcessHandoff(taskDir string, meta *domain.Metadata) (bool, error) {
	handoffPath := taskDir + "/handoff.yaml"
	data, err := os.ReadFile(handoffPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	handoff, err := domain.ParseHandoff(data)
	if err != nil {
		p.logger.Warn("invalid handoff.yaml, skipping",
			"task_dir", taskDir,
			"error", err,
		)
		return false, nil
	}

	// Record history
	p.history.Record(taskDir, &domain.HistoryEntry{
		Type:    "handoff",
		Content: "Handoff to: " + handoff.NextAssignee + "\nReason: " + handoff.Reason,
	})

	// Update metadata
	meta.CurrentAssignee = handoff.NextAssignee
	meta.Status = domain.StatusInbox
	meta.RetryCount = 0
	if err := p.taskStore.WriteMetadata(taskDir, meta); err != nil {
		return false, err
	}

	p.logger.Info("handoff processed",
		"task_dir", taskDir,
		"next_assignee", handoff.NextAssignee,
	)

	return true, nil
}
