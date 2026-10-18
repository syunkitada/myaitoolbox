package config

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"golang.org/x/sys/unix"
)

const defaultBrowserAuditFile = "browser-audit.jsonl"

const (
	defaultBrowserAuditMaxBytes int64 = 10 << 20
	defaultBrowserAuditBackups        = 5
	maxBrowserAuditBackups            = 100
	minBrowserAuditMaxBytes     int64 = maxBrowserAuditLineSize + 1

	defaultBrowserAuditLimit = 50
	maxBrowserAuditLimit     = 100
	maxBrowserAuditOffset    = 10000
	maxBrowserAuditLineSize  = 1 << 20
)

// BrowserAuditStore appends redacted browser security events as JSON lines.
// The file is user-local and is created with owner-only permissions.
type BrowserAuditStore struct {
	path     string
	maxBytes int64
	backups  int
	mu       sync.Mutex
}

func NewBrowserAuditStore() *BrowserAuditStore {
	path := strings.TrimSpace(os.Getenv("MYBOX_BROWSER_AUDIT_LOG"))
	if path == "" {
		path = filepath.Join(filepath.Dir(configPath()), defaultBrowserAuditFile)
	}
	store := NewBrowserAuditStoreAt(path)
	store.maxBytes = browserAuditMaxBytesFromEnvironment()
	store.backups = browserAuditBackupCountFromEnvironment()
	return store
}

// NewBrowserAuditStoreAt is intended for isolated callers and tests.
func NewBrowserAuditStoreAt(path string) *BrowserAuditStore {
	return &BrowserAuditStore{
		path:     path,
		maxBytes: defaultBrowserAuditMaxBytes,
		backups:  defaultBrowserAuditBackups,
	}
}

func (s *BrowserAuditStore) Append(ctx context.Context, event domain.BrowserAuditEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(event.Event) == "" || strings.TrimSpace(event.Project) == "" || strings.TrimSpace(event.Result) == "" {
		return errors.New("browser audit event identity is incomplete")
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = timeNowUTC()
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data)-1 > maxBrowserAuditLineSize {
		return fmt.Errorf("browser audit event exceeds the maximum line size of %d bytes", maxBrowserAuditLineSize)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	lockFile, err := lockBrowserAuditFile(ctx, s.path, true)
	if err != nil {
		return err
	}
	defer func() { _ = lockFile.Close() }()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.rotateIfNeeded(int64(len(data))); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	fileInfo, err := f.Stat()
	if err != nil {
		return err
	}
	pathInfo, err := os.Lstat(s.path)
	if err != nil {
		return err
	}
	if !pathInfo.Mode().IsRegular() || !os.SameFile(pathInfo, fileInfo) {
		return fmt.Errorf("browser audit path is not a regular file: %s", s.path)
	}
	if err := f.Chmod(0o600); err != nil {
		return err
	}
	n, err := f.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return fmt.Errorf("short write to browser audit log: wrote %d of %d bytes", n, len(data))
	}
	return f.Sync()
}

func (s *BrowserAuditStore) rotateIfNeeded(nextRecordSize int64) error {
	if s.maxBytes <= 0 {
		return nil
	}
	info, err := os.Lstat(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return s.pruneBrowserAuditBackups()
		}
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("browser audit path is not a regular file: %s", s.path)
	}
	if err := s.pruneBrowserAuditBackups(); err != nil {
		return err
	}
	if err := chmodBrowserAuditFile(s.path); err != nil {
		return err
	}
	if info.Size() <= s.maxBytes-nextRecordSize {
		return nil
	}

	for index := 1; index <= s.backups; index++ {
		backupPath := browserAuditBackupPath(s.path, index)
		backupInfo, statErr := os.Lstat(backupPath)
		if statErr == nil {
			if !backupInfo.Mode().IsRegular() {
				return fmt.Errorf("browser audit backup is not a regular file: %s", backupPath)
			}
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
	}

	if s.backups == 0 {
		return os.Remove(s.path)
	}
	if err := removeBrowserAuditBackup(browserAuditBackupPath(s.path, s.backups)); err != nil {
		return err
	}
	for index := s.backups - 1; index >= 1; index-- {
		from := browserAuditBackupPath(s.path, index)
		to := browserAuditBackupPath(s.path, index+1)
		if _, err := os.Lstat(from); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := chmodBrowserAuditFile(from); err != nil {
			return err
		}
		if err := os.Rename(from, to); err != nil {
			return err
		}
	}
	return os.Rename(s.path, browserAuditBackupPath(s.path, 1))
}

