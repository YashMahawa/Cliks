package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func validatePermissions(path string, targetPerm os.FileMode) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat %s for permission validation: %w", path, err)
	}
	actual := info.Mode().Perm()
	if targetPerm&0o077 == 0 && actual&0o077 != 0 {
		return fmt.Errorf("permission restriction failed for %s: mode is %04o, expected owner-only", path, actual)
	}
	return nil
}

// atomicWriteFile writes data to path by creating a temp file in the same
// directory, fsyncing it, then renaming into place. This avoids truncated or
// empty config/state files if the process crashes mid-write.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	if perm == 0 {
		perm = 0o600
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	_ = os.Chmod(dir, 0o700)

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("failed to restrict permissions on temporary file %s: %w", tmpName, err)
	}
	if err := validatePermissions(tmpName, perm); err != nil {
		return err
	}

	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	cleanup = false

	if err := os.Chmod(path, perm); err != nil {
		return fmt.Errorf("failed to restrict permissions on file %s: %w", path, err)
	}
	return validatePermissions(path, perm)
}

