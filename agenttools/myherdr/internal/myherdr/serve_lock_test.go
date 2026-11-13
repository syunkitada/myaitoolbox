//go:build linux || android || darwin || ios || freebsd || openbsd || netbsd || dragonfly || illumos || aix || solaris || windows

package myherdr

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const serveLockHelperEnv = "MYHERDR_TEST_SERVE_LOCK_PATH"

var serveLockHelper serveLock

func TestServeLockCanBeAcquiredAfterOwnerProcessExits(t *testing.T) {
	if path := os.Getenv(serveLockHelperEnv); path != "" {
		var err error
		serveLockHelper, err = acquireServeLock(path)
		if err != nil {
			_, _ = os.Stderr.WriteString(err.Error())
			os.Exit(2)
		}
		os.Exit(0)
	}

	path := filepath.Join(t.TempDir(), "serve.lock")
	command := exec.Command(os.Args[0], "-test.run=^TestServeLockCanBeAcquiredAfterOwnerProcessExits$")
	command.Env = append(os.Environ(), serveLockHelperEnv+"="+path)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("lock owner process failed: %v: %s", err, output)
	}

	lock, err := acquireServeLock(path)
	if err != nil {
		t.Fatalf("acquireServeLock() after owner exit: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("close serve lock: %v", err)
	}
}

func TestServeLockRejectsSecondAcquisitionInSameProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serve.lock")
	first, err := acquireServeLock(path)
	if err != nil {
		t.Fatalf("first acquireServeLock(): %v", err)
	}
	if second, err := acquireServeLock(path); err == nil {
		if second != nil {
			_ = second.Close()
		}
		_ = first.Close()
		t.Fatal("second acquireServeLock() succeeded while the first lock was held")
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first serve lock: %v", err)
	}
	third, err := acquireServeLock(path)
	if err != nil {
		t.Fatalf("acquireServeLock() after close: %v", err)
	}
	if err := third.Close(); err != nil {
		t.Fatalf("close third serve lock: %v", err)
	}
}
