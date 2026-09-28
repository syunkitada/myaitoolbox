package domain

import (
	"context"
	"time"
)

type TriggerType string

const (
	TriggerTypeCron        TriggerType = "cron"
	TriggerTypeFileCreated TriggerType = "file_created"
	TriggerTypeManual      TriggerType = "manual"
)

type OverlapPolicy string

const (
	OverlapSkip  OverlapPolicy = "skip"
	OverlapAllow OverlapPolicy = "allow"
)

type MisfirePolicy string

const (
	MisfireSkip MisfirePolicy = "skip"
)

type RunStatus string

const (
	RunCreated     RunStatus = "created"
	RunDispatched  RunStatus = "dispatched"
	RunFailed      RunStatus = "failed"
	RunSkipped     RunStatus = "skipped"
	RunInterrupted RunStatus = "interrupted"
)

// TriggerDefinition is a validated automation definition rooted in a project.
// Paths are absolute after loading and are never accepted directly from an
// external caller.
type TriggerDefinition struct {
	ID           string
	ProjectRoot  string
	RootPath     string
	Enabled      bool
	Type         TriggerType
	Cron         string
	Timezone     string
	WatchPath    string
	Pattern      string
	SettleFor    time.Duration
	TemplatePath string
	AgentKind    string
	Prompt       string
	Tags         []string
	Overlap      OverlapPolicy
	Misfire      MisfirePolicy
}

// TriggerDefinitionInput contains the project-relative values needed to
// create a trigger definition. The repository validates and persists these
// values together with the rendered task template.
type TriggerDefinitionInput struct {
	ID        string
	Enabled   bool
	Type      TriggerType
	Cron      string
	Timezone  string
	WatchPath string
	Pattern   string
	AgentKind string
	Prompt    string
}

type TriggerEvent struct {
	ID          string
	TriggerID   string
	OccurredAt  time.Time
	ScheduledAt time.Time
	SourcePath  string
}

type AutomationRun struct {
	ID           string
	Project      string
	TriggerID    string
	EventID      string
	Status       RunStatus
	TaskID       string
	AgentName    string
	SourcePath   string
	StartedAt    time.Time
	FinishedAt   time.Time
	ErrorMessage string
}

type TriggerRepository interface {
	List(context.Context) ([]TriggerDefinition, error)
	Find(context.Context, string) (*TriggerDefinition, error)
	Create(context.Context, TriggerDefinitionInput, string) error
}

type AutomationRunStore interface {
	Find(context.Context, string, string, string) (*AutomationRun, error)
	Save(context.Context, AutomationRun) error
	List(context.Context, string, string) ([]AutomationRun, error)
}

// AgentDispatcher starts an agent bound to a generated task and returns the
// agent name. It does not wait for the agent to finish its work.
type AgentDispatcher interface {
	Start(context.Context, *Task, string, string) (string, error)
}
