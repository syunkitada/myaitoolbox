package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunRejectsUnknownCommand(t *testing.T) {
	var output bytes.Buffer
	err := run(context.Background(), []string{"unknown"}, &output)
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunShowsCommandHelp(t *testing.T) {
	var output bytes.Buffer
	if err := run(context.Background(), []string{"--help"}, &output); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	for _, command := range []string{"sync", "update", "status"} {
		if !strings.Contains(output.String(), command) {
			t.Fatalf("help output does not mention %q: %s", command, output.String())
		}
	}
}
