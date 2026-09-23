package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mygit/internal/mygit"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "mygit:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	command := newRootCommand(stdout)
	command.SetArgs(args)
	return command.ExecuteContext(ctx)
}

func newRootCommand(stdout io.Writer) *cobra.Command {
	service := mygit.NewService()
	command := &cobra.Command{
		Use:           "mygit",
		Short:         "Manage repositories declared in a mygit manifest",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("usage: mygit <sync|update|status>")
		},
	}
	command.SetOut(stdout)
	command.SetErr(stdout)
	command.AddCommand(
		newRepositoryCommand("sync", "Reproduce repositories from the lockfile.", func(ctx context.Context, root string, output io.Writer) error {
			return service.SyncWithOutput(ctx, root, output)
		}),
		newRepositoryCommand("update", "Resolve revisions and update the lockfile.", func(ctx context.Context, root string, output io.Writer) error {
			return service.UpdateWithOutput(ctx, root, output)
		}),
		newRepositoryCommand("status", "Show the current repository status.", func(ctx context.Context, root string, stdout io.Writer) error {
			return service.Status(ctx, root, stdout)
		}),
	)
	return command
}

func newRepositoryCommand(use, short string, run func(context.Context, string, io.Writer) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := os.Getwd()
			if err != nil {
				return err
			}
			return run(cmd.Context(), root, cmd.OutOrStdout())
		},
	}
}
