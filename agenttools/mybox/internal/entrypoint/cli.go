package entrypoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mybox/internal/application"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/config"
)

const version = "0.1.0"

func NewRootCommand() *cobra.Command {
	project := ""
	root := &cobra.Command{
		Use:           "mybox",
		Short:         "Markdown-based personal workspace",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&project, "project", "", "project name (default: default_project)")

	root.AddCommand(newVersionCommand())
	root.AddCommand(newProjectCommand())
	root.AddCommand(newTaskCommand(&project))
	root.AddCommand(newFilesCommand(&project))
	root.AddCommand(newServeCommand())
	return root
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "mybox "+version)
			return nil
		},
	}
}

func newProjectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects",
	}
	cmd.AddCommand(newProjectListCommand())
	cmd.AddCommand(newProjectAddCommand())
	cmd.AddCommand(newProjectRemoveCommand())
	return cmd
}

func newProjectListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, err := NewProjectApp().List(cmd.Context())
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			for _, p := range projects {
				_, _ = fmt.Fprintf(w, "%s\t%s\n", p.Name, p.Path)
			}
			return w.Flush()
		},
	}
}

func newProjectAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add <path>",
		Short: "Register a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := NewProjectApp().Add(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "added project %s (%s)\n", project.Name, project.Path)
			return nil
		},
	}
}

func newProjectRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := NewProjectApp().Remove(cmd.Context(), args[0]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "removed project %s\n", args[0])
			return nil
		},
	}
}

func newTaskCommand(project *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks",
	}
	cmd.AddCommand(newTaskListCommand(project))
	cmd.AddCommand(newTaskShowCommand(project))
	cmd.AddCommand(newTaskCreateCommand(project))
	cmd.AddCommand(newTaskEditCommand(project))
	cmd.AddCommand(newTaskSetCommand(project))
	cmd.AddCommand(newTaskArchiveCommand(project))
	return cmd
}

func newTaskListCommand(project *string) *cobra.Command {
	var all, jsonOut bool
	var status, tag, assignee, taskType string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			tasks, err := app.Tasks.List(cmd.Context(), application.TaskFilter{
				All: all, Status: status, Tag: tag, Assignee: assignee, Type: taskType,
			})
			if err != nil {
				return err
			}
			return printTasks(cmd, tasks, jsonOut)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "include archived tasks")
	cmd.Flags().StringVar(&status, "status", "", "filter by status")
	cmd.Flags().StringVar(&tag, "tag", "", "filter by tag")
	cmd.Flags().StringVar(&assignee, "assignee", "", "filter by assignee")
	cmd.Flags().StringVar(&taskType, "type", "", "filter by type (regular|adhoc)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func newTaskShowCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "show <task-id>",
		Short: "Show task details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			task, err := app.Tasks.Show(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printTaskDetail(cmd, task)
		},
	}
}

func newTaskCreateCommand(project *string) *cobra.Command {
	var name, description, agentKind, prompt string
	var adhoc, jsonOut, noAgent bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a task",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			taskType := ""
			if adhoc {
				taskType = string(domain.TaskTypeAdhoc)
			}
			agentKind = strings.TrimSpace(agentKind)
			if agentKind != "" && !validHerdrAgentKind(agentKind) {
				return fmt.Errorf("unsupported agent kind %q", agentKind)
			}
			task, err := app.Tasks.Create(cmd.Context(), application.TaskInput{
				Name:        name,
				Description: description,
				AgentKind:   agentKind,
				Type:        taskType,
			})
			if err != nil {
				return err
			}
			kind := agentKind
			if kind == "" {
				kind = task.AgentKind
			}
			if kind != "" && !validHerdrAgentKind(kind) {
				return fmt.Errorf("unsupported agent kind %q", kind)
			}
			started := false
			var sentPrompt string
			if !noAgent && kind != "" {
				text, _, err := startTaskAgent(cmd.Context(), app, task, kind, prompt)
				if err != nil {
					return err
				}
				started = true
				sentPrompt = text
			}
			if jsonOut {
				return writeJSON(cmd, task)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), task.ID)
			if started {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "started agent (kind=%s) bound to %s\n", kind, app.Tasks.RelativePathFor(task))
				if sentPrompt != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "prompt sent: %s\n", sentPrompt)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "task name")
	cmd.Flags().StringVar(&description, "description", "", "task description")
	cmd.Flags().StringVar(&agentKind, "agent-kind", "", "agent kind to launch for this task (e.g. opencode)")
	cmd.Flags().StringVar(&prompt, "prompt", "", "prompt template name or inline prompt sent to the agent")
	cmd.Flags().BoolVar(&adhoc, "adhoc", false, "create an adhoc task")
	cmd.Flags().BoolVar(&noAgent, "no-agent", false, "do not start a herdr agent")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

