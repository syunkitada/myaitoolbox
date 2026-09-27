package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	myntfy "github.com/syunkitada/myaitoolbox/myntfy/internal/myntfy"
)

const version = "0.1.0"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "myntfy:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	return runWithInput(ctx, args, os.Stdin, output)
}

func runWithInput(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	topicPath, err := myntfy.DefaultTopicPath()
	if err != nil {
		return err
	}
	root := newRootCommand(myntfy.NewTopicStore(topicPath), http.DefaultClient)
	root.SetIn(input)
	root.SetOut(output)
	root.SetErr(output)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

func newRootCommand(store *myntfy.TopicStore, httpClient *http.Client) *cobra.Command {
	root := &cobra.Command{
		Use:           "myntfy",
		Short:         "Send and subscribe to ntfy notifications",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCommand())
	root.AddCommand(newTopicCommand(store))
	root.AddCommand(newSendCommand(store, httpClient))
	root.AddCommand(newSubscribeCommand(store, httpClient))
	return root
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "myntfy "+version)
			return nil
		},
	}
}

func newTopicCommand(store *myntfy.TopicStore) *cobra.Command {
	command := &cobra.Command{
		Use:   "topic",
		Short: "Manage the saved ntfy topic",
		Args:  cobra.NoArgs,
	}
	command.AddCommand(newTopicShowCommand(store))
	command.AddCommand(newTopicRegenerateCommand(store))
	return command
}

func newTopicShowCommand(store *myntfy.TopicStore) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the saved topic",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			topic, err := store.Read()
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), topic)
			return nil
		},
	}
}

func newTopicRegenerateCommand(store *myntfy.TopicStore) *cobra.Command {
	var prefix string

	command := &cobra.Command{
		Use:   "regenerate --prefix <prefix>",
		Short: "Generate and save a new topic",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			confirmedPrefix, err := confirmPrefix(cmd, prefix)
			if err != nil {
				return err
			}
			topic, err := store.Regenerate(confirmedPrefix)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), topic)
			return nil
		},
	}
	command.Flags().StringVar(&prefix, "prefix", "", "topic prefix identifying the sender (maximum 31 characters)")
	_ = command.MarkFlagRequired("prefix")
	return command
}

func confirmPrefix(cmd *cobra.Command, prefix string) (string, error) {
	replaced, changed := myntfy.ReplaceInvalidPrefixChars(prefix)
	if !changed {
		return prefix, nil
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "prefix %q contains unsupported characters; replace with %q? [y/N] ", prefix, replaced)
	answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	_, _ = fmt.Fprintln(cmd.OutOrStdout())
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read prefix confirmation: %w", err)
	}
	if answer := strings.ToLower(strings.TrimSpace(answer)); answer == "y" || answer == "yes" {
		return replaced, nil
	}
	return "", errors.New("prefix replacement canceled")
}

func newSendCommand(store *myntfy.TopicStore, httpClient *http.Client) *cobra.Command {
	var server, title, priority, tags string
	command := &cobra.Command{
		Use:   "send <message>",
		Short: "Send a notification using the saved topic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			topic, err := store.Read()
			if err != nil {
				return err
			}
			if err := myntfy.ValidatePriority(priority); err != nil {
				return err
			}
			client, err := newClient(server, httpClient)
			if err != nil {
				return err
			}
			if err := client.Publish(cmd.Context(), topic, args[0], myntfy.PublishOptions{
				Title: title, Priority: priority, Tags: tags,
			}); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "sent")
			return nil
		},
	}
	command.Flags().StringVar(&server, "server", "", "ntfy server URL (default: $MYNTFY_SERVER or https://ntfy.sh)")
	command.Flags().StringVar(&title, "title", "", "notification title")
	command.Flags().StringVar(&priority, "priority", "", "notification priority (min, low, default, high, max, urgent, or 1-5)")
	command.Flags().StringVar(&tags, "tags", "", "comma-separated notification tags")
	return command
}

func newSubscribeCommand(store *myntfy.TopicStore, httpClient *http.Client) *cobra.Command {
	var server string
	command := &cobra.Command{
		Use:   "subscribe",
		Short: "Stream JSON events from the saved topic",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			topic, err := store.Read()
			if err != nil {
				return err
			}
			client, err := newClient(server, httpClient)
			if err != nil {
				return err
			}
			return client.Subscribe(cmd.Context(), topic, cmd.OutOrStdout())
		},
	}
	command.Flags().StringVar(&server, "server", "", "ntfy server URL (default: $MYNTFY_SERVER or https://ntfy.sh)")
	return command
}

func newClient(flagValue string, httpClient *http.Client) (*myntfy.Client, error) {
	return myntfy.NewClient(myntfy.ResolveServerURL(flagValue), httpClient)
}
