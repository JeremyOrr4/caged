package userconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
)

// StateFS is the filesystem access Approvals needs. system.FS implements it.
type StateFS interface {
	ReadFile(name string) ([]byte, error)
	MkdirAllPrivate(dir string) error
	// ReplaceFile atomically replaces name with data.
	ReplaceFile(name string, data []byte, perm fs.FileMode) error
}

// ConfigPath names the config file whose approval is tracked. *Store
// implements it.
type ConfigPath interface {
	Path() (string, error)
}

// Approvals remembers the last config.yaml the user approved, so a change
// made behind their back, for example by an agent editing the mounted config
// directory, can be shown and confirmed. The copy lives under
// $XDG_STATE_HOME/caged (default ~/.local/state/caged), which is never
// mounted into a container; Store.LinkPolicy puts it on the built-in deny
// list, so agent config cannot link into it. Each config path has its own
// copy.
type Approvals struct {
	fs     StateFS
	env    Env
	config ConfigPath
}

// NewApprovals returns the Approvals for the file that config names.
func NewApprovals(fsys StateFS, env Env, config ConfigPath) *Approvals {
	return &Approvals{fs: fsys, env: env, config: config}
}

// path is where the approved copy lives.
func (a *Approvals) path() (string, error) {
	config, err := a.config.Path()
	if err != nil {
		return "", err
	}
	dir, err := stateDir(a.env)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(filepath.Clean(config)))
	return filepath.Join(dir, "config-"+hex.EncodeToString(sum[:6])+".approved.yaml"), nil
}

// Approved returns the last approved content, and false when nothing has been
// approved yet.
func (a *Approvals) Approved() ([]byte, bool, error) {
	path, err := a.path()
	if err != nil {
		return nil, false, err
	}
	data, err := a.fs.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read approved config copy: %w", err)
	}
	return data, true, nil
}

// Approve records data as the approved content.
func (a *Approvals) Approve(data []byte) error {
	path, err := a.path()
	if err != nil {
		return err
	}
	err = a.fs.MkdirAllPrivate(filepath.Dir(path))
	if err == nil {
		err = a.fs.ReplaceFile(path, data, 0o600)
	}
	if err != nil {
		return fmt.Errorf("record approved config: %w", err)
	}
	return nil
}
