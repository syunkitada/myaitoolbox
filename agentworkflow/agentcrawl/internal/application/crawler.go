package application

import (
	"log/slog"
	"path/filepath"

	"github.com/syunkitada/myaitoolbox/agentcrawl/internal/domain"
)

// EventReader reads events from a source directory.
type EventReader interface {
	ReadEvents() ([]*domain.Event, error)
}

// TaskWriter creates task directories from events.
type TaskWriter interface {
	WriteTask(taskID string, event *domain.Event, title string) error
}

// Crawler detects events and generates tasks.
type Crawler struct {
	eventReader EventReader
	taskWriter  TaskWriter
	source      string
	logger      *slog.Logger
}

// NewCrawler creates a Crawler with the given dependencies.
func NewCrawler(reader EventReader, writer TaskWriter, logger *slog.Logger) *Crawler {
	return &Crawler{
		eventReader: reader,
		taskWriter:  writer,
		source:      "file",
		logger:      logger,
	}
}

// Run executes a single crawl cycle: reads events and generates tasks.
func (c *Crawler) Run() error {
	events, err := c.eventReader.ReadEvents()
	if err != nil {
		return err
	}

	if len(events) == 0 {
		c.logger.Info("no events found")
		return nil
	}

	for _, event := range events {
		taskID := domain.GenerateTaskID(c.source, event.Data)
		title := eventToTitle(event)

		c.logger.Info("creating task",
			"task_id", taskID,
			"event", event.Name,
			"title", title,
		)

		if err := c.taskWriter.WriteTask(taskID, event, title); err != nil {
			c.logger.Error("failed to create task",
				"event", event.Name,
				"error", err,
			)
			continue
		}

		c.logger.Info("task created", "task_id", taskID)
	}

	return nil
}

// eventToTitle derives a task title from the event filename.
func eventToTitle(event *domain.Event) string {
	name := event.Name
	ext := filepath.Ext(name)
	if ext != "" {
		name = name[:len(name)-len(ext)]
	}
	return "Process " + name
}
