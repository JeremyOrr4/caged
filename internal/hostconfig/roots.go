package hostconfig

import (
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/plan"
)

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
	staged      bool               // mounted as staged trees (agent.ConfigPath.Staged)
	policy      *pathpolicy.Policy // decides which symlink targets are followed
}

// agentPath checks one agent config path and records its mounts, or why it
// is missing or refused.
func (r *resolution) agentPath(agentName string, configPath agent.ConfigPath, hostPath string) {
	resolvedHost, ok := r.resolveRoot(hostPath)
	if !ok {
		return
	}
	if filepath.Clean(resolvedHost) == filepath.Clean(r.resolvedHome) {
		r.refuse(hostPath, "config path resolves to the home directory")
		return
	}
	o := owner{agent: agentName, name: filepath.Clean(configPath.HostPath), root: hostPath, writable: configPath.Writable, staged: configPath.Staged, policy: r.policy}
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
		r.addMount(o, resolvedHost, dest, append([]string(nil), configPath.MaskPaths...), nil, via)
		for _, link := range links {
			r.followMaskedLink(o, resolvedHost, link, dest)
		}
		return
	}
	r.mountTree(o, resolvedHost, dest, 0, via)
}

// cagedDir records the mount of caged's own config directory, read/write, at
// the container's ~/.config/caged. Unlike agent config it may live outside the
// home directory (an absolute XDG_CONFIG_HOME). It must not be the home
// directory, as no config root may, and the built-in deny list applies to it
// even when it is not a symlink. Reached through a symlink it is judged like
// any other link target: it needs a links.allow root, and read_only roots make
// it read-only. Its own built-in
// deny entries, ~/.config/caged and dir, are set aside for this one path;
// every other deny root still applies, and the entries still refuse links
// from agent config into the directory. It must not be, hold or sit inside
// state, the folder of the approved copies, which setting the entries aside
// would otherwise let it mount.
func (r *resolution) cagedDir(dir, state string) {
	resolved, ok := r.resolveRoot(dir)
	if !ok {
		return
	}
	if filepath.Clean(resolved) == filepath.Clean(r.resolvedHome) {
		r.refuse(dir, "config path resolves to the home directory")
		return
	}
	if r.overlaps([]string{dir, resolved}, state) {
		r.report.Refused = appendUnique(r.report.Refused, plan.Refusal{Path: dir, Reason: "config path overlaps caged's approval state folder " + pathpolicy.Tilde(r.home, state), Hint: "set XDG_CONFIG_HOME and XDG_STATE_HOME to different folders"})
		return
	}
	// Without matches an entry's Path, Check every form of it: a Stow-linked
	// ~/.config/caged denies its target through the default entry even when
	// XDG_CONFIG_HOME moved dir, so both entries are set aside.
	own := r.policy.Without(filepath.Join(r.home, ".config", "caged")).Without(dir)
	decision := own.Check(resolved)
	o := owner{agent: plan.SharedOwner, name: "caged", root: dir, writable: true, policy: r.policy}
	via := ""
	switch {
	case r.linked(dir, resolved):
		if decision.Verdict != pathpolicy.Allowed {
			r.refuseLink(dir, resolved, decision)
			return
		}
		o.policy, o.writable, via = own, !decision.Root.ReadOnly, dir
	case r.builtinDenied(dir, resolved, decision):
		return
	}
	r.mountTree(o, resolved, inContainer(".config/caged"), 0, via)
}

// overlaps reports whether any of paths is dir, holds it or sits inside it,
// comparing dir both as named and with its symlinks resolved, and names as
// the link policy's file system does.
func (r *resolution) overlaps(paths []string, dir string) bool {
	forms := []string{dir}
	if resolved, err := r.fs.EvalSymlinks(dir); err == nil {
		forms = append(forms, resolved)
	}
	for _, path := range paths {
		for _, form := range forms {
			if r.policy.Overlap(path, form) {
				return true
			}
		}
	}
	return false
}

