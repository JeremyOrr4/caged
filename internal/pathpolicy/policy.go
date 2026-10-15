// Package pathpolicy decides whether a resolved host path may be mounted into
// the container. It does no IO: callers resolve symlinks before they ask.
package pathpolicy

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Root is one configured path of an allow or deny list.
type Root struct {
	Path     string   // absolute and clean, after ~ expansion
	Forms    []string // Path and, when it differs, its symlink-resolved form
	ReadOnly bool     // mounts allowed by this root are read-only
	Builtin  bool     // from caged's built-in deny list
}

// Config is everything New needs.
type Config struct {
	Home, ResolvedHome string // the home directory before and after symlink resolution
	Source             string // the config file, shown with ~, for messages
	Field              string // the YAML key of Allow, e.g. "links.allow"
	Allow, Deny        []Root
}

// Verdict is the outcome of Check.
type Verdict int

// The verdicts of Check.
const (
	NotAllowed Verdict = iota // under no allow root
	Allowed
	Denied // inside, or containing, a deny root
)

// Decision is a verdict and the root that decided it. Root is the zero value
// when the verdict is NotAllowed.
type Decision struct {
	Verdict Verdict
	Root    Root
}

// Policy is an immutable allow and deny list.
type Policy struct{ cfg Config }

// New validates cfg and returns its policy. An allow root must be absolute,
// must not contain the home directory (so "/" and "~" are refused as too
// broad), and must not lie inside a deny root.
func New(cfg Config) (*Policy, error) {
	cfg.Home = filepath.Clean(cfg.Home)
	if cfg.ResolvedHome == "" {
		cfg.ResolvedHome = cfg.Home
	}
	cfg.ResolvedHome = filepath.Clean(cfg.ResolvedHome)
	cfg.Allow, cfg.Deny = normalize(cfg.Allow), normalize(cfg.Deny)
	p := &Policy{cfg: cfg}
	for _, root := range cfg.Allow {
		for _, form := range root.Forms {
			if !filepath.IsAbs(form) {
				return nil, fmt.Errorf("%s: %s entry %q must be an absolute path or start with ~/", cfg.Source, cfg.Field, root.Path)
			}
			if p.tooBroad(form) {
				return nil, fmt.Errorf("%s: %s entry %q is too broad: it contains the home directory", cfg.Source, cfg.Field, p.tilde(root.Path))
			}
			for _, deny := range cfg.Deny {
				for _, denyForm := range deny.Forms {
					if Within(form, denyForm) {
						return nil, fmt.Errorf("%s: %s entry %q is inside the denied path %q", cfg.Source, cfg.Field, p.tilde(root.Path), p.tilde(deny.Path))
					}
				}
			}
		}
	}
	return p, nil
}

// normalize cleans every root and makes Forms start with Path, without duplicates.
func normalize(roots []Root) []Root {
	out := make([]Root, 0, len(roots))
	for _, root := range roots {
		root.Path = filepath.Clean(root.Path)
		forms := []string{root.Path}
		for _, form := range root.Forms {
			form = filepath.Clean(form)
			duplicate := false
			for _, existing := range forms {
				duplicate = duplicate || existing == form
			}
			if !duplicate {
				forms = append(forms, form)
			}
		}
		root.Forms = forms
		out = append(out, root)
	}
	return out
}

// Check decides whether resolved, a symlink-free host path, may be mounted.
// lexical are other names of the same path that are checked against the deny
// list only. A deny root wins when a candidate is inside it or when it is
// inside a candidate (a directory that contains ~/.ssh is denied too). Allow
// roots match the resolved path only.
func (p *Policy) Check(resolved string, lexical ...string) Decision {
	candidates := []string{filepath.Clean(resolved)}
	for _, name := range lexical {
		candidates = append(candidates, filepath.Clean(name))
	}
	for _, deny := range p.cfg.Deny {
		for _, form := range deny.Forms {
			for _, candidate := range candidates {
				if Within(candidate, form) || Within(form, candidate) {
					return Decision{Verdict: Denied, Root: deny}
				}
			}
		}
	}
	for _, allow := range p.cfg.Allow {
		for _, form := range allow.Forms {
			if Within(candidates[0], form) {
				return Decision{Verdict: Allowed, Root: allow}
			}
		}
	}
	return Decision{Verdict: NotAllowed}
}

// Remedy returns the one line that tells the user how to act on a refusal of
// target, or "" for an allowed decision. For a file the suggestion is its
// directory, unless that directory would be too broad.
func (p *Policy) Remedy(d Decision, target string, isDir bool) string {
	switch d.Verdict {
	case Denied:
		if d.Root.Builtin {
			return fmt.Sprintf("%s is on caged's built-in deny list and cannot be allowed", p.tilde(d.Root.Path))
		}
		return fmt.Sprintf("%s is denied by links.deny in %s", p.tilde(d.Root.Path), p.cfg.Source)
	case NotAllowed:
		suggest := filepath.Clean(target)
		if !isDir {
			if dir := filepath.Dir(suggest); !p.tooBroad(dir) {
				suggest = dir
			}
		}
		return fmt.Sprintf("add \"- path: %s\" under %s in %s", p.tilde(suggest), p.cfg.Field, p.cfg.Source)
	default:
		return ""
	}
}

// AllowPaths returns every form of every allow root.
func (p *Policy) AllowPaths() []string {
	var paths []string
	for _, root := range p.cfg.Allow {
		paths = append(paths, root.Forms...)
	}
	return paths
}

// tooBroad reports whether path is the home directory or one of its ancestors.
func (p *Policy) tooBroad(path string) bool {
	return Within(p.cfg.Home, path) || Within(p.cfg.ResolvedHome, path)
}

// tilde shows path with the home directory, in either form, as "~".
func (p *Policy) tilde(path string) string {
	if shown := Tilde(p.cfg.Home, path); shown != path {
		return shown
	}
	return Tilde(p.cfg.ResolvedHome, path)
}

// Tilde shows path with home replaced by "~".
func Tilde(home, path string) string {
	home, path = filepath.Clean(home), filepath.Clean(path)
	if path == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~/" + filepath.ToSlash(rest)
	}
	return path
}

// Within reports whether path is root itself or inside it.
func Within(path, root string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
