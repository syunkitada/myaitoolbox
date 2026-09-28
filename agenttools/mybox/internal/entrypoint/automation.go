package entrypoint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mybox/internal/application"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	automationinfra "github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/automation"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/config"
)

type automationContext struct {
	app         *App
	definitions *automationinfra.DefinitionRepository
	runs        *automationinfra.RunStore
	useCase     *application.AutomationUseCase
}

type automationAgentDispatcher struct {
	app *App
}

func (d *automationAgentDispatcher) Start(ctx context.Context, task *domain.Task, kind, prompt string) (string, error) {
	_, name, err := startTaskAgent(ctx, d.app, task, kind, prompt)
	return name, err
}

func newAutomationContext(ctx context.Context, project string) (*automationContext, error) {
	app, err := NewApp(ctx, project)
	if err != nil {
		return nil, err
	}
	runs := automationinfra.NewRunStore(config.AutomationStateDir())
	return &automationContext{
		app:         app,
		definitions: automationinfra.NewDefinitionRepository(app.Project.Path),
		runs:        runs,
		useCase:     app.Automation,
	}, nil
}

func newAutomationCommand(project *string) *cobra.Command {
	cmd := &cobra.Command{Use: "automation", Short: "Manage automatic task triggers"}
	cmd.AddCommand(newAutomationValidateCommand(project))
	cmd.AddCommand(newAutomationListCommand(project))
	cmd.AddCommand(newAutomationRunOnceCommand(project))
	cmd.AddCommand(newAutomationDaemonCommand(project))
	cmd.AddCommand(newAutomationRunsCommand(project))
	return cmd
}

func newAutomationValidateCommand(project *string) *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate automatic task trigger definitions",
		RunE: func(cmd *cobra.Command, args []string) error {
			ac, err := newAutomationContext(cmd.Context(), *project)
			if err != nil {
				return err
			}
			defs, loadErr := ac.definitions.List(cmd.Context())
			var errs []error
			if loadErr != nil {
				errs = append(errs, loadErr)
			}
			matched := 0
			for _, def := range defs {
				if id != "" && def.ID != id {
					continue
				}
				matched++
				if def.Enabled && !validHerdrAgentKind(def.AgentKind) {
					errs = append(errs, fmt.Errorf("trigger %q: unsupported agent kind %q", def.ID, def.AgentKind))
					continue
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "valid %s (%s)\n", def.ID, def.Type)
			}
			if id != "" && matched == 0 {
				errs = append(errs, fmt.Errorf("%w: trigger %s", domain.ErrNotFound, id))
			}
			return errors.Join(errs...)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "trigger id")
	return cmd
}

