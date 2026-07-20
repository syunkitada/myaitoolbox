package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/agentcrawl/internal/application"
	"github.com/syunkitada/myaitoolbox/agentcrawl/internal/infrastructure"
)

var (
	flagDir    string
	flagWatch  bool
	flagPretty bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "agentcrawl",
		Short: "Detect events and generate tasks for the agent workflow system",
		RunE:  run,
	}

	rootCmd.Flags().StringVar(&flagDir, "dir", "", "Event source directory (required)")
	rootCmd.Flags().BoolVar(&flagWatch, "watch", false, "Run in daemon mode with inotify watch")
	rootCmd.Flags().BoolVar(&flagPretty, "pretty", false, "Pretty-print log output")

	_ = rootCmd.MarkFlagRequired("dir")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Logger setup
	logLevel := slog.LevelInfo
	opts := &slog.HandlerOptions{Level: logLevel}
	var handler slog.Handler
	if flagPretty {
		handler = slog.NewTextHandler(os.Stderr, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}
	logger := slog.New(handler)

	// Determine workspace root from --dir's parent or env
	workspaceRoot := resolveWorkspaceRoot(flagDir)
	logger.Info("starting agentcrawl",
		"dir", flagDir,
		"workspace_root", workspaceRoot,
		"watch", flagWatch,
	)

	// Build dependencies
	reader := infrastructure.NewEventReader(flagDir)
	writer := infrastructure.NewTaskWriter(workspaceRoot)
	crawler := application.NewCrawler(reader, writer, logger)

	if flagWatch {
		return runDaemon(crawler, flagDir, logger)
	}
	return runOneshot(crawler, logger)
}

func runOneshot(crawler *application.Crawler, logger *slog.Logger) error {
	logger.Info("running in oneshot mode")
	if err := crawler.Run(); err != nil {
		logger.Error("crawl failed", "error", err)
		return err
	}
	logger.Info("oneshot complete")
	return nil
}

func runDaemon(crawler *application.Crawler, dir string, logger *slog.Logger) error {
	logger.Info("running in daemon mode")

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Create watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	// Add the directory to watch
	if err := watcher.Add(dir); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	logger.Info("watching for events", "dir", dir)

	// Debounce timer to batch rapid file changes
	var debounceTimer *time.Timer

	for {
		select {
		case <-ctx.Done():
			logger.Info("daemon shutting down")
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return fmt.Errorf("watcher channel closed")
			}
			// Only care about new files
			if event.Op&fsnotify.Create == 0 {
				continue
			}
			// Skip hidden files
			base := filepath.Base(event.Name)
			if len(base) > 0 && base[0] == '.' {
				continue
			}
			logger.Info("new event detected", "file", event.Name)

			// Debounce: wait a short time for more files to arrive
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
				if err := crawler.Run(); err != nil {
					logger.Error("crawl cycle failed", "error", err)
				}
			})
		case err, ok := <-watcher.Errors:
			if !ok {
				return fmt.Errorf("watcher error channel closed")
			}
			logger.Error("watcher error", "error", err)
		}
	}
}

// resolveWorkspaceRoot determines the workspace root from the event directory.
// Convention: events are in <workspace>/events/incoming or similar.
// For simplicity, we use WORKSPACE_ROOT env var or go two levels up from --dir.
func resolveWorkspaceRoot(eventDir string) string {
	if ws := os.Getenv("WORKSPACE_ROOT"); ws != "" {
		return ws
	}
	// Default: assume eventDir is under <workspace>/events/*
	return filepath.Dir(filepath.Dir(eventDir))
}
