//go:build linux || android || darwin || ios || freebsd || openbsd || netbsd || dragonfly || illumos

package myherdr

import (
	"os"
	"syscall"
)

func acquireServeLock(path string) (serveLock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}
