//go:build windows

package fsutil

import "sync"

var windowsLocks sync.Map

func LockFile(path string) (func() error, error) {
	value, _ := windowsLocks.LoadOrStore(path, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return func() error {
		mu.Unlock()
		return nil
	}, nil
}
