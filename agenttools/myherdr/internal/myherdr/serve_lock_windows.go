//go:build windows

package myherdr

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

const (
	lockFileFailImmediately = 0x00000001
	lockFileExclusiveLock   = 0x00000002
)

var (
	kernel32DLL  = syscall.NewLazyDLL("kernel32.dll")
	lockFileEx   = kernel32DLL.NewProc("LockFileEx")
	unlockFileEx = kernel32DLL.NewProc("UnlockFileEx")
)

type windowsServeLock struct {
	file       *os.File
	overlapped syscall.Overlapped
}

func acquireServeLock(path string) (serveLock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	lock := &windowsServeLock{file: file}
	result, _, callErr := lockFileEx.Call(
		file.Fd(),
		lockFileFailImmediately|lockFileExclusiveLock,
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&lock.overlapped)),
	)
	if result == 0 {
		_ = file.Close()
		if callErr == syscall.Errno(0) {
			callErr = syscall.GetLastError()
		}
		return nil, callErr
	}
	return lock, nil
}

func (lock *windowsServeLock) Close() error {
	result, _, callErr := unlockFileEx.Call(
		lock.file.Fd(),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&lock.overlapped)),
	)
	var unlockErr error
	if result == 0 {
		unlockErr = callErr
		if unlockErr == syscall.Errno(0) {
			unlockErr = syscall.GetLastError()
		}
	}
	return errors.Join(unlockErr, lock.file.Close())
}
