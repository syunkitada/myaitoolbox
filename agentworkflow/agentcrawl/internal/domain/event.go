package domain

import (
	"fmt"
	"os"
)

// Event represents a raw event file read from the event source directory.
type Event struct {
	// Path is the absolute path to the event file.
	Path string

	// Name is the base filename (e.g., "alert.json").
	Name string

	// Data is the raw file content.
	Data []byte
}

// ReadEvent reads an event file from disk.
func ReadEvent(path string) (*Event, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &Event{
		Path: path,
		Name: info.Name(),
		Data: data,
	}, nil
}
