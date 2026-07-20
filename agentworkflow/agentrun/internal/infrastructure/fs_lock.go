package infrastructure

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/syunkitada/myaitoolbox/agentrun/internal/domain"
	"gopkg.in/yaml.v3"
)

// LockManager implements domain.LockManager using mkdir for atomic locking.
type LockManager struct{}

// NewLockManager creates a LockManager.
func NewLockManager() *LockManager {
	return &LockManager{}
}

func lockDir(taskDir string) string {
	return filepath.Join(taskDir, domain.LockDirName)
}

func lockOwnerPath(taskDir string) string {
	return filepath.Join(lockDir(taskDir), domain.LockOwnerFile)
}

// Acquire attempts to acquire a lock by creating the .lock directory.
func (m *LockManager) Acquire(taskDir string, info *domain.LockInfo) error {
	dir := lockDir(taskDir)
	if err := os.Mkdir(dir, 0755); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("lock already held")
		}
		return fmt.Errorf("failed to create lock: %w", err)
	}

	data, err := yaml.Marshal(info)
	if err != nil {
		os.Remove(dir)
		return err
	}
	if err := os.WriteFile(lockOwnerPath(taskDir), data, 0644); err != nil {
		os.Remove(dir)
		return err
	}
	return nil
}

// Release removes the lock directory.
func (m *LockManager) Release(taskDir string) error {
	dir := lockDir(taskDir)
	os.Remove(lockOwnerPath(taskDir))
	return os.Remove(dir)
}

// IsLocked checks if the lock directory exists.
func (m *LockManager) IsLocked(taskDir string) (bool, error) {
	_, err := os.Stat(lockDir(taskDir))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// GetLockInfo reads the lock owner info.
func (m *LockManager) GetLockInfo(taskDir string) (*domain.LockInfo, error) {
	data, err := os.ReadFile(lockOwnerPath(taskDir))
	if err != nil {
		return nil, err
	}
	var info domain.LockInfo
	if err := yaml.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// IsStale checks if a lock is stale: TTL exceeded or recorded PID is dead.
func (m *LockManager) IsStale(taskDir string) (bool, error) {
	info, err := m.GetLockInfo(taskDir)
	if err != nil {
		// Lock dir exists but owner.yaml missing or unparseable -> stale
		return true, nil
	}

	// Check TTL
	acquiredAt, err := time.Parse(time.RFC3339, info.AcquiredAt)
	if err != nil {
		return true, nil
	}
	if time.Since(acquiredAt) > domain.LockTTL {
		return true, nil
	}

	// Check if worker PID is still alive
	if info.WorkerPID > 0 {
		if !isPIDAlive(info.WorkerPID) {
			return true, nil
		}
	}

	return false, nil
}

// ForceRelease forcefully removes a stale lock.
func (m *LockManager) ForceRelease(taskDir string) error {
	dir := lockDir(taskDir)
	os.Remove(lockOwnerPath(taskDir))
	return os.Remove(dir)
}

// CleanupOrphan kills orphaned agent processes if they are still running.
func (m *LockManager) CleanupOrphan(taskDir string) error {
	info, err := m.GetLockInfo(taskDir)
	if err != nil {
		return nil
	}
	if info.WorkerPID > 0 && isPIDAlive(info.WorkerPID) {
		// Kill the entire process group
		return killProcessGroup(info.WorkerPID)
	}
	return nil
}

func isPIDAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func killProcessGroup(pid int) error {
	// Find the process group leader and kill the group
	cmd := exec.Command("kill", "-9", fmt.Sprintf("-%d", pid))
	return cmd.Run()
}