// startTaskAgent runs the herdr orchestration for a freshly created task: start
// the agent bound to the task file (kind) and, when prompt is non-empty, render
// and submit the prompt. It reuses the server-side file-agent logic via a
// lightweight Server whose herdr runner talks to the herdr CLI directly
// (mirroring the StartTaskAgent API endpoint). It returns the submitted prompt
// ("" when none) and the agent name.
func startTaskAgent(ctx context.Context, app *App, task *domain.Task, kind string, prompt string) (string, string, error) {
	srv := &Server{herdrRun: defaultHerdrRun}
	relPath := app.Tasks.RelativePathFor(task)
	agent, err := srv.startHerdrFileAgent(ctx, app, relPath, kind)
	if err != nil {
		return "", "", err
	}
	sent := ""
	if strings.TrimSpace(prompt) != "" {
		text, err := app.Tasks.RenderPrompt(ctx, prompt, task)
		if err != nil {
			return "", "", err
		}
		if strings.TrimSpace(text) != "" {
			if _, err := srv.runHerdr(ctx, "agent", "prompt", agent.Name, text); err != nil {
				return "", "", err
			}
			sent = text
		}
	}
	return sent, agent.Name, nil
}

func newTaskEditCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <task-id>",
		Short: "Edit task in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			path, err := taskFilePath(app, args[0])
			if err != nil {
				return err
			}
			return editFile(cmd, path)
		},
	}
}

func newTaskSetCommand(project *string) *cobra.Command {
	var status, priority, assignee, due string
	var tags string
	cmd := &cobra.Command{
		Use:   "set <task-id>",
		Short: "Update task fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			input := application.TaskInput{Status: status, Priority: priority, Assignee: assignee, Due: due}
			if tags != "" {
				input.Tags = splitTags(tags)
			}
			task, err := app.Tasks.Update(cmd.Context(), args[0], input)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "updated %s (status=%s priority=%s)\n", task.ID, task.Status, task.Priority)
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "status (todo|doing|blocked|review|done)")
	cmd.Flags().StringVar(&priority, "priority", "", "priority (low|medium|high|urgent)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "assignee")
	cmd.Flags().StringVar(&due, "due", "", "due date")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated tags")
	return cmd
}

func newTaskArchiveCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "archive <task-id>",
		Short: "Archive a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			if err := app.Tasks.Archive(cmd.Context(), args[0]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "archived %s\n", args[0])
			return nil
		},
	}
}

func newFilesCommand(project *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "files",
		Short: "Manage files (project root)",
	}
	cmd.AddCommand(newFilesListCommand(project))
	cmd.AddCommand(newFilesShowCommand(project))
	cmd.AddCommand(newFilesCreateCommand(project))
	cmd.AddCommand(newFilesMkdirCommand(project))
	cmd.AddCommand(newFilesEditCommand(project))
	cmd.AddCommand(newFilesMoveCommand(project))
	cmd.AddCommand(newFilesCopyCommand(project))
	cmd.AddCommand(newFilesRenameCommand(project))
	cmd.AddCommand(newFilesDeleteCommand(project))
	return cmd
}

func newFilesListCommand(project *string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list [path]",
		Short: "List files from the project root",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			entries, err := app.Files.Tree(cmd.Context())
			if err != nil {
				return err
			}
			if len(args) == 1 {
				prefix := strings.TrimSuffix(args[0], "/")
				var filtered []domain.FileEntry
				for _, e := range entries {
					if e.Path == prefix || strings.HasPrefix(e.Path, prefix+"/") {
						filtered = append(filtered, e)
					}
				}
				entries = filtered
			}
			return printFileEntries(cmd, entries, jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func newFilesShowCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "show <path>",
		Short: "Show file content",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			content, err := app.Files.Content(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			_, _ = fmt.Fprint(cmd.OutOrStdout(), content)
			if !strings.HasSuffix(content, "\n") {
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
}

func newFilesCreateCommand(project *string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "create <path>",
		Short: "Create an empty file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			if err := app.Files.Create(cmd.Context(), args[0]); err != nil {
				return err
			}
			if jsonOut {
				return writeJSON(cmd, map[string]string{"path": args[0]})
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func newFilesMkdirCommand(project *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mkdir <path>",
		Short: "Create a directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			if err := app.Files.CreateDir(cmd.Context(), args[0]); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), args[0])
			return nil
		},
	}
	return cmd
}

func newFilesEditCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <path>",
		Short: "Edit file in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			return editFile(cmd, filepath.Join(app.Project.Path, filepath.FromSlash(args[0])))
		},
	}
}

func newFilesMoveCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "move <old> <new>",
		Short: "Move a file or directory",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			if err := app.Files.Move(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "moved %s -> %s\n", args[0], args[1])
			return nil
		},
	}
}

func newFilesCopyCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "copy <old> <new>",
		Short: "Copy a file or directory",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			if err := app.Files.Copy(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "copied %s -> %s\n", args[0], args[1])
			return nil
		},
	}
}

func newFilesRenameCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old> <new-name>",
		Short: "Rename a file or directory",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			if args[1] == "" || strings.ContainsAny(args[1], `/\`) || strings.Contains(args[1], "..") {
				return fmt.Errorf("%w: %q", domain.ErrInvalidPath, args[1])
			}
			newPath := filepath.ToSlash(filepath.Join(filepath.Dir(args[0]), args[1]))
			if err := app.Files.Move(cmd.Context(), args[0], newPath); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "renamed %s -> %s\n", args[0], newPath)
			return nil
		},
	}
}

func newFilesDeleteCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <path>",
		Short: "Delete a file or directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			if err := app.Files.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", args[0])
			return nil
		},
	}
}

func newServeCommand() *cobra.Command {
	var host string
	var port int
	var noBrowser, readOnly bool
	var basePath string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the web UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Flags().GetString("project")
			store := config.NewStore()
			cfg, err := store.Load(cmd.Context())
			if err != nil {
				return err
			}
			server := NewServer(cfg, project, readOnly, basePath)
			basePath = normalizeBasePath(basePath)
			addr := net.JoinHostPort(host, strconv.Itoa(port))
			url := "http://" + addr + basePath + "/"
			if !noBrowser {
				go openBrowser(url)
			}
			dispProject := project
			if dispProject == "" {
				dispProject = cfg.DefaultProject
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "mybox web UI: %s (default project: %s)\n", url, dispProject)
			srv := &http.Server{Addr: addr, Handler: server.Handler()}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			go func() {
				<-ctx.Done()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = srv.Shutdown(shutdownCtx)
			}()
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "bind host")
	cmd.Flags().IntVar(&port, "port", 8080, "bind port")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "do not open the browser")
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "disallow writes")
	cmd.Flags().StringVar(&basePath, "base-path", "", "serve the app under this URL path prefix (e.g. /mybox)")
	return cmd
}

func openBrowser(url string) {
	for _, name := range []string{"xdg-open", "open", "cmd"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		if name == "cmd" {
			_ = exec.Command(path, "/c", "start", url).Run()
			return
		}
		_ = exec.Command(path, url).Run()
		return
	}
}

func taskFilePath(app *App, id string) (string, error) {
	active := filepath.Join(app.Project.Path, "tasks", id, "task.md")
	if _, err := os.Stat(active); err == nil {
		return active, nil
	}
	adhoc := filepath.Join(app.Project.Path, "tasks", "adhoc", id+".md")
	if _, err := os.Stat(adhoc); err == nil {
		return adhoc, nil
	}
	archived := filepath.Join(app.Project.Path, "archives", "tasks", id, "task.md")
	if _, err := os.Stat(archived); err == nil {
		return archived, nil
	}
	return "", fmt.Errorf("%w: task %s", domain.ErrNotFound, id)
}

func editFile(cmd *cobra.Command, path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	c := exec.Command("sh", "-c", editor+" "+strconv.Quote(path))
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func splitTags(s string) []string {
	parts := strings.Split(s, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

func printTasks(cmd *cobra.Command, tasks []domain.Task, jsonOut bool) error {
	if jsonOut {
		return writeJSON(cmd, tasks)
	}
	if len(tasks) == 0 {
		return nil
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tSTATUS\tPRIORITY\tTYPE\tASSIGNEE\tDUE\tTITLE")
	for _, t := range tasks {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", t.ID, t.Status, t.Priority, t.Type, t.Assignee, t.Due, t.Title)
	}
	return w.Flush()
}

func printTaskDetail(cmd *cobra.Command, task *domain.Task) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "ID:\t%s\n", task.ID)
	_, _ = fmt.Fprintf(w, "Title:\t%s\n", task.Title)
	_, _ = fmt.Fprintf(w, "Status:\t%s\n", task.Status)
	_, _ = fmt.Fprintf(w, "Priority:\t%s\n", task.Priority)
	_, _ = fmt.Fprintf(w, "Type:\t%s\n", task.Type)
	_, _ = fmt.Fprintf(w, "Assignee:\t%s\n", task.Assignee)
	_, _ = fmt.Fprintf(w, "Due:\t%s\n", task.Due)
	_, _ = fmt.Fprintf(w, "Tags:\t%s\n", strings.Join(task.Tags, ", "))
	_, _ = fmt.Fprintf(w, "Project:\t%s\n", task.Project)
	_, _ = fmt.Fprintf(w, "Created:\t%s\n", task.Created.Format("2006-01-02 15:04"))
	if task.Archived {
		_, _ = fmt.Fprintln(w, "Archived:\ttrue")
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if strings.TrimSpace(task.Body) != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", task.Body)
	}
	return nil
}

func printFileEntries(cmd *cobra.Command, entries []domain.FileEntry, jsonOut bool) error {
	if jsonOut {
		return writeJSON(cmd, entries)
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "PATH\tKIND\tSTATUS")
	for _, e := range entries {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", e.Path, e.Kind, e.Status)
	}
	return w.Flush()
}

func writeJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
