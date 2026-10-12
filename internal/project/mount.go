// Package project resolves and validates the host project directory bind mount.
package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/plan"
)

func ResolveMount(workDir, home string, allowBroadMount bool) (*plan.Mount, error) {
	workDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}
	info, err := os.Stat(workDir)
	if err != nil {
		return nil, fmt.Errorf("inspect working directory %q: %w", workDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("working directory %q is not a directory", workDir)
	}
	workDir, err = filepath.EvalSymlinks(workDir)
	if err != nil {
		return nil, fmt.Errorf("resolve working directory symlinks: %w", err)
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	return NewMount(workDir, home, allowBroadMount)
}

func NewMount(cwd, home string, allowBroadMount bool) (*plan.Mount, error) {
	cwd, home = filepath.Clean(cwd), filepath.Clean(home)
	if !allowBroadMount && cwd == string(filepath.Separator) {
		return nil, fmt.Errorf("refusing to mount the filesystem root; use --allow-broad-mount to override")
	}
	if !allowBroadMount && cwd == home {
		return nil, fmt.Errorf("refusing to mount your entire home directory; use --allow-broad-mount to override")
	}
	name := filepath.Base(cwd)
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = "project"
	}
	return &plan.Mount{HostPath: cwd, ContainerPath: filepath.ToSlash(filepath.Join("/workspace", name))}, nil
}
