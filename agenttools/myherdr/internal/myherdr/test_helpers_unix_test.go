//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package myherdr

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFakeExecutable(t *testing.T, directory, name, script string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func prependTestPath(t *testing.T, directory string) {
	t.Helper()
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
}
