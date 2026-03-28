//go:build !windows

package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func LockFile(path string) (func() error, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}

	fh, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o640)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := syscall.Flock(int(fh.Fd()), syscall.LOCK_EX); err != nil {
		_ = fh.Close()
		return nil, fmt.Errorf("lock file: %w", err)
	}

	return func() error {
		defer fh.Close()
		return syscall.Flock(int(fh.Fd()), syscall.LOCK_UN)
	}, nil
}
