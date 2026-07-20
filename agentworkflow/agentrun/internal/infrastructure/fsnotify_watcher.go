package infrastructure

import (
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/syunkitada/myaitoolbox/agentrun/internal/domain"
)

// FSWatcher implements domain.Watcher using fsnotify.
type FSWatcher struct {
 watcher *fsnotify.Watcher
}

// NewFSWatcher creates an FSWatcher.
func NewFSWatcher() (*FSWatcher, error) {
 w, err := fsnotify.NewWatcher()
 if err != nil {
  return nil, err
 }
 return &FSWatcher{watcher: w}, nil
}

// Watch starts watching the given directory and returns a channel of events.
func (w *FSWatcher) Watch(path string) (<-chan domain.WatchEvent, error) {
 if err := w.watcher.Add(path); err != nil {
  return nil, err
 }

 out := make(chan domain.WatchEvent, 100)

 go func() {
  defer close(out)
  for event := range w.watcher.Events {
   // Only care about create and write events
   if event.Op&(fsnotify.Create|fsnotify.Write) == 0 {
    continue
   }
   // Skip hidden files and lock directories
   base := filepath.Base(event.Name)
   if len(base) > 0 && base[0] == '.' {
    continue
   }
   var eventType domain.WatchEventType
   if event.Op&fsnotify.Create != 0 {
    eventType = domain.EventCreated
   } else {
    eventType = domain.EventModified
   }
   out <- domain.WatchEvent{
    Path: event.Name,
    Type: eventType,
   }
  }
 }()

 return out, nil
}

// Close stops the watcher.
func (w *FSWatcher) Close() error {
 return w.watcher.Close()
}
