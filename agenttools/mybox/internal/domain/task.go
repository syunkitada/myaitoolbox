package domain

import "time"

// MaxTaskDirectoryNameLength is the maximum number of Unicode characters in a task directory name.
const MaxTaskDirectoryNameLength = 80

type TaskStatus string

const (
	TaskStatusTodo    TaskStatus = "todo"
	TaskStatusDoing   TaskStatus = "doing"
	TaskStatusBlocked TaskStatus = "blocked"
	TaskStatusReview  TaskStatus = "review"
	TaskStatusDone    TaskStatus = "done"
)

type Task struct {
	ID            string
	Title         string
	Status        TaskStatus
	Due           string
	PendingUntil  string
	PendingReason string
	Tags          []string
	Project       string
	Created       time.Time
	Body          string
	Archived      bool
}
