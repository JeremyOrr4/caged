package extension

import (
	"errors"
	"io/fs"
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
)

type fakeFS struct {
	files map[string][]byte
	err   error // returned for every read when set
	read  []string
}

func (f *fakeFS) ReadFile(name string) ([]byte, error) {
	f.read = append(f.read, name)
	if f.err != nil {
		return nil, f.err
	}
	data, ok := f.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return data, nil
}

type fakeEnv struct {
	vars    map[string]string
	home    string
	homeErr error
}

func (e fakeEnv) LookupEnv(key string) (string, bool) { v, ok := e.vars[key]; return v, ok }
func (e fakeEnv) UserHomeDir() (string, error)        { return e.home, e.homeErr }

func TestLoaderPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		env         fakeEnv
		want        string
		wantDisplay string
		wantErr     string
	}{
		{name: "default", env: fakeEnv{home: "/home/me"}, want: "/home/me/.config/caged/image.json", wantDisplay: "~/.config/caged/image.json"},
		{name: "XDG_CONFIG_HOME wins when absolute", env: fakeEnv{home: "/home/me", vars: map[string]string{"XDG_CONFIG_HOME": "/xdg"}}, want: "/xdg/caged/image.json", wantDisplay: "/xdg/caged/image.json"},
		{name: "XDG_CONFIG_HOME under the home directory is shown with ~", env: fakeEnv{home: "/home/me", vars: map[string]string{"XDG_CONFIG_HOME": "/home/me/cfg"}}, want: "/home/me/cfg/caged/image.json", wantDisplay: "~/cfg/caged/image.json"},
		{name: "relative XDG_CONFIG_HOME is ignored", env: fakeEnv{home: "/home/me", vars: map[string]string{"XDG_CONFIG_HOME": "cfg"}}, want: "/home/me/.config/caged/image.json", wantDisplay: "~/.config/caged/image.json"},
		{name: "empty XDG_CONFIG_HOME is ignored", env: fakeEnv{home: "/home/me", vars: map[string]string{"XDG_CONFIG_HOME": ""}}, want: "/home/me/.config/caged/image.json", wantDisplay: "~/.config/caged/image.json"},
		{name: "no home directory", env: fakeEnv{homeErr: errors.New("unset")}, wantErr: "find home directory: unset"},
		{name: "XDG needs no home directory", env: fakeEnv{homeErr: errors.New("unset"), vars: map[string]string{"XDG_CONFIG_HOME": "/xdg"}}, want: "/xdg/caged/image.json", wantDisplay: "/xdg/caged/image.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			loader := NewLoader(&fakeFS{}, tt.env)
			got, err := loader.Path()
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("Path() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("Path() = %q, %v; want %q", got, err, tt.want)
			}
			if display, err := loader.Source(); err != nil || display != tt.wantDisplay {
				t.Fatalf("Source() = %q, %v; want %q", display, err, tt.wantDisplay)
			}
		})
	}
}

func TestLoaderLoad(t *testing.T) {
	t.Parallel()

	const path = "/home/me/.config/caged/image.json"
	const source = "~/.config/caged/image.json"
	env := fakeEnv{home: "/home/me"}

	t.Run("a missing file is not an error", func(t *testing.T) {
		t.Parallel()
		got, err := NewLoader(&fakeFS{}, env).Load()
		if got != nil || err != nil {
			t.Fatalf("Load() = %+v, %v; want nil, nil", got, err)
		}
	})
	t.Run("reads the file at Path and shows it with ~", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{files: map[string][]byte{path: []byte(`{"apt":["tmux","neovim"]}`)}}
		got, err := NewLoader(fsys, env).Load()
		want := &plan.Extension{Source: source, Apt: []string{"neovim", "tmux"}}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("Load() = %+v, %v; want %+v", got, err, want)
		}
		if !reflect.DeepEqual(fsys.read, []string{path}) {
			t.Fatalf("read %v, want only %s", fsys.read, path)
		}
	})
	t.Run("a read error names the file", func(t *testing.T) {
		t.Parallel()
		_, err := NewLoader(&fakeFS{err: errors.New("permission denied")}, env).Load()
		if err == nil || err.Error() != "read ~/.config/caged/image.json: permission denied" {
			t.Fatalf("Load() error = %v", err)
		}
	})
	t.Run("a validation error names the file", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{files: map[string][]byte{path: []byte(`{"npm":[]}`)}}
		_, err := NewLoader(fsys, env).Load()
		if err == nil || err.Error() != `~/.config/caged/image.json: unknown field "npm"; only "apt" is supported` {
			t.Fatalf("Load() error = %v", err)
		}
	})
	t.Run("no home directory is an error", func(t *testing.T) {
		t.Parallel()
		_, err := NewLoader(&fakeFS{}, fakeEnv{homeErr: errors.New("unset")}).Load()
		if err == nil || err.Error() != "find home directory: unset" {
			t.Fatalf("Load() error = %v", err)
		}
	})
}