// sharedFile checks one shared file. It must be a regular file, inside home
// or reached through an allowed symlink, and it is always mounted read-only.
func (r *resolution) sharedFile(file SharedFile) {
	name := filepath.Clean(file.path)
	hostPath := filepath.Join(r.home, name)
	resolved, ok := r.resolveRoot(hostPath)
	if !ok {
		return
	}
	o := owner{agent: plan.SharedOwner, name: name, root: hostPath, writable: false, policy: r.policy}
	via, ok := r.checkRoot(hostPath, resolved, &o)
	if !ok {
		return
	}
	if info, err := r.fs.Stat(resolved); err != nil || !info.Mode().IsRegular() {
		r.refuse(hostPath, file.description+" must be a regular file")
		return
	}
	r.addMount(o, resolved, inContainer(name), nil, nil, via)
}

// checkRoot applies the link policy to a config root reached through a
// symlink, and to one that is not the inside-home rule and the built-in deny
// list. It returns the link to record on the mounts ("" for a direct path)
// and false when the root is refused.
func (r *resolution) checkRoot(hostPath, resolved string, o *owner) (string, bool) {
	if !r.linked(hostPath, resolved) {
		if !pathpolicy.Within(resolved, r.resolvedHome) {
			r.refuse(hostPath, "resolves outside home")
			return "", false
		}
		if r.builtinDenied(hostPath, resolved, o.policy.Check(resolved)) {
			return "", false
		}
		return "", true
	}
	decision := o.policy.Check(resolved)
	if decision.Verdict != pathpolicy.Allowed {
		r.refuseLink(hostPath, resolved, decision)
		return "", false
	}
	o.writable = o.writable && !decision.Root.ReadOnly
	return hostPath, true
}

// builtinDenied refuses hostPath, a config root that is not a symlink, and
// reports true when decision denies it by a built-in root. links.deny is
// about links, so its roots do not refuse such a root.
func (r *resolution) builtinDenied(hostPath, resolved string, decision pathpolicy.Decision) bool {
	if decision.Verdict != pathpolicy.Denied || !decision.Root.Builtin {
		return false
	}
	r.report.Refused = appendUnique(r.report.Refused, plan.Refusal{Path: hostPath, Reason: "config path is denied", Hint: r.policy.Remedy(decision, resolved, true)})
	return true
}

// linked reports whether hostPath reaches resolved through a symlink: in any
// component below home, or anywhere when hostPath is outside home.
func (r *resolution) linked(hostPath, resolved string) bool {
	expected := filepath.Clean(hostPath)
	if pathpolicy.Within(hostPath, r.home) {
		if rel, err := filepath.Rel(r.home, hostPath); err == nil {
			expected = filepath.Join(r.resolvedHome, rel)
		}
	} else if parent, err := r.fs.EvalSymlinks(filepath.Dir(hostPath)); err == nil {
		// Canonicalize the parent so platform aliases such as macOS's /var ->
		// /private/var are not mistaken for a symlink at the config path.
		expected = filepath.Join(parent, filepath.Base(hostPath))
	}
	return filepath.Clean(resolved) != expected
}

// resolveRoot resolves a config root, recording it as missing when it does
// not exist and as refused when its symlinks cannot be resolved.
func (r *resolution) resolveRoot(hostPath string) (string, bool) {
	if _, err := r.fs.Stat(hostPath); err != nil {
		r.report.Missing = appendUnique(r.report.Missing, hostPath)
		return "", false
	}
	resolved, err := r.fs.EvalSymlinks(hostPath)
	if err != nil {
		r.refuse(hostPath, "unresolvable symlink")
		return "", false
	}
	return resolved, true
}

func (r *resolution) refuse(path, reason string) {
	r.report.Refused = appendUnique(r.report.Refused, plan.Refusal{Path: path, Reason: reason})
}
