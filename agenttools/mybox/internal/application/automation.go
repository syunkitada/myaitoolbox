package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/fsutil"
)

type AutomationUseCase struct {
	Project     string
	ProjectPath string
	Runs        domain.AutomationRunStore
	Tasks       *TaskUseCase
	Agent       domain.AgentDispatcher
	Now         func() time.Time
}

func NewAutomationUseCase(project, projectPath string, runs domain.AutomationRunStore, tasks *TaskUseCase, agent domain.AgentDispatcher) *AutomationUseCase {
	return &AutomationUseCase{
		Project:     project,
		ProjectPath: projectPath,
		Runs:        runs,
		Tasks:       tasks,
		Agent:       agent,
		Now:         time.Now,
	}
}

// RunManual executes one explicit trigger invocation. Each invocation gets a
// unique event ID so repeated manual runs create independent tasks.
func (u *AutomationUseCase) RunManual(ctx context.Context, def domain.TriggerDefinition) (*domain.AutomationRun, error) {
	if !def.Enabled {
		return nil, fmt.Errorf("%w: trigger %s is disabled", domain.ErrInvalidArgument, def.ID)
	}
	now := u.now()
	eventID, err := manualEventID(now, def.ID)
	if err != nil {
		return nil, err
	}
	return u.RunEvent(ctx, def, domain.TriggerEvent{
		ID:          eventID,
		TriggerID:   def.ID,
		OccurredAt:  now,
		ScheduledAt: now,
	})
}

// RunEvent converts one trigger event into one task and dispatches its agent.
// The event ID is the idempotency key, so repeated filesystem or scheduler
// notifications return the original run without creating another task.
func (u *AutomationUseCase) RunEvent(ctx context.Context, def domain.TriggerDefinition, event domain.TriggerEvent) (*domain.AutomationRun, error) {
	if u.Runs == nil || u.Tasks == nil {
		return nil, fmt.Errorf("%w: automation dependencies are incomplete", domain.ErrInvalidArgument)
	}
	if strings.TrimSpace(event.ID) == "" {
		return nil, fmt.Errorf("%w: event id is required", domain.ErrInvalidArgument)
	}
	if existing, err := u.Runs.Find(ctx, u.Project, def.ID, event.ID); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	now := u.now()
	run := domain.AutomationRun{
		ID:         automationRunID(def.ID, event.ID, eventTime(event, now)),
		Project:    u.Project,
		TriggerID:  def.ID,
		EventID:    event.ID,
		Status:     domain.RunCreated,
		SourcePath: filepath.ToSlash(event.SourcePath),
		StartedAt:  now,
	}
	if def.Type != domain.TriggerTypeManual && def.Overlap == domain.OverlapSkip {
		active, err := u.hasActiveRun(ctx, def.ID)
		if err != nil {
			return nil, err
		}
		if active {
			run.Status = domain.RunSkipped
			run.FinishedAt = now
			if err := u.Runs.Save(ctx, run); err != nil {
				return nil, err
			}
			return &run, nil
		}
	}
	if err := u.Runs.Save(ctx, run); err != nil {
		return nil, err
	}

	content, err := os.ReadFile(def.TemplatePath)
	if err != nil {
		return u.failRun(ctx, run, err)
	}
	taskID := automationTaskID(def.ID, event.ID, eventTime(event, now))
	task, err := u.Tasks.CreateFromContent(ctx, taskID, string(content), def.AgentKind, def.Tags)
	if err != nil {
		return u.failRun(ctx, run, err)
	}
	run.TaskID = task.ID
	inputPath, err := u.copyAutomationInput(event, taskID)
	if err != nil {
		return u.failRun(ctx, run, err)
	}
	if inputPath != "" {
		task.Body = appendAutomationInput(task.Body, inputPath)
		if err := u.Tasks.Tasks.Update(ctx, *task); err != nil {
			return u.failRun(ctx, run, err)
		}
	}
	if err := u.Runs.Save(ctx, run); err != nil {
		return u.failRun(ctx, run, err)
	}

	prompt, err := u.Tasks.RenderPromptWithVars(ctx, def.Prompt, task, map[string]string{
		"trigger_id":          def.ID,
		"trigger_source_path": filepath.ToSlash(inputPath),
		"automation_run_id":   run.ID,
	})
	if err != nil {
		return u.failRun(ctx, run, err)
	}
	if u.Agent == nil {
		return u.failRun(ctx, run, fmt.Errorf("%w: agent dispatcher is unavailable", domain.ErrInvalidArgument))
	}
	agentName, err := u.Agent.Start(ctx, task, def.AgentKind, prompt)
	if err != nil {
		return u.failRun(ctx, run, err)
	}
	run.AgentName = agentName
	run.Status = domain.RunDispatched
	run.FinishedAt = u.now()
	if err := u.Runs.Save(ctx, run); err != nil {
		return nil, err
	}
	return &run, nil
}

