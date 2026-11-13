//go:build js || plan9 || wasip1

package myherdr

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeLockUnsupportedPlatformDoesNotCreateLockFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serve.lock")
	lock, err := acquireServeLock(path)
	if err == nil {
		if lock != nil {
			_ = lock.Close()
		}
		t.Fatal("acquireServeLock() succeeded on an unsupported platform")
	}
	if err.Error() != "myherdr serve is unsupported on this platform" {
		t.Fatalf("acquireServeLock() error = %q, want unsupported-platform error", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unsupported serve created lock file: stat error = %v", err)
	}

	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	err = Serve(context.Background(), Project{}, 0, &strings.Builder{})
	if err == nil || err.Error() != "myherdr serve is unsupported on this platform" {
		t.Fatalf("Serve() error = %v, want unsupported-platform error", err)
	}
}
