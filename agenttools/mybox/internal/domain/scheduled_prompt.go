package domain

import (
	"context"
	"time"
)

// ScheduledPrompt is a prompt that should be submitted to a herdr agent at a
// specific time. Project identifies the mybox project whose agent namespace
// contains Target.
type ScheduledPrompt struct {
	ID          string
	Project     string
	Target      string
	Text        string
	ScheduledAt time.Time
}

// ScheduledPromptRepository persists prompts independently from the browser.
// An empty project passed to List returns prompts for every project; this is
// used by the background worker.
type ScheduledPromptRepository interface {
	List(ctx context.Context, project string) ([]ScheduledPrompt, error)
	Create(ctx context.Context, prompt ScheduledPrompt) error
	Delete(ctx context.Context, project, id string) error
}
