package domain

// TaskStatus represents the high-level progress state of a task (Kanban column).
type TaskStatus string

const (
	StatusInProgress TaskStatus = "inprogress"
	StatusInbox      TaskStatus = "inbox"
	StatusWaiting    TaskStatus = "waiting"
	StatusDone       TaskStatus = "done"
)

// IsValid checks if the status is one of the allowed values.
func (s TaskStatus) IsValid() bool {
	switch s {
	case StatusInbox, StatusInProgress, StatusWaiting, StatusDone:
		return true
	}
	return false
}
