// Package userconfig reads the user's caged config file,
// $XDG_CONFIG_HOME/caged/config.yaml or ~/.config/caged/config.yaml, and
// turns it into the image extension and the path policies the rest of caged
// uses. The file is read at most once per Store.
package userconfig

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// FileSystem is the filesystem access the store needs. system.FS implements it.
type FileSystem interface {
	ReadFile(name string) ([]byte, error)
	Stat(name string) (fs.FileInfo, error)
	EvalSymlinks(path string) (string, error)
}

// Env supplies the settings that decide where the file lives. system.Env
// implements it.
type Env interface {
	LookupEnv(key string) (string, bool)
	UserHomeDir() (string, error)
}

// Store reads the config file once and answers every question about it.
type Store struct {
	fs  FileSystem
	env Env

	done bool
	cfg  config
	err  error
}

// NewStore returns a Store that reads through fsys and finds the file with env.
func NewStore(fsys FileSystem, env Env) *Store {
	return &Store{fs: fsys, env: env}
}

// dir is the caged config directory: $XDG_CONFIG_HOME/caged when
// XDG_CONFIG_HOME is an absolute path, else ~/.config/caged.
func (s *Store) dir() (string, error) {
	if xdg, ok := s.env.LookupEnv("XDG_CONFIG_HOME"); ok && filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "caged"), nil
	}
	home, err := s.env.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".config", "caged"), nil
}

// Path returns where config.yaml lives.
func (s *Store) Path() (string, error) {
	dir, err := s.dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Source returns Path with the home directory shown as "~", the form used in
// every message about the file.
func (s *Store) Source() (string, error) {
	path, err := s.Path()
	if err != nil {
		return "", err
	}
	return s.display(path), nil
}

// display shows path with the home directory replaced by "~".
func (s *Store) display(path string) string {
	home, err := s.env.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if rest, ok := strings.CutPrefix(path, strings.TrimSuffix(home, "/")+"/"); ok {
		return "~/" + rest
	}
	return path
}

// load reads and validates the file the first time it is called.
func (s *Store) load() (config, error) {
	if !s.done {
		s.done = true
		s.cfg, s.err = s.read()
	}
	return s.cfg, s.err
}

func (s *Store) read() (config, error) {
	path, err := s.Path()
	if err != nil {
		return config{}, err
	}
	source := s.display(path)
	data, err := s.fs.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return config{}, nil
	}
	if err != nil {
		return config{}, fmt.Errorf("read %s: %w", source, err)
	}
	cfg, err := parse(data, source)
	if err != nil {
		return config{}, err
	}
	cfg.found = true
	return cfg, nil
}

// Load returns the image extension: the image.apt packages, or nil when the
// file is missing or lists none.
func (s *Store) Load() (*plan.Extension, error) {
	cfg, err := s.load()
	if err != nil || len(cfg.apt) == 0 {
		return nil, err
	}
	source, err := s.Source()
	if err != nil {
		return nil, err
	}
	return &plan.Extension{Source: source, Apt: append([]string(nil), cfg.apt...)}, nil
}

// Status summarizes the file for caged doctor. Legacy is set when an old
// image.json is still next to it. The status is returned with the error so
// doctor can still name the file.
func (s *Store) Status() (plan.ConfigStatus, error) {
	source, err := s.Source()
	if err != nil {
		return plan.ConfigStatus{}, err
	}
	status := plan.ConfigStatus{Source: source}
	if dir, err := s.dir(); err == nil {
		legacy := filepath.Join(dir, "image.json")
		if _, err := s.fs.Stat(legacy); err == nil {
			status.Legacy = s.display(legacy)
		}
	}
	cfg, err := s.load()
	if err != nil {
		return status, err
	}
	status.Found = cfg.found
	status.Apt = append([]string(nil), cfg.apt...)
	status.LinkAllow = len(cfg.linkAllow)
	status.Workdirs = cfg.workdirsSet
	status.WorkdirAllow = len(cfg.workdirs)
	return status, nil
}

// LinkPolicy returns the policy for config symlink targets under home: the
// links.allow roots, and the built-in deny list plus links.deny. It is never
// nil when err is nil; with no file, nothing outside a config folder is allowed.
func (s *Store) LinkPolicy(home string) (*pathpolicy.Policy, error) {
	cfg, err := s.load()
	if err != nil {
		return nil, err
	}
	source, err := s.Source()
	if err != nil {
		return nil, err
	}
	home = filepath.Clean(home)
	pc := pathpolicy.Config{Home: home, ResolvedHome: s.resolve(home), Source: source, Field: "links.allow"}
	for _, root := range cfg.linkAllow {
		pc.Allow = append(pc.Allow, s.root(expand(home, root.path), root.readOnly, false))
	}
	for _, path := range pathpolicy.BuiltinDeny(home) {
		pc.Deny = append(pc.Deny, s.root(path, false, true))
	}
	for _, path := range cfg.linkDeny {
		pc.Deny = append(pc.Deny, s.root(expand(home, path), false, false))
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
	source, err := s.Source()
	if err != nil {
		return nil, err
	}
	home = filepath.Clean(home)
	pc := pathpolicy.Config{Home: home, ResolvedHome: s.resolve(home), Source: source, Field: "workdirs.allow"}
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
