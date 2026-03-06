package fetch

import (
	"fmt"
	"os"
	"syscall"
)

// fileLock represents an exclusive filesystem lock.
type fileLock struct {
	path string
	file *os.File
}

// lockRepo acquires an exclusive lock for the given repo path.
// Creates a .lock file adjacent to the repo directory.
func lockRepo(repoPath string) (*fileLock, error) {
	lockPath := repoPath + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening lock file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("acquiring lock: %w", err)
	}

	return &fileLock{path: lockPath, file: f}, nil
}

// Unlock releases the filesystem lock.
func (l *fileLock) Unlock() {
	if l.file != nil {
		_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
		_ = l.file.Close()
	}
}
