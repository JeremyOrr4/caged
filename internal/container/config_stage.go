package container

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/JeremyOrr4/caged/internal/config"
)

func stageConfigs(mounts []config.ResolvedMount) ([]config.ResolvedMount, func() error, error) {
	staged := append([]config.ResolvedMount(nil), mounts...)
	if len(staged) == 0 {
		return staged, func() error { return nil }, nil
	}

	dir, err := os.MkdirTemp("", "caged-config-")
	if err != nil {
		return nil, nil, fmt.Errorf("create config staging directory: %w", err)
	}
	cleanup := func() error { return os.RemoveAll(dir) }
	for i := range staged {
		if staged[i].Writable {
			continue
		}
		path := filepath.Join(dir, strconv.Itoa(i))
		if err := copyConfig(staged[i].HostPath, path, staged[i].Excludes); err != nil {
			cleanupErr := cleanup()
			return nil, nil, errors.Join(
				fmt.Errorf("stage config %q: %w", staged[i].Name, err),
				cleanupErr,
			)
		}
		staged[i].HostPath = path
	}
	return staged, cleanup, nil
}

func copyConfig(source, destination string, excludes []string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("inspect config %q: %w", source, err)
	}
	if info.IsDir() {
		return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			if relative == "." {
				return os.MkdirAll(destination, info.Mode().Perm())
			}
			if excluded(relative, excludes) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			target := filepath.Join(destination, relative)
			entryInfo, err := entry.Info()
			if err != nil {
				return err
			}
			switch {
			case entryInfo.IsDir():
				return os.MkdirAll(target, entryInfo.Mode().Perm())
			case entryInfo.Mode()&os.ModeSymlink != 0:
				link, err := os.Readlink(path)
				if err != nil {
					return err
				}
				return os.Symlink(link, target)
			case entryInfo.Mode().IsRegular():
				return copyRegularFile(path, target, entryInfo.Mode().Perm())
			default:
				return nil
			}
		})
	}
	if excluded(filepath.Base(source), excludes) {
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(source)
		if err != nil {
			return err
		}
		return os.Symlink(link, destination)
	}
	if info.Mode().IsRegular() {
		return copyRegularFile(source, destination, info.Mode().Perm())
	}
	return nil
}

func copyRegularFile(source, destination string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func excluded(relative string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, filepath.Base(relative))
		if err == nil && matched {
			return true
		}
	}
	return false
}