func newAutomationListCommand(project *string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List automatic task triggers",
		RunE: func(cmd *cobra.Command, args []string) error {
			ac, err := newAutomationContext(cmd.Context(), *project)
			if err != nil {
				return err
			}
			defs, loadErr := ac.definitions.List(cmd.Context())
			if jsonOut {
				return errors.Join(loadErr, writeJSON(cmd, automationSummaries(defs)))
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			for _, def := range defs {
				status := "disabled"
				if def.Enabled {
					status = "enabled"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", def.ID, def.Type, status, def.AgentKind)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			return loadErr
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func newAutomationRunOnceCommand(project *string) *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "run-once",
		Short: "Run currently due automatic triggers once",
		RunE: func(cmd *cobra.Command, args []string) error {
			ac, err := newAutomationContext(cmd.Context(), *project)
			if err != nil {
				return err
			}
			release, err := automationinfra.AcquireProjectLock(config.AutomationStateDir(), ac.app.Project.Name)
			if err != nil {
				return err
			}
			defer release()
			return runAutomationOnce(cmd, ac, id, time.Now())
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "trigger id")
	return cmd
}

func newAutomationDaemonCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run automatic task triggers continuously",
		RunE: func(cmd *cobra.Command, args []string) error {
			ac, err := newAutomationContext(cmd.Context(), *project)
			if err != nil {
				return err
			}
			release, err := automationinfra.AcquireProjectLock(config.AutomationStateDir(), ac.app.Project.Name)
			if err != nil {
				return err
			}
			defer release()
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runAutomationDaemon(cmd, ac, ctx)
		},
	}
}

func newAutomationRunsCommand(project *string) *cobra.Command {
	var id string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "List automatic task executions",
		RunE: func(cmd *cobra.Command, args []string) error {
			ac, err := newAutomationContext(cmd.Context(), *project)
			if err != nil {
				return err
			}
			defs, err := ac.definitions.List(cmd.Context())
			if err != nil {
				return err
			}
			var all []domain.AutomationRun
			for _, def := range defs {
				if id != "" && def.ID != id {
					continue
				}
				runs, listErr := ac.runs.List(cmd.Context(), ac.app.Project.Name, def.ID)
				if listErr != nil {
					return listErr
				}
				all = append(all, runs...)
			}
			if jsonOut {
				return writeJSON(cmd, all)
			}
			for _, run := range all {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\n", run.ID, run.TriggerID, run.Status, run.TaskID, run.ErrorMessage)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "trigger id")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

type automationSummary struct {
	ID        string `json:"id"`
	Enabled   bool   `json:"enabled"`
	Type      string `json:"type"`
	AgentKind string `json:"agent_kind"`
	WatchPath string `json:"watch_path,omitempty"`
	Cron      string `json:"cron,omitempty"`
	Timezone  string `json:"timezone,omitempty"`
}

func automationSummaries(defs []domain.TriggerDefinition) []automationSummary {
	out := make([]automationSummary, 0, len(defs))
	for _, def := range defs {
		out = append(out, automationSummary{
			ID: def.ID, Enabled: def.Enabled, Type: string(def.Type), AgentKind: def.AgentKind,
			WatchPath: def.WatchPath, Cron: def.Cron, Timezone: def.Timezone,
		})
	}
	return out
}

func runAutomationOnce(cmd *cobra.Command, ac *automationContext, id string, now time.Time) error {
	defs, loadErr := ac.definitions.List(cmd.Context())
	var errs []error
	if loadErr != nil {
		errs = append(errs, loadErr)
	}
	for _, def := range defs {
		if !def.Enabled || (id != "" && def.ID != id) {
			continue
		}
		if def.Type == domain.TriggerTypeManual {
			if id == "" {
				continue
			}
			run, runErr := ac.useCase.RunManual(cmd.Context(), def)
			if run != nil {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", run.ID, run.Status, run.TaskID)
			}
			if runErr != nil {
				errs = append(errs, fmt.Errorf("trigger %s: %w", def.ID, runErr))
			}
			continue
		}
		events, err := currentAutomationEvents(cmd.Context(), def, now)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, event := range events {
			run, runErr := ac.useCase.RunEvent(cmd.Context(), def, event)
			if run != nil {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", run.ID, run.Status, run.TaskID)
			}
			if runErr != nil {
				errs = append(errs, fmt.Errorf("trigger %s: %w", def.ID, runErr))
			}
		}
	}
	return errors.Join(errs...)
}

func currentAutomationEvents(ctx context.Context, def domain.TriggerDefinition, now time.Time) ([]domain.TriggerEvent, error) {
	switch def.Type {
	case domain.TriggerTypeCron:
		minute := now.Truncate(time.Minute)
		return automationinfra.CronEventsBetween(ctx, def, minute.Add(-time.Second), now)
	case domain.TriggerTypeFileCreated:
		return automationinfra.ScanFileEvents(ctx, def, now)
	case domain.TriggerTypeManual:
		return []domain.TriggerEvent{}, nil
	default:
		return nil, fmt.Errorf("%w: trigger type %s", domain.ErrInvalidArgument, def.Type)
	}
}

func runAutomationDaemon(cmd *cobra.Command, ac *automationContext, ctx context.Context) error {
	defs, loadErr := ac.definitions.List(ctx)
	if loadErr != nil {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), loadErr)
	}
	watchCtx, cancelWatch := context.WithCancel(ctx)
	events, err := automationinfra.NewFileWatcher(defs).Events(watchCtx)
	if err != nil {
		cancelWatch()
		return err
	}
	last := time.Now()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			cancelWatch()
			return nil
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			processAutomationEvent(cmd, ac, defs, event)
		case now := <-ticker.C:
			newDefs, reloadErr := ac.definitions.List(ctx)
			if reloadErr != nil {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), reloadErr)
			}
			cancelWatch()
			watchCtx, cancelWatch = context.WithCancel(ctx)
			defs = newDefs
			events, err = automationinfra.NewFileWatcher(defs).Events(watchCtx)
			if err != nil {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), err)
				events = nil
			}
			for _, def := range defs {
				if !def.Enabled || def.Type != domain.TriggerTypeCron {
					continue
				}
				cronEvents, cronErr := automationinfra.CronEventsBetween(ctx, def, last, now)
				if cronErr != nil {
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), cronErr)
					continue
				}
				for _, event := range cronEvents {
					processAutomationEvent(cmd, ac, defs, event)
				}
			}
			last = now
		}
	}
}

func processAutomationEvent(cmd *cobra.Command, ac *automationContext, defs []domain.TriggerDefinition, event domain.TriggerEvent) {
	var def *domain.TriggerDefinition
	for i := range defs {
		if defs[i].ID == event.TriggerID {
			def = &defs[i]
			break
		}
	}
	if def == nil || !def.Enabled {
		return
	}
	run, err := ac.useCase.RunEvent(cmd.Context(), *def, event)
	if run != nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", run.ID, run.Status, run.TaskID)
	}
	if err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "trigger %s: %v\n", def.ID, err)
	}
}
