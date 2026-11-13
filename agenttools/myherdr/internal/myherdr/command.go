package myherdr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func Run(ctx context.Context, args []string, input io.Reader, output, errorOutput io.Writer) error {
	root := newRootCommand(input, output, errorOutput)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

func newRootCommand(input io.Reader, output, errorOutput io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "myherdr",
		Short:         "Use Herdr sessions with Codex and a local browser terminal",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("usage: myherdr <start|session-name|serve|url>")
		},
	}
	root.SetIn(input)
	root.SetOut(output)
	root.SetErr(errorOutput)
	root.AddCommand(newStartCommand(output))
	root.AddCommand(newSessionNameCommand(output))
	root.AddCommand(newServeCommand(output))
	root.AddCommand(newURLCommand(output))
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "myherdr 0.1.0")
			return err
		},
	})
	return root
}

func newStartCommand(output io.Writer) *cobra.Command {
	var projectPath, session string
	command := &cobra.Command{
		Use:   "start",
		Short: "Start or attach to this project's named Herdr session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			project, err := ResolveProject(projectPath)
			if err != nil {
				return err
			}
			name := session
			if name == "" {
				name = DefaultSessionName(project.Name)
			}
			if err := ValidateSessionName(name); err != nil {
				return err
			}

			if os.Getenv("HERDR_ENV") == "1" {
				current := os.Getenv("HERDR_SESSION")
				if current == "" {
					current = "current"
				}
				_, err := fmt.Fprintf(output, "Already inside Herdr session %q. Continue in the current pane; Herdr will not be started inside itself.\n", current)
				return err
			}

			binary, err := exec.LookPath("herdr")
			if err != nil {
				return errors.New("herdr is not installed or not on PATH; install it from https://herdr.dev/ and try again")
			}
			child := exec.CommandContext(cmd.Context(), binary, "--session", name)
			child.Dir = project.Root
			child.Stdin = cmd.InOrStdin()
			child.Stdout = cmd.OutOrStdout()
			child.Stderr = cmd.ErrOrStderr()
			if err := child.Run(); err != nil {
				return fmt.Errorf("start Herdr session %q: %w", name, err)
			}
			return nil
		},
	}
	command.Flags().StringVar(&projectPath, "project", "", "project directory (default: current Git repository root)")
	command.Flags().StringVar(&session, "session", "", "Herdr session name (default: codex-<project-name>)")
	return command
}

func newSessionNameCommand(output io.Writer) *cobra.Command {
	var projectPath, session string
	command := &cobra.Command{
		Use:   "session-name",
		Short: "Print the Herdr command for this project's session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			project, err := ResolveProject(projectPath)
			if err != nil {
				return err
			}
			name := session
			if name == "" {
				name = DefaultSessionName(project.Name)
			}
			if err := ValidateSessionName(name); err != nil {
				return err
			}
			_, err = fmt.Fprintln(output, SessionCommand(name))
			return err
		},
	}
	command.Flags().StringVar(&projectPath, "project", "", "project directory (default: current Git repository root)")
	command.Flags().StringVar(&session, "session", "", "Herdr session name (default: codex-<project-name>)")
	return command
}

func newServeCommand(output io.Writer) *cobra.Command {
	var port int
	command := &cobra.Command{
		Use:   "serve",
		Short: "Serve the local browser terminal",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			project, err := ResolveProject("")
			if err != nil {
				return err
			}
			return Serve(cmd.Context(), project, port, output)
		},
	}
	command.Flags().IntVar(&port, "port", 0, "loopback HTTP port (default: an available port)")
	return command
}

func newURLCommand(output io.Writer) *cobra.Command {
	var pane string
	command := &cobra.Command{
		Use:   "url <session>",
		Short: "Print a URL for an existing local or configured remote Herdr session",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			state, err := FindServeInstance()
			if err != nil {
				return err
			}
			target, err := SessionURL(state.Port, args[0], pane)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(output, target)
			return err
		},
	}
	command.Flags().StringVar(&pane, "pane", "", "open a specific existing pane ID")
	return command
}
