package pptmutation

import (
	"path/filepath"
	"sync"
)

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
