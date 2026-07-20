package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/syunkitada/myaitoolbox/agentrun/internal/domain"
)

// HistoryRecorder implements domain.HistoryRecorder using the filesystem.
type HistoryRecorder struct{}

// NewHistoryRecorder creates a HistoryRecorder.
func NewHistoryRecorder() *HistoryRecorder {
	return &HistoryRecorder{}
}

// Record writes a history entry to the task's history/ directory.
func (r *HistoryRecorder) Record(taskDir string, entry *domain.HistoryEntry) error {
	historyDir := filepath.Join(taskDir, "history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return err
	}

	seq, err := r.NextSequence(taskDir)
	if err != nil {
		return err
	}
	entry.Sequence = seq

	filename := fmt.Sprintf("%04d-%s.md", seq, entry.Type)
	path := filepath.Join(historyDir, filename)

	content := fmt.Sprintf("# %s\n\nTimestamp: %s\n\n%s\n",
		strings.Title(entry.Type),
		domain.NowISO(),
		entry.Content,
	)

	return os.WriteFile(path, []byte(content), 0644)
}

// NextSequence returns the next sequence number based on existing history files.
func (r *HistoryRecorder) NextSequence(taskDir string) (int, error) {
	historyDir := filepath.Join(taskDir, "history")
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}

	maxSeq := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		parts := strings.SplitN(name, "-", 2)
		if len(parts) < 2 {
			continue
		}
		seq, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		if seq > maxSeq {
			maxSeq = seq
		}
	}
	return maxSeq + 1, nil
}

// ListHistoryFiles returns all history files sorted by sequence.
func (r *HistoryRecorder) ListHistoryFiles(taskDir string) ([]string, error) {
	historyDir := filepath.Join(taskDir, "history")
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}
