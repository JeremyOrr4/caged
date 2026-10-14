// Package extension finds, reads, and validates the user's image.json, which
// asks for extra apt packages in the runtime image.
package extension

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// FileSystem reads the extension file. system.FS implements it.
type FileSystem interface {
	ReadFile(name string) ([]byte, error)
}

// Env supplies the settings that decide where the file lives. system.Env
// implements it.
type Env interface {
	LookupEnv(key string) (string, bool)
	UserHomeDir() (string, error)
}

// Loader reads the user's image.json.
type Loader struct {
	fs  FileSystem
	env Env
}

// NewLoader returns a Loader that reads through fsys and finds the file with env.
func NewLoader(fsys FileSystem, env Env) *Loader {
	return &Loader{fs: fsys, env: env}
}

// Path returns where image.json lives: $XDG_CONFIG_HOME/caged/image.json when
// XDG_CONFIG_HOME is set to an absolute path, else ~/.config/caged/image.json.
func (l *Loader) Path() (string, error) {
	if xdg, ok := l.env.LookupEnv("XDG_CONFIG_HOME"); ok && filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "caged", "image.json"), nil
	}
	home, err := l.env.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".config", "caged", "image.json"), nil
}

// Source returns Path with the home directory shown as "~", the form used in
// every message about the file.
func (l *Loader) Source() (string, error) {
	path, err := l.Path()
	if err != nil {
		return "", err
	}
	return l.display(path), nil
}

// display shows path with the home directory replaced by "~".
func (l *Loader) display(path string) string {
	home, err := l.env.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if rest, ok := strings.CutPrefix(path, strings.TrimSuffix(home, "/")+"/"); ok {
		return "~/" + rest
	}
	return path
}

// Load reads and validates the file. A missing file, or one that asks for no
// packages, is not an error: Load returns nil.
func (l *Loader) Load() (*plan.Extension, error) {
	path, err := l.Path()
	if err != nil {
		return nil, err
	}
	source := l.display(path)
	data, err := l.fs.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", source, err)
	}
	return Parse(data, source)
}

// Parse validates the contents of an image.json. source names the file in error
// messages. It returns nil when the file asks for no packages.
func Parse(data []byte, source string) (*plan.Extension, error) {
	var file struct {
		Apt []string `json:"apt"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return nil, decodeError(data, source, err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: invalid JSON at line %d: unexpected data after the top-level object", source, lineAt(data, decoder.InputOffset()))
	}
	for _, name := range file.Apt {
		if !validAptName(name) {
			return nil, fmt.Errorf("%s: invalid apt package name %q; names must match ^[a-z0-9][a-z0-9+.-]+$", source, name)
		}
	}
	apt := sortedUnique(file.Apt)
	if len(apt) == 0 {
		return nil, nil
	}
	return &plan.Extension{Source: source, Apt: apt}, nil
}

// decodeError turns a json decoding failure into a message that names the file
// and, where the decoder knows it, the line.
func decodeError(data []byte, source string, err error) error {
	if field, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
		return fmt.Errorf("%s: unknown field %s; only \"apt\" is supported", source, field)
	}
	var syntax *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax):
		return fmt.Errorf("%s: invalid JSON at line %d: %w", source, lineAt(data, syntax.Offset), err)
	case errors.As(err, &typeErr):
		return fmt.Errorf("%s: invalid JSON at line %d: %w", source, lineAt(data, typeErr.Offset), err)
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return fmt.Errorf("%s: invalid JSON at line %d: unexpected end of file", source, lineAt(data, int64(len(data))))
	default:
		return fmt.Errorf("%s: invalid JSON: %w", source, err)
	}
}

// lineAt returns the 1-based line of the byte at offset.
func lineAt(data []byte, offset int64) int {
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	if offset < 0 {
		offset = 0
	}
	return 1 + bytes.Count(data[:offset], []byte("\n"))
}
