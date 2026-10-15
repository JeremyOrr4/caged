// Package project resolves and validates the host project directory bind mount.
package project

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// FileSystem is the filesystem access the resolver needs.
type FileSystem interface {
	Abs(path string) (string, error)
	Stat(name string) (fs.FileInfo, error)
	EvalSymlinks(path string) (string, error)
}

// WorkdirPolicies supplies the policy for project directories, or nil when
// any directory may be mounted. *userconfig.Store implements it.
type WorkdirPolicies interface {
	WorkdirPolicy(home string) (*pathpolicy.Policy, error)
}

// Resolver validates the host project directory and builds its bind mount.
type Resolver struct {
	fs       FileSystem
	policies WorkdirPolicies
}

// NewResolver returns a Resolver that inspects paths through fsys and checks
// them against policies; a nil policies allows every directory.
func NewResolver(fsys FileSystem, policies WorkdirPolicies) *Resolver {
	return &Resolver{fs: fsys, policies: policies}
}

// Resolve makes workDir absolute, checks that it is a directory, resolves
// symlinks in it and in home, checks it against workdirs.allow when the user's
// config file sets it, and then applies the NewMount guard rails.
// --allow-broad-mount does not bypass workdirs.allow.
func (r *Resolver) Resolve(workDir, home string, allowBroadMount bool) (*plan.Mount, error) {
	lexicalHome := home
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
	if r.policies != nil {
		policy, err := r.policies.WorkdirPolicy(lexicalHome)
		if err != nil {
			return nil, err
		}
		if policy != nil {
			if decision := policy.Check(workDir); decision.Verdict != pathpolicy.Allowed {
				return nil, fmt.Errorf("refusing to mount %s: it is not under workdirs.allow; %s", workDir, policy.Remedy(decision, workDir, true))
			}
		}
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
