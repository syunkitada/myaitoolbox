package fsutil

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to path via a temp file in the same directory
// followed by an atomic rename. A crash mid-write can never leave a truncated
// or partially-written file at path, which matters because the project files
// (tasks, notes, config/state) are the only copy of the user's data.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	return WriteFileAtomicReader(path, bytes.NewReader(data), perm)
}

// WriteFileAtomicReader is the streaming counterpart of WriteFileAtomic. It
// keeps the temporary file in the destination directory so the final rename
// remains atomic, without requiring the entire input to be held in memory.
func WriteFileAtomicReader(path string, data io.Reader, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := io.Copy(tmp, data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	tmpName = ""
	return nil
}
