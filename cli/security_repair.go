package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// remediatePermissions scans session state, config, and internal CLI runtime
// directories and files, automatically restricting any overly permissive artifacts.
func remediatePermissions() {
	if runtime.GOOS == "windows" {
		return
	}

	dirsToRepair := []string{
		stateDir(),
		filepath.Dir(configPath()),
	}

	if cacheRoot, err := os.UserCacheDir(); err == nil && cacheRoot != "" {
		dirsToRepair = append(dirsToRepair, filepath.Join(cacheRoot, "cliks"))
	}

	for _, dir := range dirsToRepair {
		repairDirectoryPermissions(dir)
	}

	if legacy := legacyConfigPath(); legacy != "" {
		repairFilePermissions(legacy)
	}
}

func repairDirectoryPermissions(dir string) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return
	}

	if info.Mode().Perm()&0o077 != 0 {
		_ = os.Chmod(dir, 0o700)
	}

	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		entryInfo, err := d.Info()
		if err != nil {
			return nil
		}
		currentPerm := entryInfo.Mode().Perm()
		if currentPerm&0o077 != 0 {
			if d.IsDir() {
				_ = os.Chmod(path, 0o700)
			} else {
				_ = os.Chmod(path, 0o600)
			}
		}
		return nil
	})
}

func repairFilePermissions(path string) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}
	if info.Mode().Perm()&0o077 != 0 {
		_ = os.Chmod(path, 0o600)
	}
}
