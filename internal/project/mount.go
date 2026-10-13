// Package project resolves and validates the host project directory bind mount.
package project

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// FileSystem is the filesystem access the resolver needs.
type FileSystem interface {
	Abs(path string) (string, error)
	Stat(name string) (fs.FileInfo, error)
	EvalSymlinks(path string) (string, error)
}

// Resolver validates the host project directory and builds its bind mount.
type Resolver struct {
	fs FileSystem
}

// NewResolver returns a Resolver that inspects paths through fsys.
func NewResolver(fsys FileSystem) *Resolver {
	return &Resolver{fs: fsys}
}

// Resolve makes workDir absolute, checks that it is a directory, resolves
// symlinks in it and in home, and then applies the NewMount guard rails.
func (r *Resolver) Resolve(workDir, home string, allowBroadMount bool) (*plan.Mount, error) {
	workDir, err := r.fs.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}
	info, err := r.fs.Stat(workDir)
	if err != nil {
		return nil, fmt.Errorf("inspect working directory %q: %w", workDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("working directory %q is not a directory", workDir)
	}
	workDir, err = r.fs.EvalSymlinks(workDir)
	if err != nil {
		return nil, fmt.Errorf("resolve working directory symlinks: %w", err)
	}
	home, err = r.fs.EvalSymlinks(home)
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	return NewMount(workDir, home, allowBroadMount)
}

// NewMount refuses the filesystem root and the home directory unless
// allowBroadMount is set, and mounts cwd at /workspace/<folder-name>.
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