func (s *BrowserAuditStore) pruneBrowserAuditBackups() error {
	for index := s.backups + 1; index <= maxBrowserAuditBackups; index++ {
		path := browserAuditBackupPath(s.path, index)
		info, err := os.Lstat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("browser audit backup is not a regular file: %s", path)
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

func removeBrowserAuditBackup(path string) error {
	if _, err := os.Lstat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.Remove(path)
}

func chmodBrowserAuditFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	fileInfo, err := f.Stat()
	if err != nil {
		return err
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !pathInfo.Mode().IsRegular() || !os.SameFile(pathInfo, fileInfo) {
		return fmt.Errorf("browser audit path is not a regular file: %s", path)
	}
	return f.Chmod(0o600)
}

func browserAuditBackupPath(path string, index int) string {
	return path + "." + strconv.Itoa(index)
}

func lockBrowserAuditFile(ctx context.Context, path string, exclusive bool) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	lockPath := path + ".lock"
	fd, err := unix.Open(lockPath, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open browser audit lock: %w", err)
	}
	lockFile := os.NewFile(uintptr(fd), lockPath)
	if lockFile == nil {
		_ = unix.Close(fd)
		return nil, errors.New("open browser audit lock: invalid file descriptor")
	}
	closeOnError := func(err error) (*os.File, error) {
		_ = lockFile.Close()
		return nil, err
	}
	fileInfo, err := lockFile.Stat()
	if err != nil {
		return closeOnError(err)
	}
	pathInfo, err := os.Lstat(lockPath)
	if err != nil {
		return closeOnError(err)
	}
	if !fileInfo.Mode().IsRegular() || !pathInfo.Mode().IsRegular() || !os.SameFile(fileInfo, pathInfo) {
		return closeOnError(fmt.Errorf("browser audit lock is not a regular file: %s", lockPath))
	}
	if err := lockFile.Chmod(0o600); err != nil {
		return closeOnError(err)
	}
	lockMode := unix.LOCK_SH
	if exclusive {
		lockMode = unix.LOCK_EX
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return closeOnError(err)
		}
		err := unix.Flock(fd, lockMode|unix.LOCK_NB)
		if err == nil {
			return lockFile, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			return closeOnError(fmt.Errorf("lock browser audit file: %w", err))
		}
		select {
		case <-ctx.Done():
			return closeOnError(ctx.Err())
		case <-ticker.C:
		}
	}
}

func browserAuditMaxBytesFromEnvironment() int64 {
	value := strings.TrimSpace(os.Getenv("MYBOX_BROWSER_AUDIT_MAX_BYTES"))
	if value == "" {
		return defaultBrowserAuditMaxBytes
	}
	maxBytes, err := strconv.ParseInt(value, 10, 64)
	if err != nil || maxBytes < 0 || (maxBytes > 0 && maxBytes < minBrowserAuditMaxBytes) {
		return defaultBrowserAuditMaxBytes
	}
	return maxBytes
}

func browserAuditBackupCountFromEnvironment() int {
	value := strings.TrimSpace(os.Getenv("MYBOX_BROWSER_AUDIT_BACKUPS"))
	if value == "" {
		return defaultBrowserAuditBackups
	}
	backups, err := strconv.Atoi(value)
	if err != nil || backups < 0 || backups > maxBrowserAuditBackups {
		return defaultBrowserAuditBackups
	}
	return backups
}