func TestParse(t *testing.T) {
	t.Parallel()

	const src = "~/.config/caged/image.json"
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr string
	}{
		{name: "packages", input: `{"apt": ["neovim", "tmux", "shellcheck"]}`, want: []string{"neovim", "shellcheck", "tmux"}},
		{name: "reordering gives the same list", input: `{"apt": ["tmux", "shellcheck", "neovim"]}`, want: []string{"neovim", "shellcheck", "tmux"}},
		{name: "duplicates are removed", input: `{"apt": ["tmux", "neovim", "tmux", "tmux"]}`, want: []string{"neovim", "tmux"}},
		{name: "all the allowed characters", input: `{"apt": ["g++", "libstdc++-13-dev", "python3.12", "0ad", "a-b.c+d"]}`, want: []string{"0ad", "a-b.c+d", "g++", "libstdc++-13-dev", "python3.12"}},
		{name: "surrounding whitespace is fine", input: "\n {\n \"apt\": [\"vim\"]\n }\n\n", want: []string{"vim"}},

		{name: "empty object", input: `{}`, want: nil},
		{name: "empty list", input: `{"apt": []}`, want: nil},
		{name: "null list", input: `{"apt": null}`, want: nil},

		{name: "invalid JSON reports the line", input: "{\n  \"apt\": [\n    \"vim\",\n  ]\n}", wantErr: src + `: invalid JSON at line 4: invalid character ']' looking for beginning of value`},
		{name: "invalid JSON on the first line", input: `{"apt": [}`, wantErr: src + `: invalid JSON at line 1: invalid character '}' looking for beginning of value`},
		{name: "truncated file", input: "{\n\"apt\": [\"vim\"", wantErr: src + ": invalid JSON at line 2: unexpected end of file"},
		{name: "empty file", input: ``, wantErr: src + ": invalid JSON at line 1: unexpected end of file"},
		{name: "not an object", input: "[\"vim\"]", wantErr: src + ": invalid JSON at line 1: json: cannot unmarshal array into Go value of type struct { Apt []string \"json:\\\"apt\\\"\" }"},
		{name: "apt is not a list", input: `{"apt": "vim"}`, wantErr: src + ": invalid JSON at line 1: json: cannot unmarshal string into Go struct field .apt of type []string"},
		{name: "data after the object", input: "{}\n{}", wantErr: src + ": invalid JSON at line 2: unexpected data after the top-level object"},

		{name: "unknown key", input: `{"apt": ["vim"], "npm": ["left-pad"]}`, wantErr: src + `: unknown field "npm"; only "apt" is supported`},

		{name: "shell syntax", input: `{"apt": ["vim; curl x"]}`, wantErr: src + `: invalid apt package name "vim; curl x"; names must match ^[a-z0-9][a-z0-9+.-]+$`},
		{name: "version pin", input: `{"apt": ["vim=2:9.1"]}`, wantErr: src + `: invalid apt package name "vim=2:9.1"; names must match ^[a-z0-9][a-z0-9+.-]+$`},
		{name: "architecture suffix", input: `{"apt": ["vim:arm64"]}`, wantErr: src + `: invalid apt package name "vim:arm64"; names must match ^[a-z0-9][a-z0-9+.-]+$`},
		{name: "uppercase", input: `{"apt": ["Vim"]}`, wantErr: src + `: invalid apt package name "Vim"; names must match ^[a-z0-9][a-z0-9+.-]+$`},
		{name: "one character", input: `{"apt": ["a"]}`, wantErr: src + `: invalid apt package name "a"; names must match ^[a-z0-9][a-z0-9+.-]+$`},
		{name: "empty name", input: `{"apt": [""]}`, wantErr: src + `: invalid apt package name ""; names must match ^[a-z0-9][a-z0-9+.-]+$`},
		{name: "leading dash", input: `{"apt": ["-y"]}`, wantErr: src + `: invalid apt package name "-y"; names must match ^[a-z0-9][a-z0-9+.-]+$`},
		{name: "leading dot", input: `{"apt": [".x"]}`, wantErr: src + `: invalid apt package name ".x"; names must match ^[a-z0-9][a-z0-9+.-]+$`},
		{name: "space", input: `{"apt": ["vim tmux"]}`, wantErr: src + `: invalid apt package name "vim tmux"; names must match ^[a-z0-9][a-z0-9+.-]+$`},
		{name: "dollar", input: `{"apt": ["$(id)"]}`, wantErr: src + `: invalid apt package name "$(id)"; names must match ^[a-z0-9][a-z0-9+.-]+$`},
		{name: "the first bad name is reported", input: `{"apt": ["vim", "BAD", "also bad"]}`, wantErr: src + `: invalid apt package name "BAD"; names must match ^[a-z0-9][a-z0-9+.-]+$`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse([]byte(tt.input), src)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("Parse() error = %v, want %q", err, tt.wantErr)
				}
				if got != nil {
					t.Fatalf("Parse() = %+v with an error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.want == nil {
				if got != nil {
					t.Fatalf("Parse() = %+v, want nil", got)
				}
				return
			}
			if want := (&plan.Extension{Source: src, Apt: tt.want}); !reflect.DeepEqual(got, want) {
				t.Fatalf("Parse() = %+v, want %+v", got, want)
			}
		})
	}
}