func manualEventID(now time.Time, triggerID string) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate manual event id: %w", err)
	}
	return fmt.Sprintf("manual:%s:%s:%s", triggerID, now.UTC().Format(time.RFC3339Nano), hex.EncodeToString(random[:])), nil
}

func (u *AutomationUseCase) copyAutomationInput(event domain.TriggerEvent, taskID string) (string, error) {
	if strings.TrimSpace(event.SourcePath) == "" {
		return "", nil
	}
	relSource := filepath.Clean(filepath.FromSlash(event.SourcePath))
	sourcePath := filepath.Join(u.ProjectPath, relSource)
	rel, err := filepath.Rel(u.ProjectPath, sourcePath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: source path %q", domain.ErrInvalidPath, event.SourcePath)
	}
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: source path %q", domain.ErrInvalidPath, event.SourcePath)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = source.Close() }()
	name := filepath.Base(rel)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "", fmt.Errorf("%w: source path %q", domain.ErrInvalidPath, event.SourcePath)
	}
	inputRel := filepath.ToSlash(filepath.Join("_tasks", taskID, "input", name))
	if err := fsutil.WriteFileAtomicReader(filepath.Join(u.ProjectPath, filepath.FromSlash(inputRel)), source, 0o644); err != nil {
		return "", err
	}
	return inputRel, nil
}

func (u *AutomationUseCase) hasActiveRun(ctx context.Context, triggerID string) (bool, error) {
	runs, err := u.Runs.List(ctx, u.Project, triggerID)
	if err != nil {
		return false, err
	}
	for _, run := range runs {
		if run.Status != domain.RunCreated && run.Status != domain.RunDispatched {
			continue
		}
		if run.TaskID == "" {
			return true, nil
		}
		task, err := u.Tasks.Show(ctx, run.TaskID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return true, nil
			}
			return false, err
		}
		if !task.Archived && task.Status != domain.TaskStatusDone {
			return true, nil
		}
	}
	return false, nil
}

func (u *AutomationUseCase) failRun(ctx context.Context, run domain.AutomationRun, runErr error) (*domain.AutomationRun, error) {
	run.Status = domain.RunFailed
	run.FinishedAt = u.now()
	run.ErrorMessage = runErr.Error()
	if err := u.Runs.Save(ctx, run); err != nil {
		return nil, errors.Join(runErr, err)
	}
	return &run, runErr
}

func (u *AutomationUseCase) now() time.Time {
	if u.Now == nil {
		return time.Now()
	}
	return u.Now()
}

func eventTime(event domain.TriggerEvent, fallback time.Time) time.Time {
	if !event.ScheduledAt.IsZero() {
		return event.ScheduledAt
	}
	if !event.OccurredAt.IsZero() {
		return event.OccurredAt
	}
	return fallback
}

func automationRunID(triggerID, eventID string, when time.Time) string {
	return when.Format("20060102_150405") + "_" + safeID(triggerID) + "_" + shortHash(eventID)
}

func automationTaskID(triggerID, eventID string, when time.Time) string {
	return automationRunID(triggerID, eventID, when)
}

func safeID(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "automation"
	}
	return b.String()
}

func shortHash(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])[:10]
}

func appendAutomationInput(content, sourcePath string) string {
	if strings.TrimSpace(sourcePath) == "" {
		return content
	}
	sourcePath = strings.ReplaceAll(filepath.ToSlash(sourcePath), "`", "")
	return strings.TrimRight(content, "\n") + "\n\n## Automation Input\n\n- Source file: `" + sourcePath + "`\n"
}
