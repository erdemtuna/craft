package fetch

import (
	"fmt"
	"os"

	"github.com/gofrs/flock"
)

// fileLock represents an exclusive filesystem lock.
type fileLock struct {
	path string
	lock *flock.Flock
}

// lockRepo acquires an exclusive lock for the given repo path.
// Creates a .lock file adjacent to the repo directory.
func lockRepo(repoPath string) (*fileLock, error) {
	lockPath := repoPath + ".lock"
	fl := flock.New(lockPath)
	if err := fl.Lock(); err != nil {
		return nil, fmt.Errorf("acquiring lock: %w", err)
	}
	return &fileLock{path: lockPath, lock: fl}, nil
}

// Unlock releases the filesystem lock.
func (l *fileLock) Unlock() {
	if l.lock != nil {
		_ = l.lock.Unlock()
		_ = os.Remove(l.path)
	}
}
