package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const lockFile = "config.lock"

// Lock takes an exclusive lock on the config directory, shared by every cubecli
// process, and returns the function that releases it. Token refreshes run under
// it: refresh tokens rotate on every use, and presenting an old one revokes the
// session.
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
