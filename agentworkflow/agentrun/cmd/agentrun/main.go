package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/agentrun/internal/application"
	"github.com/syunkitada/myaitoolbox/agentrun/internal/infrastructure"
)

var (
	flagDir      string
	flagWatch    bool
	flagPretty   bool
	flagTimeout  time.Duration
	flagAgentsDir string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "agentrun",
		Short: "Agent workflow orchestrator - manages task execution via subprocess agents",
		RunE:  run,
	}

	rootCmd.Flags().StringVar(&flagDir, "dir", "", "Workspace root directory (required)")
	rootCmd.Flags().BoolVar(&flagWatch, "watch", false, "Run in daemon mode with inotify watch")
	rootCmd.Flags().BoolVar(&flagPretty, "pretty", false, "Pretty-print log output")
	rootCmd.Flags().DurationVar(&flagTimeout, "timeout", 10*time.Minute, "Agent subprocess timeout")
	rootCmd.Flags().StringVar(&flagAgentsDir, "agents-dir", "agents", "Agents directory relative to workspace")

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

	logger.Info("starting agentrun",
		"dir", flagDir,
		"watch", flagWatch,
		"timeout", flagTimeout,
	)

	// Build dependencies
	workspaceRoot := flagDir
	taskStore := infrastructure.NewTaskStore(workspaceRoot)
	lockManager := infrastructure.NewLockManager()
	history := infrastructure.NewHistoryRecorder()
	spawner := infrastructure.NewProcessSpawner(workspaceRoot, flagTimeout)

	detector := application.NewDetector(taskStore, lockManager, history, logger)
	handoffProc := application.NewHandoffProcessor(taskStore, history, logger)

	runner := application.NewRunner(
		taskStore,
		lockManager,
		history,
		spawner,
		detector,
		handoffProc,
		flagAgentsDir,
		logger,
	)

	if flagWatch {
		return runDaemon(runner, logger)
	}
	return runOneshot(runner, logger)
}

func runOneshot(runner *application.Runner, logger *slog.Logger) error {
	logger.Info("running in oneshot mode")
	if err := runner.RunOneshot(); err != nil {
		logger.Error("oneshot failed", "error", err)
		return err
	}
	logger.Info("oneshot complete")
	return nil
}

func runDaemon(runner *application.Runner, logger *slog.Logger) error {
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
	watcher, err := infrastructure.NewFSWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	return runner.RunDaemon(ctx, watcher)
}
