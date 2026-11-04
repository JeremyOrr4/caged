// Package hostconfig decides which host config paths are mounted into the
// container and refuses anything that is unsafe to expose. Symlinks are
// followed on the host and their targets mounted where the links point, when
// the link policy allows the target.
package hostconfig

import (
	"io/fs"
	"path"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// containerHome is the home directory of the user inside the runtime image.
const containerHome = "/home/caged"

// maxLinks bounds the symlinks one Resolve follows; maxLinkDepth bounds how
// many symlinked directories may be nested inside each other.
const (
	maxLinks     = 64
	maxLinkDepth = 8
)

// FileSystem is the read-only filesystem access the resolver needs. WalkDir
// must behave as filepath.WalkDir does, including the second call, with the
// error, for a directory it cannot read: that is how an unreadable staged
// tree is refused.
type FileSystem interface {
	Stat(name string) (fs.FileInfo, error)
	Lstat(name string) (fs.FileInfo, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	EvalSymlinks(path string) (string, error)
	WalkDir(root string, fn fs.WalkDirFunc) error
	Readlink(name string) (string, error)
}

// Env looks up config-directory overrides such as CODEX_HOME.
type Env interface {
	LookupEnv(key string) (string, bool)
}

// LinkPolicies supplies the policy that decides which symlink targets may be
// mounted. *userconfig.Store implements it.
type LinkPolicies interface {
	LinkPolicy(home string) (*pathpolicy.Policy, error)
}

// ConfigDirs supplies caged's own config directory, which is mounted
// read/write so the config can be edited inside the container, and the
// folder of its approved copies, which that mount must not reach.
// *userconfig.Store implements it.
type ConfigDirs interface {
	Dir() (string, error)
	StateDir() (string, error)
}

// SharedFile is a regular file in the home directory, such as .gitconfig,
// that all agents share. It is mounted read-only at the same relative path
// under the container home.
type SharedFile struct {
	path        string // relative to the home directory
	description string // used in refusal messages
}

// DefaultSharedFiles returns the shared files caged mounts.
func DefaultSharedFiles() []SharedFile {
	return []SharedFile{{path: ".gitconfig", description: "shared git config"}}
}

// Deps are the collaborators of a Resolver. FS, Env and Policies are
// required. Agents' config paths are resolved first, in order, then Shared:
// when two of them name the same mount, the first owner keeps it.
type Deps struct {
	FS       FileSystem
	Env      Env
	Agents   []agent.Agent
	Shared   []SharedFile
	Policies LinkPolicies // decides which symlinks are followed
	// ConfigDirs reports caged's own config directory, which is then also
	// mounted, at ~/.config/caged in the container. A nil interface means it
	// is not mounted.
	ConfigDirs ConfigDirs
	// FoldCase says the host file system ignores case in names (macOS), so
	// the deny list and caged's own folders are matched in any case.
	FoldCase bool
}

// Resolver finds the agent config paths and shared files that can be mounted.
type Resolver struct {
	deps Deps
}

// NewResolver returns a Resolver built from deps.
func NewResolver(deps Deps) *Resolver {
	return &Resolver{deps: deps}
}

// Resolve inspects every config path under home and reports what will be
// mounted, what is missing, and what is refused. It fails only when the link
// policy cannot be loaded (for example because the config file is invalid) or
// when ConfigDirs cannot report caged's config directory.
func (r *Resolver) Resolve(home string) (plan.ConfigReport, error) {
	home = filepath.Clean(home)
	resolvedHome, err := r.deps.FS.EvalSymlinks(home)
	if err != nil {
		return plan.ConfigReport{Refused: []plan.Refusal{{Path: home, Reason: "home cannot be resolved"}}}, nil
	}
	policy, err := r.deps.Policies.LinkPolicy(home)
	if err != nil {
		return plan.ConfigReport{}, err
	}
	if r.deps.FoldCase {
		if policy, err = policy.FoldCase(); err != nil {
			return plan.ConfigReport{}, err
		}
	}
	pass := &resolution{fs: r.deps.FS, policy: policy, home: home, resolvedHome: resolvedHome, seen: make(map[string]struct{})}
	for _, definition := range r.deps.Agents {
		for _, configPath := range definition.ConfigPaths {
			pass.agentPath(definition.Name, configPath, r.hostPath(home, configPath))
		}
	}
	for _, file := range r.deps.Shared {
		pass.sharedFile(file)
	}
	if r.deps.ConfigDirs != nil {
		dir, err := r.deps.ConfigDirs.Dir()
		if err != nil {
			return plan.ConfigReport{}, err
		}
		state, err := r.deps.ConfigDirs.StateDir()
		if err != nil {
			return plan.ConfigReport{}, err
		}
		pass.cagedDir(dir, state)
	}
	pass.dropNestedLinkMounts()
	return pass.report, nil
}

// hostPath is where a config path lives on the host. A set override variable
// (absolute, or relative to home) replaces the default location.
func (r *Resolver) hostPath(home string, path agent.ConfigPath) string {
	hostPath := filepath.Join(home, filepath.Clean(path.HostPath))
	if path.EnvOverride == "" {
		return hostPath
	}
	override, _ := r.deps.Env.LookupEnv(path.EnvOverride)
	if override == "" {
		return hostPath
	}
	if !filepath.IsAbs(override) {
		override = filepath.Join(home, override)
	}
	return filepath.Clean(override)
}

// inContainer returns rel, relative to the container home, as a container path.
func inContainer(rel string) string {
	return path.Join(containerHome, filepath.ToSlash(rel))
}
