package domain

// TaskInfo contains the essential information to process a task.
type TaskInfo struct {
	TaskDir  string
	TaskID   string
	Metadata *Metadata
}

// AgentSpawnerResult contains the result of an agent subprocess execution.
type AgentSpawnerResult struct {
	ExitCode int
	Output   string
	Stderr   string
}

// HistoryEntry represents a single history record to write.
type HistoryEntry struct {
	Sequence  int
	Type      string // created, started, finished, handoff, error
	Content   string
}

// Port interfaces define the capabilities the application layer needs.
// Infrastructure implements these; application depends only on these.

// TaskStore reads and writes task state on the filesystem.
type TaskStore interface {
	// ListTaskDirs returns all task directories under the workspace.
	ListTaskDirs() ([]string, error)

	// ListTaskDirsByStatus returns task directories filtered by status.
	ListTaskDirsByStatus(status TaskStatus) ([]*TaskInfo, error)

	// ReadMetadata reads metadata.yaml for a task.
	ReadMetadata(taskDir string) (*Metadata, error)

	// WriteMetadata writes metadata.yaml for a task.
	WriteMetadata(taskDir string, meta *Metadata) error

	// MoveToArchive moves a completed task directory to archive/.
	MoveToArchive(taskDir string) error

	// EnsureWorkspace creates the workspace directory structure if it doesn't exist.
	EnsureWorkspace() error
}

// LockManager provides atomic locking for tasks.
type LockManager interface {
	// Acquire attempts to acquire the lock for a task.
	Acquire(taskDir string, info *LockInfo) error

	// Release removes the lock for a task.
	Release(taskDir string) error

	// IsLocked checks if a task has an active lock.
	IsLocked(taskDir string) (bool, error)

	// GetLockInfo reads the lock info for a task.
	GetLockInfo(taskDir string) (*LockInfo, error)

	// IsStale checks if a lock is stale (TTL exceeded or PID dead).
	IsStale(taskDir string) (bool, error)

	// ForceRelease forcefully removes a stale lock.
	ForceRelease(taskDir string) error

	// CleanupOrphan kills orphaned agent processes if they are still running.
	CleanupOrphan(taskDir string) error
}

// HistoryRecorder writes history entries for a task.
type HistoryRecorder interface {
	// Record writes a history entry to the task's history/ directory.
	Record(taskDir string, entry *HistoryEntry) error

	// NextSequence returns the next sequence number for a task.
	NextSequence(taskDir string) (int, error)
}

// AgentSpawner executes an agent as a subprocess.
type AgentSpawner interface {
	// Spawn runs the agent and returns the result.
	Spawn(taskInfo *TaskInfo, agentDir string) (*AgentSpawnerResult, error)
}

// Watcher monitors a directory for changes.
type Watcher interface {
	// Watch starts watching and returns a channel of events.
	Watch(path string) (<-chan WatchEvent, error)

	// Close stops the watcher.
	Close() error
}

// WatchEvent represents a filesystem change event.
type WatchEvent struct {
	Path string
	Type WatchEventType
}

// WatchEventType indicates the kind of filesystem event.
type WatchEventType int

const (
	EventCreated WatchEventType = iota
	EventModified
	EventDeleted
)
