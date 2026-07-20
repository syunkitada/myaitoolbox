package infrastructure

import (
	"os"
	"path/filepath"

	"github.com/syunkitada/myaitoolbox/agentcrawl/internal/domain"
)

// EventReader reads event files from a directory.
type EventReader struct {
	dir string
}

// NewEventReader creates an EventReader for the given directory.
func NewEventReader(dir string) *EventReader {
	return &EventReader{dir: dir}
}

// ReadEvents lists all non-hidden, non-directory files in the directory
// and returns them as domain.Event slices.
func (r *EventReader) ReadEvents() ([]*domain.Event, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}

	var events []*domain.Event
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) > 0 && name[0] == '.' {
			continue
		}
		path := filepath.Join(r.dir, name)
		event, err := domain.ReadEvent(path)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}
