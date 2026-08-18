package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LockBackend prevents overlapping scans of the same Terraform root.
type LockBackend interface {
	// Acquire locks directory for an exclusive scan. release must be called when the scan finishes.
	Acquire(directory string) (release func(), err error)
}

// LocalFileLockBackend implements LockBackend with a per-root O_EXCL lock file.
type LocalFileLockBackend struct{}

// Acquire creates .terradrift-scan.lock in directory.
func (LocalFileLockBackend) Acquire(directory string) (func(), error) {
	return acquireScanLock(directory)
}

// ParseLockBackend returns a LockBackend for name. Empty and "local" use LocalFileLockBackend.
func ParseLockBackend(name string) (LockBackend, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "local":
		return LocalFileLockBackend{}, nil
	default:
		return nil, fmt.Errorf("unsupported lock backend %q; supported values: local", name)
	}
}

func acquireScanLock(directory string) (func(), error) {
	path := filepath.Join(directory, scanLockFilename)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("terraform scan already running for %s; remove stale %s after confirming no scan is active", directory, path)
		}
		return nil, fmt.Errorf("create terraform scan lock: %w", err)
	}
	if _, err := fmt.Fprintln(file, os.Getpid()); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("write terraform scan lock: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("close terraform scan lock: %w", err)
	}
	// ponytail: local O_EXCL lock; use a shared lock service for distributed runners.
	return func() { _ = os.Remove(path) }, nil
}
