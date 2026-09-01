package domain

import "time"

type TaskStatus string

const (
	TaskStatusTodo    TaskStatus = "todo"
	TaskStatusDoing   TaskStatus = "doing"
	TaskStatusBlocked TaskStatus = "blocked"
	TaskStatusReview  TaskStatus = "review"
	TaskStatusDone    TaskStatus = "done"
)

type TaskPriority string

const (
	TaskPriorityLow    TaskPriority = "low"
	TaskPriorityMedium TaskPriority = "medium"
	TaskPriorityHigh   TaskPriority = "high"
	TaskPriorityUrgent TaskPriority = "urgent"
)

type TaskType string

const (
	TaskTypeRegular TaskType = "regular"
	TaskTypeAdhoc   TaskType = "adhoc"
)

type Task struct {
	ID            string
	Title         string
	Status        TaskStatus
	Priority      TaskPriority
	Type          TaskType
	Assignee      string
	Due           string
	PendingUntil  string
	PendingReason string
	Tags          []string
	Project       string
	Created       time.Time
	Body          string
	Archived      bool
}
