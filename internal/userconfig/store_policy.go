package userconfig

import (
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/pathpolicy"
)

// LinkPolicy returns the policy for config symlink targets under home: the
// links.allow roots, and the built-in deny list plus links.deny. The built-in
// list also holds the folder Dir mounts and the folder of the approved
// copies, wherever the XDG variables put them, so an agent can reach neither
// through a link. It is never nil when err is nil; with no file, nothing
// outside a config folder is allowed.
func (s *Store) LinkPolicy(home string) (*pathpolicy.Policy, error) {
	cfg, err := s.load()
	if err != nil {
		return nil, err
	}
	dir, err := s.Dir()
	if err != nil {
		return nil, err
	}
	state, err := stateDir(s.env)
	if err != nil {
		return nil, err
	}
	home = filepath.Clean(home)
	pc := pathpolicy.Config{Home: home, ResolvedHome: s.resolve(home), Source: cfg.source, Field: "links.allow"}
	for _, root := range cfg.linkAllow {
		pc.Allow = append(pc.Allow, s.root(expand(home, root.path), root.readOnly, false))
	}
	for _, path := range pathpolicy.BuiltinDeny(home, dir, state) {
		pc.Deny = append(pc.Deny, s.denyRoot(path, true))
	}
	for _, path := range cfg.linkDeny {
		pc.Deny = append(pc.Deny, s.denyRoot(expand(home, path), false))
	}
	return pathpolicy.New(pc)
}

// WorkdirPolicy returns the policy for project directories, or nil when the
// file has no workdirs key and any directory may be mounted.
func (s *Store) WorkdirPolicy(home string) (*pathpolicy.Policy, error) {
	cfg, err := s.load()
	if err != nil || !cfg.workdirsSet {
		return nil, err
	}
	home = filepath.Clean(home)
	pc := pathpolicy.Config{Home: home, ResolvedHome: s.resolve(home), Source: cfg.source, Field: "workdirs.allow"}
	for _, path := range cfg.workdirs {
		pc.Allow = append(pc.Allow, s.root(expand(home, path), false, false))
	}
	return pathpolicy.New(pc)
}

// resolve returns path with symlinks resolved, or path when it cannot be.
func (s *Store) resolve(path string) string {
	if resolved, err := s.fs.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func (s *Store) root(path string, readOnly, builtin bool) pathpolicy.Root {
	return pathpolicy.Root{Path: path, Forms: []string{path, s.resolve(path)}, ReadOnly: readOnly, Builtin: builtin}
}

// denyRoot is root for a deny entry, with one more form: a path that does not
// exist yet has its deepest existing parent resolved, so a symlink above it
// (a symlinked home, a Stow-linked ~/.local) does not hide it from a folder
// that holds it, which could then create it.
func (s *Store) denyRoot(path string, builtin bool) pathpolicy.Root {
	root := s.root(path, false, builtin)
	rest := ""
	for dir := path; ; dir = filepath.Dir(dir) {
		if resolved, err := s.fs.EvalSymlinks(dir); err == nil {
			root.Forms = append(root.Forms, filepath.Join(resolved, rest))
			return root
		}
		if parent := filepath.Dir(dir); parent == dir {
			return root
		}
		rest = filepath.Join(filepath.Base(dir), rest)
	}
}
