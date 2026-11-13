//go:build aix || (solaris && !illumos)

package myherdr

import (
	"errors"
	"io"
	"os"
	"sync"
	"syscall"
)

var fcntlServeLockMu sync.Mutex

type fcntlServeLock struct {
	file     *os.File
	close    sync.Once
	closeErr error
}

func acquireServeLock(path string) (serveLock, error) {
	if !fcntlServeLockMu.TryLock() {
		return nil, errors.New("myherdr serve is already running in this process")
	}
	keepProcessLock := false
	defer func() {
		if !keepProcessLock {
			fcntlServeLockMu.Unlock()
		}
	}()

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	lock := syscall.Flock_t{
		Type:   syscall.F_WRLCK,
		Whence: int16(io.SeekStart),
		Start:  0,
		Len:    0,
	}
	if err := syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &lock); err != nil {
		_ = file.Close()
		return nil, err
	}
	keepProcessLock = true
	return &fcntlServeLock{file: file}, nil
}

func (lock *fcntlServeLock) Close() error {
	lock.close.Do(func() {
		lock.closeErr = lock.file.Close()
		fcntlServeLockMu.Unlock()
	})
	return lock.closeErr
}
