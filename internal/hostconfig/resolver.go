// Package hostconfig decides which host config paths are mounted into the
// container and refuses anything that is unsafe to expose. Symlinks are
// followed on the host and their targets mounted where the links point, when
// the link policy allows the target.
package hostconfig

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// SharedOwner is the ConfigMount.Agent value of mounts shared by all agents.
// No agent may use this name.
const SharedOwner = plan.SharedOwner

// containerHome is the home directory of the user inside the runtime image.
const containerHome = "/home/caged"

// maxLinks bounds the symlinks one Resolve follows; maxLinkDepth bounds how
// many symlinked directories may be nested inside each other.
const (
	maxLinks     = 64
	maxLinkDepth = 8
)

// FileSystem is the read-only filesystem access the resolver needs.
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
	fs       FileSystem
	env      Env
	agents   []agent.Agent
	shared   []SharedFile
	policies LinkPolicies
}

// NewResolver returns a Resolver for the config paths of agents, in order,
// followed by the shared files. policies decides which symlinks are followed.
func NewResolver(fsys FileSystem, env Env, agents []agent.Agent, shared []SharedFile, policies LinkPolicies) *Resolver {
	return &Resolver{fs: fsys, env: env, agents: agents, shared: shared, policies: policies}
}

// Resolve inspects every config path under home and reports what will be
// mounted, what is missing, and what is refused. It fails only when the link
// policy cannot be loaded, for example because the config file is invalid.
func (r *Resolver) Resolve(home string) (plan.ConfigReport, error) {
	home = filepath.Clean(home)
	resolvedHome, err := r.fs.EvalSymlinks(home)
	if err != nil {
		return plan.ConfigReport{Refused: []plan.Refusal{{Path: home, Reason: "home cannot be resolved"}}}, nil
	}
	policy, err := r.policies.LinkPolicy(home)
	if err != nil {
		return plan.ConfigReport{}, err
	}
	pass := &resolution{fs: r.fs, policy: policy, home: home, resolvedHome: resolvedHome, seen: make(map[string]struct{})}
	for _, definition := range r.agents {
		for _, configPath := range definition.ConfigPaths {
			pass.agentPath(definition.Name, configPath, r.hostPath(home, configPath))
		}
	}
	for _, file := range r.shared {
		pass.sharedFile(file)
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
	fs            FileSystem
	policy        *pathpolicy.Policy
	home          string
	resolvedHome  string
	seen          map[string]struct{} // host path + "\x00" + container path already mounted
	linksFollowed int
	report        plan.ConfigReport
}

// owner is what every mount made for one config path shares.
type owner struct {
	agent, name string
	root        string // the config path before symlink resolution
	writable    bool
}

// agentPath checks one agent config path and records its mounts, or why it
// is missing or refused.
func (r *resolution) agentPath(agentName string, configPath agent.ConfigPath, hostPath string) {
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
	o := owner{agent: agentName, name: filepath.Clean(configPath.HostPath), root: hostPath, writable: configPath.Writable}
	via, ok := r.checkRoot(hostPath, resolvedHost, &o)
	if !ok {
		return
	}
	dest := inContainer(filepath.Clean(configPath.ContainerPath))
	if len(configPath.MaskPaths) > 0 {
		// A masked tree is mounted whole; tmpfs mounts hide the masked entries.
		links, err := validateMaskedConfigTree(r.fs, resolvedHost, configPath.MaskPaths)
		if err != nil {
			r.refuse(hostPath, err.Error())
			return
		}
		r.addMount(o, resolvedHost, dest, append([]string(nil), configPath.MaskPaths...), via)
		for _, link := range links {
			r.followMaskedLink(o, resolvedHost, link, dest)
		}
		return
	}
	r.mountTree(o, resolvedHost, dest, 0, via)
}

// sharedFile checks one shared file. It must be a regular file, inside home
// or reached through an allowed symlink, and it is always mounted read-only.
func (r *resolution) sharedFile(file SharedFile) {
	name := filepath.Clean(file.Path)
	hostPath := filepath.Join(r.home, name)
	if !r.exists(hostPath) {
		r.report.Missing = appendUnique(r.report.Missing, hostPath)
		return
	}
	resolved, err := r.fs.EvalSymlinks(hostPath)
	if err != nil {
		r.refuse(hostPath, "unresolvable symlink")
		return
	}
	o := owner{agent: SharedOwner, name: name, root: hostPath, writable: false}
	via, ok := r.checkRoot(hostPath, resolved, &o)
	if !ok {
		return
	}
	if info, err := r.fs.Stat(resolved); err != nil || !info.Mode().IsRegular() {
		r.refuse(hostPath, file.Description+" must be a regular file")
		return
	}
	r.addMount(o, resolved, inContainer(name), nil, via)
}

// checkRoot applies the link policy to a config root reached through a
// symlink, and the inside-home rule to one that is not. It returns the link to
// record on the mounts ("" for a direct path) and false when the root is refused.
func (r *resolution) checkRoot(hostPath, resolved string, o *owner) (string, bool) {
	if !r.linked(hostPath, resolved) {
		if !isWithin(resolved, r.resolvedHome) {
			r.refuse(hostPath, "resolves outside home")
			return "", false
		}
		return "", true
	}
	decision := r.policy.Check(resolved)
	if decision.Verdict != pathpolicy.Allowed {
		r.refuseLink(hostPath, resolved, decision)
		return "", false
	}
	o.writable = o.writable && !decision.Root.ReadOnly
	return hostPath, true
}

// linked reports whether hostPath reaches resolved through a symlink: in any
// component below home, or anywhere when hostPath is outside home.
func (r *resolution) linked(hostPath, resolved string) bool {
	expected := filepath.Clean(hostPath)
	if isWithin(hostPath, r.home) {
		if rel, err := filepath.Rel(r.home, hostPath); err == nil {
			expected = filepath.Join(r.resolvedHome, rel)
		}
	}
	return filepath.Clean(resolved) != expected
}

// mountTree mounts root, a resolved file or directory, at dest: its largest
// symlink-free subtrees directly, and each symlink inside it through followLink.
func (r *resolution) mountTree(o owner, root, dest string, depth int, via string) {
	mounts, links, refused := collectSafeMounts(r.fs, root)
	for _, refusal := range refused {
		r.report.Refused = appendUnique(r.report.Refused, refusal)
	}
	for _, mountPath := range mounts {
		if mountDest, ok := r.below(root, mountPath, dest); ok {
			r.addMount(o, mountPath, mountDest, nil, via)
		}
	}
	for _, link := range links {
		if linkDest, ok := r.below(root, link, dest); ok {
			r.followLink(o, root, link, linkDest, depth)
		}
	}
}

// followLink mounts the target of link, a symlink in the tree rooted at
// treeRoot, at dest. A target outside treeRoot must pass the link policy, and
// a directory target is mounted with mountTree, so links inside it are
// followed too.
func (r *resolution) followLink(o owner, treeRoot, link, dest string, depth int) {
	if r.linksFollowed >= maxLinks {
		r.refuse(link, fmt.Sprintf("more than %d symlinks in config; not followed", maxLinks))
		return
	}
	r.linksFollowed++
	target, err := r.fs.EvalSymlinks(link)
	if err != nil {
		r.refuse(link, "dangling or unresolvable symlink")
		return
	}
	info, err := r.fs.Stat(target)
	if err != nil {
		r.refuse(link, "cannot inspect symlink target")
		return
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		r.refuse(link, "symlink target is not a regular file or directory")
		return
	}
	if !isWithin(target, treeRoot) {
		decision := r.policy.Check(target)
		if decision.Verdict != pathpolicy.Allowed {
			r.refuseLink(link, target, decision)
			return
		}
		o.writable = o.writable && !decision.Root.ReadOnly
	}
	if !info.IsDir() {
		r.addMount(o, target, dest, nil, link)
		return
	}
	if isWithin(link, target) {
		r.refuse(link, "symlink points to a directory that contains it")
		return
	}
	if depth >= maxLinkDepth {
		r.refuse(link, "too many nested symlinked directories")
		return
	}
	r.mountTree(o, target, dest, depth+1, link)
}

// followMaskedLink handles a symlink that leaves a config tree mounted whole.
// The link itself is visible in the container, so its target is mounted where
// the link points inside the container.
func (r *resolution) followMaskedLink(o owner, treeRoot, link, treeDest string) {
	text, err := r.fs.Readlink(link)
	if err != nil {
		r.refuse(link, "cannot read symlink")
		return
	}
	linkDest, ok := r.below(treeRoot, link, treeDest)
	if !ok {
		return
	}
	roots := append([]string{r.home, r.resolvedHome}, r.policy.AllowPaths()...)
	dest, ok := containerLinkDest(linkDest, text, roots)
	if !ok {
		r.refuse(link, fmt.Sprintf("symlink to %s cannot be followed inside the container", text))
		return
	}
	r.followLink(o, treeRoot, link, dest, 0)
}

// below maps hostPath, inside root, to the same relative path under dest.
func (r *resolution) below(root, hostPath, dest string) (string, bool) {
	rel, err := filepath.Rel(root, hostPath)
	if err != nil {
		r.refuse(hostPath, "cannot map safe config path")
		return "", false
	}
	if rel == "." {
		return dest, true
	}
	return path.Join(dest, filepath.ToSlash(rel)), true
}

// addMount records a mount of hostPath at dest, a container path, unless the
// same host path is already mounted there. link is the symlink it follows.
func (r *resolution) addMount(o owner, hostPath, dest string, masks []string, link string) {
	key := hostPath + "\x00" + dest
	if _, ok := r.seen[key]; ok {
		return
	}
	r.seen[key] = struct{}{}
	r.report.Mounts = append(r.report.Mounts, plan.ConfigMount{
		Agent:     o.agent,
		Name:      o.name,
		HostPath:  hostPath,
		DestPath:  dest,
		Root:      o.root,
		Writable:  o.writable,
		MaskPaths: masks,
		Link:      link,
	})
}

// dropNestedLinkMounts refuses symlink mounts whose destination lies inside
// another mount: Docker would create the mountpoint inside that host directory.
func (r *resolution) dropNestedLinkMounts() {
	var kept []plan.ConfigMount
	for _, mount := range r.report.Mounts {
		if mount.Link != "" && mount.Link != mount.Root && r.insideOtherMount(mount) {
			r.refuse(mount.Link, "symlink target would be mounted inside another mount at "+mount.DestPath)
			continue
		}
		kept = append(kept, mount)
	}
	r.report.Mounts = kept
}

func (r *resolution) insideOtherMount(mount plan.ConfigMount) bool {
	for _, other := range r.report.Mounts {
		if other.DestPath != mount.DestPath && withinSlash(mount.DestPath, other.DestPath) {
			return true
		}
	}
	return false
}

// refuseLink records a symlink whose target the policy refused, with the hint
// that tells the user how to allow it.
func (r *resolution) refuseLink(link, target string, decision pathpolicy.Decision) {
	isDir := false
	if info, err := r.fs.Stat(target); err == nil {
		isDir = info.IsDir()
	}
	shown := pathpolicy.Tilde(r.resolvedHome, target)
	reason := "symlink target " + shown + " is not under links.allow"
	if decision.Verdict == pathpolicy.Denied {
		reason = "symlink target " + shown + " is denied"
	}
	r.report.Refused = appendUnique(r.report.Refused, plan.Refusal{Path: link, Reason: reason, Hint: r.policy.Remedy(decision, target, isDir)})
}

func (r *resolution) exists(path string) bool {
	_, err := r.fs.Stat(path)
	return err == nil
}

func (r *resolution) refuse(path, reason string) {
	r.report.Refused = appendUnique(r.report.Refused, plan.Refusal{Path: path, Reason: reason})
}

// inContainer returns rel, relative to the container home, as a container path.
func inContainer(rel string) string {
	return path.Join(containerHome, filepath.ToSlash(rel))
}
