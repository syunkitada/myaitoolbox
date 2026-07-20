package domain

import "time"

// LockInfo stores information about who holds the lock on a task.
type LockInfo struct {
	DaemonPID  int    `yaml:"daemon_pid"`
	WorkerPID  int    `yaml:"worker_pid"`
	Hostname   string `yaml:"hostname"`
	AcquiredAt string `yaml:"acquired_at"`
}

// NewLockInfo creates a LockInfo with the current time.
func NewLockInfo(daemonPID, workerPID int, hostname string) *LockInfo {
	return &LockInfo{
		DaemonPID:  daemonPID,
		WorkerPID:  workerPID,
		Hostname:   hostname,
		AcquiredAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// LockTTL is the maximum duration a lock is considered valid.
// It should be set to AgentTimeout * (MaxRetry + 1).
// Default: 10 min * 4 = 40 min.
const LockTTL = 40 * time.Minute

// LockDirName is the name of the lock directory inside a task.
const LockDirName = ".lock"

// LockOwnerFile is the name of the lock info file.
const LockOwnerFile = "owner.yaml"
