// Package hostconfig decides which host config paths are mounted into the
// container and refuses anything that is unsafe to expose.
package hostconfig

import (
	"io/fs"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// SharedOwner is the ConfigMount.Agent value of mounts shared by all agents.
// No agent may use this name.
const SharedOwner = plan.SharedOwner

// containerHome is the home directory of the user inside the runtime image.
const containerHome = "/home/caged"

// FileSystem is the read-only filesystem access the resolver needs.
type FileSystem interface {
	Stat(name string) (fs.FileInfo, error)
	Lstat(name string) (fs.FileInfo, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	EvalSymlinks(path string) (string, error)
	WalkDir(root string, fn fs.WalkDirFunc) error
}

// Env looks up config-directory overrides such as CODEX_HOME.
type Env interface {
	LookupEnv(key string) (string, bool)
}

// SharedFile is a regular file in the home directory, such as .gitconfig,
// that all agents share. It is mounted read-only at the same relative path
// under the container home.
type SharedFile struct {
	Path        string // relative to the home directory
	Description string // used in refusal messages
}

// DefaultSharedFiles returns the shared files caged mounts.
func DefaultSharedFiles() []SharedFile {
	return []SharedFile{{Path: ".gitconfig", Description: "shared git config"}}
}

// Resolver finds the agent config paths and shared files that can be mounted.
type Resolver struct {
	fs     FileSystem
	env    Env
	agents []agent.Agent
	shared []SharedFile
}

// NewResolver returns a Resolver for the config paths of agents, in order,
// followed by the shared files.
func NewResolver(fsys FileSystem, env Env, agents []agent.Agent, shared []SharedFile) *Resolver {
	return &Resolver{fs: fsys, env: env, agents: agents, shared: shared}
}

// Resolve inspects every config path under home and reports what will be
// mounted, what is missing, and what is refused.
func (r *Resolver) Resolve(home string) plan.ConfigReport {
	home = filepath.Clean(home)
	resolvedHome, err := r.fs.EvalSymlinks(home)
	if err != nil {
		return plan.ConfigReport{Refused: []plan.Refusal{{Path: home, Reason: "home cannot be resolved"}}}
	}
	pass := &resolution{fs: r.fs, home: home, resolvedHome: resolvedHome, seen: make(map[string]struct{})}
	for _, definition := range r.agents {
		for _, path := range definition.ConfigPaths {
			pass.agentPath(definition.Name, path, r.hostPath(home, path))
		}
	}
	for _, file := range r.shared {
		pass.sharedFile(file)
	}
	return pass.report
}

// hostPath is where a config path lives on the host. A set override variable
// (absolute, or relative to home) replaces the default location.
func (r *Resolver) hostPath(home string, path agent.ConfigPath) string {
	hostPath := filepath.Join(home, filepath.Clean(path.HostPath))
	if path.EnvOverride == "" {
		return hostPath
	}
	override, _ := r.env.LookupEnv(path.EnvOverride)
	if override == "" {
		return hostPath
	}
	if !filepath.IsAbs(override) {
		override = filepath.Join(home, override)
	}
	return filepath.Clean(override)
}

// resolution accumulates the report of one Resolve call.
type resolution struct {
	fs           FileSystem
	home         string
	resolvedHome string
	seen         map[string]struct{} // host path + "\x00" + container path already mounted
	report       plan.ConfigReport
}

// agentPath checks one agent config path and records its mounts, or why it
// is missing or refused.
func (r *resolution) agentPath(agentName string, path agent.ConfigPath, hostPath string) {
	if !r.exists(hostPath) {
		r.report.Missing = appendUnique(r.report.Missing, hostPath)
		return
	}
	resolvedHost, err := r.fs.EvalSymlinks(hostPath)
	if err != nil {
		r.refuse(hostPath, "unresolvable symlink")
		return
	}
	if filepath.Clean(resolvedHost) == filepath.Clean(r.resolvedHome) {
		r.refuse(hostPath, "config path resolves to the home directory")
		return
	}
	if !isWithin(resolvedHost, r.resolvedHome) {
		r.refuse(hostPath, "resolves outside home")
		return
	}
	name, dest := filepath.Clean(path.HostPath), filepath.Clean(path.ContainerPath)
	if len(path.MaskPaths) > 0 {
		// A masked tree is mounted whole; tmpfs mounts hide the masked entries.
		if err := validateMaskedConfigTree(r.fs, resolvedHost, path.MaskPaths); err != nil {
			r.refuse(hostPath, err.Error())
			return
		}
		r.addMount(agentName, name, hostPath, resolvedHost, dest, path.Writable, append([]string(nil), path.MaskPaths...))
		return
	}
	mounts, refused := collectSafeMounts(r.fs, resolvedHost)
	for _, refusal := range refused {
		r.report.Refused = appendUnique(r.report.Refused, refusal)
	}
	for _, mountPath := range mounts {
		relative, err := filepath.Rel(resolvedHost, mountPath)
		if err != nil {
			r.refuse(hostPath, "cannot map safe config path")
			continue
		}
		mountDest := dest
		if relative != "." {
			mountDest = filepath.Join(dest, relative)
		}
		r.addMount(agentName, name, hostPath, mountPath, mountDest, path.Writable, nil)
	}
}

// sharedFile checks one shared file. It must resolve inside home and be a
// regular file, and it is always mounted read-only.
func (r *resolution) sharedFile(file SharedFile) {
	name := filepath.Clean(file.Path)
	hostPath := filepath.Join(r.home, name)
	if !r.exists(hostPath) {
		r.report.Missing = appendUnique(r.report.Missing, hostPath)
		return
	}
	resolved, err := r.fs.EvalSymlinks(hostPath)
	if err != nil || !isWithin(resolved, r.resolvedHome) {
		r.refuse(hostPath, "resolves outside home or cannot be resolved")
		return
	}
	if info, err := r.fs.Stat(resolved); err != nil || !info.Mode().IsRegular() {
		r.refuse(hostPath, file.Description+" must be a regular file")
		return
	}
	r.report.Mounts = append(r.report.Mounts, plan.ConfigMount{
		Agent:    SharedOwner,
		Name:     name,
		HostPath: resolved,
		DestPath: filepath.ToSlash(filepath.Join(containerHome, name)),
		Root:     hostPath,
		Writable: false,
	})
}

// addMount records a mount of hostPath unless the same host path is already
// mounted at the same container path. root is the config root before symlink
// resolution.
func (r *resolution) addMount(agentName, name, root, hostPath, dest string, writable bool, masks []string) {
	key := hostPath + "\x00" + dest
	if _, ok := r.seen[key]; ok {
		return
	}
	r.seen[key] = struct{}{}
	r.report.Mounts = append(r.report.Mounts, plan.ConfigMount{
		Agent:     agentName,
		Name:      name,
		HostPath:  hostPath,
		DestPath:  filepath.ToSlash(filepath.Join(containerHome, dest)),
		Root:      root,
		Writable:  writable,
		MaskPaths: masks,
	})
}

func (r *resolution) exists(path string) bool {
	_, err := r.fs.Stat(path)
	return err == nil
}

func (r *resolution) refuse(path, reason string) {
	r.report.Refused = appendUnique(r.report.Refused, plan.Refusal{Path: path, Reason: reason})
}
