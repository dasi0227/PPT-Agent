// Package workrootlock prevents two processes from mutating one work directory.
package workrootlock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

func Acquire(root string) (func(), error) {
	if root == "" {
		return nil, errors.New("work directory is required for the writer lock")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(root, ".writer.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := tryLock(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("work directory is already open by another writer: %w", err)
	}
	var once sync.Once
	return func() { once.Do(func() { _ = unlock(f); _ = f.Close() }) }, nil
}