func (s *BrowserAuditStore) List(ctx context.Context, query domain.BrowserAuditQuery) (domain.BrowserAuditPage, error) {
	if err := ctx.Err(); err != nil {
		return domain.BrowserAuditPage{}, err
	}
	project := strings.TrimSpace(query.Project)
	if project == "" {
		return domain.BrowserAuditPage{}, errors.New("browser audit project is required")
	}
	if query.Offset < 0 {
		return domain.BrowserAuditPage{}, errors.New("browser audit offset must not be negative")
	}
	if query.Offset > maxBrowserAuditOffset {
		return domain.BrowserAuditPage{}, errors.New("browser audit offset is too large")
	}
	limit := query.Limit
	if limit <= 0 {
		limit = defaultBrowserAuditLimit
	}
	if limit > maxBrowserAuditLimit {
		limit = maxBrowserAuditLimit
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	lockFile, err := lockBrowserAuditFile(ctx, s.path, false)
	if err != nil {
		return domain.BrowserAuditPage{}, err
	}
	defer func() { _ = lockFile.Close() }()
	if err := ctx.Err(); err != nil {
		return domain.BrowserAuditPage{}, err
	}
	windowSize := query.Offset + limit
	matches := make([]domain.BrowserAuditEvent, 0, windowSize)
	total := 0
	paths := make([]string, 0, s.backups+1)
	for index := s.backups; index >= 1; index-- {
		paths = append(paths, browserAuditBackupPath(s.path, index))
	}
	paths = append(paths, s.path)
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return domain.BrowserAuditPage{}, err
		}
		pathInfo, statErr := os.Lstat(path)
		fileInfo, fileStatErr := f.Stat()
		if statErr != nil || fileStatErr != nil || !pathInfo.Mode().IsRegular() || !os.SameFile(pathInfo, fileInfo) {
			_ = f.Close()
			if statErr != nil {
				return domain.BrowserAuditPage{}, statErr
			}
			if fileStatErr != nil {
				return domain.BrowserAuditPage{}, fileStatErr
			}
			return domain.BrowserAuditPage{}, fmt.Errorf("browser audit path is not a regular file: %s", path)
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 4096), maxBrowserAuditLineSize)
		lineNumber := 0
		for scanner.Scan() {
			lineNumber++
			if err := ctx.Err(); err != nil {
				_ = f.Close()
				return domain.BrowserAuditPage{}, err
			}
			if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
				continue
			}
			var event domain.BrowserAuditEvent
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				_ = f.Close()
				return domain.BrowserAuditPage{}, fmt.Errorf("invalid browser audit log %s line %d: %w", filepath.Base(path), lineNumber, err)
			}
			if event.Project != project || !browserAuditEventMatches(event, query) {
				continue
			}
			total++
			if len(matches) < windowSize {
				matches = append(matches, event)
			} else {
				matches = append(matches[1:], event)
			}
		}
		scanErr := scanner.Err()
		closeErr := f.Close()
		if scanErr != nil {
			return domain.BrowserAuditPage{}, scanErr
		}
		if closeErr != nil {
			return domain.BrowserAuditPage{}, closeErr
		}
	}
	end := len(matches) - query.Offset
	if end < 0 {
		end = 0
	}
	start := end - limit
	if start < 0 {
		start = 0
	}
	events := make([]domain.BrowserAuditEvent, 0, end-start)
	for index := end - 1; index >= start; index-- {
		events = append(events, matches[index])
	}
	return domain.BrowserAuditPage{
		Events:  events,
		Total:   total,
		Offset:  query.Offset,
		Limit:   limit,
		HasMore: total > query.Offset+limit,
	}, nil
}

func browserAuditEventMatches(event domain.BrowserAuditEvent, query domain.BrowserAuditQuery) bool {
	if query.SessionID != "" && event.SessionID != query.SessionID {
		return false
	}
	if query.Event != "" && event.Event != query.Event {
		return false
	}
	if query.Actor != "" && event.Actor != query.Actor {
		return false
	}
	if query.Command != "" && event.Command != query.Command {
		return false
	}
	if query.Result != "" && event.Result != query.Result {
		return false
	}
	if !query.From.IsZero() && event.Timestamp.Before(query.From) {
		return false
	}
	if !query.To.IsZero() && event.Timestamp.After(query.To) {
		return false
	}
	return true
}

var timeNowUTC = func() time.Time { return time.Now().UTC() }
