// Package project resolves and validates the host project directory bind mount.
package project

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

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
	foldCase bool
}

// NewResolver returns a Resolver that inspects paths through fsys and checks
// them against policies; a nil policies allows every directory. foldCase says
// the host file system ignores case in names (macOS), so the home directory
// in other letters is still the home directory.
func NewResolver(fsys FileSystem, policies WorkdirPolicies, foldCase bool) *Resolver {
	return &Resolver{fs: fsys, policies: policies, foldCase: foldCase}
}

// Resolve makes workDir absolute, checks that it is a directory, resolves
// symlinks in it and in home, checks it against workdirs.allow when the user's
// config file sets it, and then applies the newMount guard rails.
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
	if err := r.checkWorkdir(workDir, lexicalHome); err != nil {
		return nil, err
	}
	return newMount(workDir, home, allowBroadMount, r.foldCase)
}

// checkWorkdir refuses workDir, a resolved directory, when the user's config
// file sets workdirs.allow and no root of it holds workDir. The policy is
// asked about home as given, before symlink resolution.
func (r *Resolver) checkWorkdir(workDir, home string) error {
	if r.policies == nil {
		return nil
	}
	policy, err := r.policies.WorkdirPolicy(home)
	if err != nil {
		return err
	}
	if policy == nil {
		return nil // the config file sets no workdirs key: any directory may be mounted
	}
	if r.foldCase {
		if policy, err = policy.FoldCase(); err != nil {
			return err
		}
	}
	if decision := policy.Check(workDir); decision.Verdict != pathpolicy.Allowed {
		return fmt.Errorf("refusing to mount %s: it is not under workdirs.allow; %s", workDir, policy.Remedy(decision, workDir, true))
	}
	return nil
}

// newMount refuses the filesystem root and the home directory (in any case
// when foldCase is set) unless allowBroadMount is set, and a path docker
// cannot mount, and mounts cwd at /workspace/<folder-name>.
func newMount(cwd, home string, allowBroadMount, foldCase bool) (*plan.Mount, error) {
	cwd, home = filepath.Clean(cwd), filepath.Clean(home)
	if !allowBroadMount && cwd == string(filepath.Separator) {
		return nil, fmt.Errorf("refusing to mount the filesystem root; use --allow-broad-mount to override")
	}
	isHome := cwd == home || (foldCase && pathpolicy.FoldName(cwd) == pathpolicy.FoldName(home))
	if !allowBroadMount && isHome {
		return nil, fmt.Errorf("refusing to mount your entire home directory; use --allow-broad-mount to override")
	}
	if strings.Contains(cwd, ":") { // docker -v splits its argument at ":"
		return nil, fmt.Errorf("refusing to mount %s: the path contains \":\", which docker cannot mount", cwd)
	}
	name := filepath.Base(cwd)
	if name == "." || name == string(filepath.Separator) {
		name = "project"
	}
	return &plan.Mount{HostPath: cwd, ContainerPath: filepath.ToSlash(filepath.Join("/workspace", name))}, nil
}
