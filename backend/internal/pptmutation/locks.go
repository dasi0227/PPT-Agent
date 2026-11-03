package pptmutation

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

var ErrRecoveryRequired = errors.New("project mutation recovery is required")

// CheckProjectWritable fences unresolved file transactions, including after a
// paused Run is canceled. The caller must hold the project file lock.
func CheckProjectWritable(directory string) error {
	entries, err := os.ReadDir(filepath.Join(directory, ".runtime", "mutations"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			return fmt.Errorf("%w: unresolved journal %s", ErrRecoveryRequired, entry.Name())
		}
	}
	return nil
}

// Management edits and Run operations share this lock through file writes,
// metadata persistence and rollback. Readers see only complete commits.
var projectLocks sync.Map

func projectLock(directory string) *sync.RWMutex {
	key, _ := filepath.Abs(directory)
	lock, _ := projectLocks.LoadOrStore(key, &sync.RWMutex{})
	return lock.(*sync.RWMutex)
}

func LockProject(directory string) func() {
	lock := projectLock(directory)
	lock.Lock()
	return lock.Unlock
}

func ReadLockProject(directory string) func() {
	lock := projectLock(directory)
	lock.RLock()
	return lock.RUnlock
}
