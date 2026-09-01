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
	root.AddCommand(newSearchCommand(&project))
	root.AddCommand(newTaskCommand(&project))
	root.AddCommand(newKnowledgeCommand(&project))
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

func newSearchCommand(project *string) *cobra.Command {
	var typeFilter string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search tasks and knowledge",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			option := domain.SearchOption{}
			switch typeFilter {
			case "", "task", "knowledge":
				option.Type = domain.SearchType(typeFilter)
			default:
				return fmt.Errorf("%w: invalid --type %q", domain.ErrInvalidArgument, typeFilter)
			}
			results, err := app.Search.Search(cmd.Context(), args[0], option)
			if err != nil {
				return err
			}
			return printResults(cmd, results, jsonOut)
		},
	}
	cmd.Flags().StringVar(&typeFilter, "type", "", "filter by type (task|knowledge)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func newTaskCommand(project *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks",
	}
	cmd.AddCommand(newTaskListCommand(project))
	cmd.AddCommand(newTaskSearchCommand(project))
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

func newTaskSearchCommand(project *string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search tasks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			results, err := app.Search.Search(cmd.Context(), args[0], domain.SearchOption{Type: domain.SearchTypeTask})
			if err != nil {
				return err
			}
			return printResults(cmd, results, jsonOut)
		},
	}
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
	var name string
	var adhoc, jsonOut bool
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
			task, err := app.Tasks.Create(cmd.Context(), application.TaskInput{Name: name, Type: taskType})
			if err != nil {
				return err
			}
			if jsonOut {
				return writeJSON(cmd, task)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), task.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "task name")
	cmd.Flags().BoolVar(&adhoc, "adhoc", false, "create an adhoc task")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	_ = cmd.MarkFlagRequired("name")
	return cmd
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

func newKnowledgeCommand(project *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "knowledge",
		Short: "Manage knowledge",
	}
	cmd.AddCommand(newKnowledgeListCommand(project))
	cmd.AddCommand(newKnowledgeSearchCommand(project))
	cmd.AddCommand(newKnowledgeShowCommand(project))
	cmd.AddCommand(newKnowledgeCreateCommand(project))
	cmd.AddCommand(newKnowledgeEditCommand(project))
	cmd.AddCommand(newKnowledgeMoveCommand(project))
	cmd.AddCommand(newKnowledgeRenameCommand(project))
	return cmd
}

func newKnowledgeListCommand(project *string) *cobra.Command {
	var all, jsonOut bool
	var tag string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List knowledge",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			list, err := app.Knowledge.List(cmd.Context(), application.KnowledgeFilter{Tag: tag})
			if err != nil {
				return err
			}
			if !all {
				list = filterHidden(list)
			}
			return printKnowledge(cmd, list, jsonOut)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "include hidden files")
	cmd.Flags().StringVar(&tag, "tag", "", "filter by tag")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func newKnowledgeSearchCommand(project *string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search knowledge",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			results, err := app.Search.Search(cmd.Context(), args[0], domain.SearchOption{Type: domain.SearchTypeKnowledge})
			if err != nil {
				return err
			}
			return printResults(cmd, results, jsonOut)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func newKnowledgeShowCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "show <path>",
		Short: "Show knowledge details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			k, err := app.Knowledge.Show(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printKnowledgeDetail(cmd, k)
		},
	}
}

func newKnowledgeCreateCommand(project *string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "create <path>",
		Short: "Create knowledge",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			k, err := app.Knowledge.Create(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if jsonOut {
				return writeJSON(cmd, k)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), k.Path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func newKnowledgeEditCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <path>",
		Short: "Edit knowledge in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			path := filepath.Join(app.Project.Path, "knowledge", args[0]+".md")
			return editFile(cmd, path)
		},
	}
}

func newKnowledgeMoveCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "move <old> <new>",
		Short: "Move knowledge to another path",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			if err := app.Knowledge.Move(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "moved %s -> %s\n", args[0], args[1])
			return nil
		},
	}
}

func newKnowledgeRenameCommand(project *string) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old> <new-name>",
		Short: "Rename knowledge",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := NewApp(cmd.Context(), *project)
			if err != nil {
				return err
			}
			if err := app.Knowledge.Rename(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "renamed %s -> %s\n", args[0], args[1])
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

func filterHidden(list []domain.Knowledge) []domain.Knowledge {
	var out []domain.Knowledge
	for _, k := range list {
		hidden := false
		for _, seg := range strings.Split(k.Path, "/") {
			if strings.HasPrefix(seg, ".") {
				hidden = true
				break
			}
		}
		if !hidden {
			out = append(out, k)
		}
	}
	return out
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

func printKnowledge(cmd *cobra.Command, list []domain.Knowledge, jsonOut bool) error {
	if jsonOut {
		return writeJSON(cmd, list)
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "PATH\tTITLE\tTAGS")
	for _, k := range list {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", k.Path, k.Title, strings.Join(k.Tags, ","))
	}
	return w.Flush()
}

func printKnowledgeDetail(cmd *cobra.Command, k *domain.Knowledge) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "Path:\t%s\n", k.Path)
	_, _ = fmt.Fprintf(w, "Title:\t%s\n", k.Title)
	_, _ = fmt.Fprintf(w, "Tags:\t%s\n", strings.Join(k.Tags, ", "))
	_, _ = fmt.Fprintf(w, "Aliases:\t%s\n", strings.Join(k.Aliases, ", "))
	_, _ = fmt.Fprintf(w, "WikiLinks:\t%s\n", strings.Join(k.WikiLinks, ", "))
	if err := w.Flush(); err != nil {
		return err
	}
	if strings.TrimSpace(k.Body) != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", k.Body)
	}
	return nil
}

func printResults(cmd *cobra.Command, results []domain.SearchResult, jsonOut bool) error {
	if jsonOut {
		return writeJSON(cmd, results)
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "TYPE\tPATH\tTITLE\tSNIPPET")
	for _, r := range results {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Type, r.Path, r.Title, r.Snippet)
	}
	return w.Flush()
}

func writeJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
