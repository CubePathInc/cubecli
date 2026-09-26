package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const lockFile = "config.lock"

// Lock takes an exclusive, cross-process lock on the config directory and
// returns the function that releases it. It blocks until the lock is free.
//
// Refreshing OAuth tokens needs it: the refresh token rotates on every use and the
// authorization server revokes the whole grant when an already-rotated token is
// presented again. Two cubecli processes refreshing at once would log the user
// out everywhere.
func Lock() (func(), error) {
	if err := os.MkdirAll(Dir(), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(Dir(), lockFile), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open config lock: %w", err)
	}
	if err := lockFileExclusive(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to lock config: %w", err)
	}
	return func() {
		_ = unlockFile(f)
		f.Close()
	}, nil
}
