package userconfig

import (
	"fmt"
	"path/filepath"
)

// xdgBase returns the value of the XDG base-directory variable key when it is
// an absolute path, the only form that moves a directory; a relative or empty
// value is ignored, as the XDG Base Directory spec says.
func xdgBase(env Env, key string) (string, bool) {
	dir, ok := env.LookupEnv(key)
	return dir, ok && filepath.IsAbs(dir)
}

// baseDir returns the directory the XDG variable key names, or the home
// directory joined with fallback when the variable does not move it.
func baseDir(env Env, key, fallback string) (string, error) {
	if dir, ok := xdgBase(env, key); ok {
		return dir, nil
	}
	home, err := env.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, fallback), nil
}

// stateDir returns caged's state folder, where Approvals keeps the approved
// copies: $XDG_STATE_HOME/caged, or ~/.local/state/caged.
func stateDir(env Env) (string, error) {
	base, err := baseDir(env, "XDG_STATE_HOME", filepath.Join(".local", "state"))
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "caged"), nil
}
