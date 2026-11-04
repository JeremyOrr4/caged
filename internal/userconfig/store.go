// Package userconfig reads the user's caged config file,
// $XDG_CONFIG_HOME/caged/config.yaml or ~/.config/caged/config.yaml, and
// turns it into the image extension and the path policies the rest of caged
// uses. The file is read once per Store, and again after Ensure.
package userconfig

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// FileSystem is the filesystem access the store needs. system.FS implements it.
type FileSystem interface {
	// ReadRegularFile reads name, following symlinks, and fails without
	// blocking when it is not a regular file, such as a FIFO, or holds more
	// than limit bytes.
	ReadRegularFile(name string, limit int64) ([]byte, error)
	Stat(name string) (fs.FileInfo, error)
	EvalSymlinks(path string) (string, error)
	// MkdirAllPrivate creates dir and its parents with mode 0700, leaving
	// existing directories alone.
	MkdirAllPrivate(dir string) error
	// CreateFile creates name only if nothing is there, all at once; it fails
	// with an error wrapping fs.ErrExist otherwise.
	CreateFile(name string, data []byte, perm fs.FileMode) error
}

// Env supplies the settings that decide where the file lives. system.Env
// implements it.
type Env interface {
	LookupEnv(key string) (string, bool)
	UserHomeDir() (string, error)
}

// Store reads the config file once, and again after Ensure, and answers every
// question about it.
type Store struct {
	fs  FileSystem
	env Env

	done    bool
	cfg     config
	err     error
	created bool // Ensure wrote the starter file, reported by Status
}

// NewStore returns a Store that reads through fsys and finds the file with env.
func NewStore(fsys FileSystem, env Env) *Store {
	return &Store{fs: fsys, env: env}
}

// Dir returns the caged config directory, the one mounted into containers:
// $XDG_CONFIG_HOME/caged when XDG_CONFIG_HOME is an absolute path, else
// ~/.config/caged.
func (s *Store) Dir() (string, error) {
	base, err := baseDir(s.env, "XDG_CONFIG_HOME", ".config")
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "caged"), nil
}

// StateDir returns the folder of the approved copies,
// $XDG_STATE_HOME/caged when XDG_STATE_HOME is an absolute path, else
// ~/.local/state/caged.
func (s *Store) StateDir() (string, error) { return stateDir(s.env) }

// Path returns where config.yaml lives.
func (s *Store) Path() (string, error) {
	dir, err := s.Dir()
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
	return pathpolicy.Tilde(home, path)
}

// maxConfigSize is the largest config.yaml caged reads; a real one is a few
// hundred bytes.
const maxConfigSize = 1 << 20

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
	data, err := s.fs.ReadRegularFile(path, maxConfigSize) // a container could leave a FIFO or a huge file here
	if errors.Is(err, fs.ErrNotExist) {
		return config{source: source}, nil
	}
	if err != nil {
		return config{}, fmt.Errorf("read %s: %w", source, err)
	}
	cfg, err := parse(data, source)
	if err != nil {
		return config{}, err
	}
	cfg.source, cfg.raw, cfg.found = source, data, true
	return cfg, nil
}

// Ensure creates the config directory (mode 0700) and a starter config.yaml
// (mode 0600) when there is no config.yaml, and does nothing otherwise. It
// reports whether it wrote the file. An existing file or symlink is never
// read, replaced, appended to, or re-permissioned, and two processes that race
// to create it both succeed. When the directory cannot be created for lack of
// permission (or on a read-only filesystem), the error has a MissingDir method
// that returns true: nothing exists to mount then. A
// config.yaml that exists but cannot be read, such as a dangling symlink, is
// an error.
func (s *Store) Ensure() (created bool, err error) {
	s.done = false // whatever was read before may be about to change
	path, err := s.Path()
	if err != nil {
		return false, err
	}
	if _, err := s.fs.Stat(path); err == nil {
		return false, nil
	}
	if _, ok := xdgBase(s.env, "XDG_CONFIG_HOME"); !ok {
		// MkdirAll would build a missing home directory too. A home that does
		// not exist is already reported where it is used, so leave it alone.
		if home, err := s.env.UserHomeDir(); err == nil {
			if _, err := s.fs.Stat(home); err != nil {
				return false, nil
			}
		}
	}
	dir := filepath.Dir(path)
	if err := s.fs.MkdirAllPrivate(dir); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			// This user cannot create the folder, so neither can a container
			// running as this user. Anything else (a dangling symlink on the
			// way: "file exists") could still lead where a container writes.
			return false, &missingDirError{s.createError(dir, err)}
		}
		return false, s.createError(dir, err)
	}
	switch err := s.fs.CreateFile(path, []byte(starterConfig), 0o600); {
	case errors.Is(err, fs.ErrExist): // another process created it first, or it cannot be followed
		if _, err := s.fs.Stat(path); err != nil {
			return false, fmt.Errorf("%s exists but cannot be read: %w", s.display(path), err)
		}
		return false, nil
	case err != nil:
		return false, s.createError(path, err)
	}
	s.created = true
	return true, nil
}

// createError explains why Ensure could not create path, and what to do.
func (s *Store) createError(path string, err error) error {
	return fmt.Errorf("create %s: %w (caged writes a starter config there on first use; fix the permissions or set XDG_CONFIG_HOME to a writable directory)", s.display(path), err)
}

// missingDirError is Ensure's error when the config directory could not be
// created.
type missingDirError struct{ err error }

func (e *missingDirError) Error() string { return e.err.Error() }
func (e *missingDirError) Unwrap() error { return e.err }

// MissingDir reports that the config directory does not exist, so no
// container can be given it to write a config.yaml into.
func (*missingDirError) MissingDir() bool { return true }

// Content returns the exact bytes of config.yaml that Load, Status and the
// policies are built from, and whether the file exists. Reviewing these bytes
// reviews what caged will act on, even if the file changes again afterwards.
func (s *Store) Content() ([]byte, bool, error) {
	cfg, err := s.load()
	if err != nil {
		return nil, false, err
	}
	return cfg.raw, cfg.found, nil
}

// Load returns the image extension: the image.apt packages, or nil when the
// file is missing or lists none.
func (s *Store) Load() (*plan.Extension, error) {
	cfg, err := s.load()
	if err != nil || len(cfg.apt) == 0 {
		return nil, err
	}
	return &plan.Extension{Source: cfg.source, Apt: append([]string(nil), cfg.apt...)}, nil
}

// Status summarizes the file for caged doctor. Legacy is set when an old
// image.json is still next to it. The status is returned with the error so
// doctor can still name the file.
func (s *Store) Status() (plan.ConfigStatus, error) {
	path, err := s.Path()
	if err != nil {
		return plan.ConfigStatus{}, err
	}
	status := plan.ConfigStatus{Source: s.display(path), Created: s.created}
	legacy := filepath.Join(filepath.Dir(path), "image.json")
	if _, err := s.fs.Stat(legacy); err == nil {
		status.Legacy = s.display(legacy)
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
