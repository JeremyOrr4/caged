package userconfig

import (
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// fakeFS is an in-memory filesystem: a map of path to content.
type fakeFS struct {
	files map[string]string
	err   error // returned for every read when set
	reads int
	read  []string
}

func (f *fakeFS) ReadFile(name string) ([]byte, error) {
	f.reads++
	f.read = append(f.read, name)
	if f.err != nil {
		return nil, f.err
	}
	data, ok := f.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return []byte(data), nil
}

func (f *fakeFS) Stat(name string) (fs.FileInfo, error) {
	if _, ok := f.files[name]; ok {
		return nil, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

func (f *fakeFS) EvalSymlinks(path string) (string, error) { return path, nil }

type fakeEnv struct {
	vars    map[string]string
	home    string
	homeErr error
}

func (e fakeEnv) LookupEnv(key string) (string, bool) { v, ok := e.vars[key]; return v, ok }
func (e fakeEnv) UserHomeDir() (string, error)        { return e.home, e.homeErr }

const (
	testPath   = "/home/me/.config/caged/config.yaml"
	testSource = "~/.config/caged/config.yaml"
)

func storeWith(body string) (*Store, *fakeFS) {
	fsys := &fakeFS{files: map[string]string{testPath: body}}
	return NewStore(fsys, fakeEnv{home: "/home/me"}), fsys
}

func TestStorePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		env         fakeEnv
		want        string
		wantDisplay string
		wantErr     string
	}{
		{name: "default", env: fakeEnv{home: "/home/me"}, want: "/home/me/.config/caged/config.yaml", wantDisplay: "~/.config/caged/config.yaml"},
		{name: "XDG_CONFIG_HOME wins when absolute", env: fakeEnv{home: "/home/me", vars: map[string]string{"XDG_CONFIG_HOME": "/xdg"}}, want: "/xdg/caged/config.yaml", wantDisplay: "/xdg/caged/config.yaml"},
		{name: "XDG_CONFIG_HOME under the home directory is shown with ~", env: fakeEnv{home: "/home/me", vars: map[string]string{"XDG_CONFIG_HOME": "/home/me/cfg"}}, want: "/home/me/cfg/caged/config.yaml", wantDisplay: "~/cfg/caged/config.yaml"},
		{name: "relative XDG_CONFIG_HOME is ignored", env: fakeEnv{home: "/home/me", vars: map[string]string{"XDG_CONFIG_HOME": "cfg"}}, want: "/home/me/.config/caged/config.yaml", wantDisplay: "~/.config/caged/config.yaml"},
		{name: "empty XDG_CONFIG_HOME is ignored", env: fakeEnv{home: "/home/me", vars: map[string]string{"XDG_CONFIG_HOME": ""}}, want: "/home/me/.config/caged/config.yaml", wantDisplay: "~/.config/caged/config.yaml"},
		{name: "no home directory", env: fakeEnv{homeErr: errors.New("unset")}, wantErr: "find home directory: unset"},
		{name: "XDG needs no home directory", env: fakeEnv{homeErr: errors.New("unset"), vars: map[string]string{"XDG_CONFIG_HOME": "/xdg"}}, want: "/xdg/caged/config.yaml", wantDisplay: "/xdg/caged/config.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := NewStore(&fakeFS{}, tt.env)
			got, err := store.Path()
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("Path() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("Path() = %q, %v; want %q", got, err, tt.want)
			}
			if display, err := store.Source(); err != nil || display != tt.wantDisplay {
				t.Fatalf("Source() = %q, %v; want %q", display, err, tt.wantDisplay)
			}
		})
	}
}

func TestStoreLoad(t *testing.T) {
	t.Parallel()

	t.Run("a missing file is not an error", func(t *testing.T) {
		t.Parallel()
		store := NewStore(&fakeFS{}, fakeEnv{home: "/home/me"})
		got, err := store.Load()
		if got != nil || err != nil {
			t.Fatalf("Load() = %+v, %v; want nil, nil", got, err)
		}
		status, err := store.Status()
		if err != nil || status.Found || status.Source != testSource {
			t.Fatalf("Status() = %+v, %v; want a missing file named %s", status, err, testSource)
		}
	})
	t.Run("reads the file at Path and shows it with ~", func(t *testing.T) {
		t.Parallel()
		store, fsys := storeWith("image:\n  apt: [tmux, neovim, tmux]\n")
		got, err := store.Load()
		want := &plan.Extension{Source: testSource, Apt: []string{"neovim", "tmux"}}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("Load() = %+v, %v; want %+v", got, err, want)
		}
		if !reflect.DeepEqual(fsys.read, []string{testPath}) {
			t.Fatalf("read %v, want only %s", fsys.read, testPath)
		}
	})
	t.Run("an empty file is valid and found", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("")
		got, err := store.Load()
		if got != nil || err != nil {
			t.Fatalf("Load() = %+v, %v; want nil, nil", got, err)
		}
		if status, err := store.Status(); err != nil || !status.Found {
			t.Fatalf("Status() = %+v, %v; want Found", status, err)
		}
	})
	t.Run("a comments-only file is valid and found", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("# nothing yet\n")
		got, err := store.Load()
		if got != nil || err != nil {
			t.Fatalf("Load() = %+v, %v; want nil, nil", got, err)
		}
		if status, err := store.Status(); err != nil || !status.Found {
			t.Fatalf("Status() = %+v, %v; want Found", status, err)
		}
	})
	t.Run("a read error names the file", func(t *testing.T) {
		t.Parallel()
		store := NewStore(&fakeFS{err: errors.New("permission denied")}, fakeEnv{home: "/home/me"})
		_, err := store.Load()
		if err == nil || err.Error() != "read ~/.config/caged/config.yaml: permission denied" {
			t.Fatalf("Load() error = %v", err)
		}
	})
	t.Run("an invalid apt name names the file and the key", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("image:\n  apt: [\"vim; curl x\"]\n")
		_, err := store.Load()
		if err == nil || !strings.Contains(err.Error(), `invalid apt package name "vim; curl x" in image.apt`) {
			t.Fatalf("Load() error = %v", err)
		}
	})
	t.Run("an unknown field names the file and the line", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("imgae:\n  apt: [jq]\n")
		_, err := store.Load()
		if err == nil || err.Error() != `~/.config/caged/config.yaml: line 1: unknown field "imgae"` {
			t.Fatalf("Load() error = %v", err)
		}
	})
	t.Run("no home directory is an error", func(t *testing.T) {
		t.Parallel()
		_, err := NewStore(&fakeFS{}, fakeEnv{homeErr: errors.New("unset")}).Load()
		if err == nil || err.Error() != "find home directory: unset" {
			t.Fatalf("Load() error = %v", err)
		}
	})
}

func TestStoreStatus(t *testing.T) {
	t.Parallel()

	t.Run("counts what the file holds", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("image:\n  apt: [tmux]\nlinks:\n  allow:\n    - path: ~/dot\n    - path: ~/dot2\nworkdirs:\n  allow:\n    - path: ~/git\n")
		status, err := store.Status()
		want := plan.ConfigStatus{Source: testSource, Found: true, Apt: []string{"tmux"}, LinkAllow: 2, Workdirs: true, WorkdirAllow: 1}
		if err != nil || !reflect.DeepEqual(status, want) {
			t.Fatalf("Status() = %+v, %v; want %+v", status, err, want)
		}
	})
	t.Run("a leftover image.json is reported as Legacy", func(t *testing.T) {
		t.Parallel()
		store := NewStore(&fakeFS{files: map[string]string{"/home/me/.config/caged/image.json": `{"apt":["jq"]}`}}, fakeEnv{home: "/home/me"})
		status, err := store.Status()
		if err != nil || status.Legacy != "~/.config/caged/image.json" || status.Found {
			t.Fatalf("Status() = %+v, %v; want Legacy ~/.config/caged/image.json and no config.yaml", status, err)
		}
	})
	t.Run("an invalid file still names itself", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("image: [\n")
		status, err := store.Status()
		if err == nil || status.Source != testSource {
			t.Fatalf("Status() = %+v, %v; want an error and the source", status, err)
		}
	})
}

func TestStoreReadsTheFileOnce(t *testing.T) {
	t.Parallel()
	store, fsys := storeWith("image:\n  apt: [tmux]\nworkdirs:\n  allow:\n    - path: ~/git\n")
	if _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Status(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LinkPolicy("/home/me"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.WorkdirPolicy("/home/me"); err != nil {
		t.Fatal(err)
	}
	if fsys.reads != 1 {
		t.Fatalf("ReadFile was called %d times, want 1", fsys.reads)
	}
}

func TestLinkPolicy(t *testing.T) {
	t.Parallel()

	t.Run("without a file only the built-in deny list applies", func(t *testing.T) {
		t.Parallel()
		policy, err := NewStore(&fakeFS{}, fakeEnv{home: "/home/me"}).LinkPolicy("/home/me")
		if err != nil || policy == nil {
			t.Fatalf("LinkPolicy() = %v, %v; want a policy", policy, err)
		}
		if got := policy.Check("/home/me/dot/a").Verdict; got != pathpolicy.NotAllowed {
			t.Fatalf("Check(/home/me/dot/a) = %d, want NotAllowed", got)
		}
		if got := policy.Check("/home/me/.ssh/id").Verdict; got != pathpolicy.Denied {
			t.Fatalf("Check(/home/me/.ssh/id) = %d, want Denied", got)
		}
	})
	t.Run("links.allow and links.deny shape the policy", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("links:\n  allow:\n    - path: ~/dot\n      read_only: true\n  deny: [~/dot/private]\n")
		policy, err := store.LinkPolicy("/home/me")
		if err != nil {
			t.Fatal(err)
		}
		if decision := policy.Check("/home/me/dot/pi"); decision.Verdict != pathpolicy.Allowed || !decision.Root.ReadOnly {
			t.Fatalf("Check(/home/me/dot/pi) = %+v, want Allowed and read-only", decision)
		}
		if got := policy.Check("/home/me/dot/private/x").Verdict; got != pathpolicy.Denied {
			t.Fatalf("Check(/home/me/dot/private/x) = %d, want Denied", got)
		}
	})
	t.Run("an allow root of ~ is too broad", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("links:\n  allow:\n    - path: \"~\"\n")
		_, err := store.LinkPolicy("/home/me")
		if err == nil || !strings.Contains(err.Error(), "too broad") {
			t.Fatalf("LinkPolicy() error = %v, want too broad", err)
		}
	})
	t.Run("an invalid file is an error", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("image: [\n")
		if _, err := store.LinkPolicy("/home/me"); err == nil {
			t.Fatal("LinkPolicy() succeeded on an invalid file")
		}
	})
}

func TestWorkdirPolicy(t *testing.T) {
	t.Parallel()

	t.Run("nil without a workdirs key", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("image:\n  apt: [tmux]\n")
		policy, err := store.WorkdirPolicy("/home/me")
		if policy != nil || err != nil {
			t.Fatalf("WorkdirPolicy() = %v, %v; want nil, nil", policy, err)
		}
	})
	t.Run("nil without a file", func(t *testing.T) {
		t.Parallel()
		policy, err := NewStore(&fakeFS{}, fakeEnv{home: "/home/me"}).WorkdirPolicy("/home/me")
		if policy != nil || err != nil {
			t.Fatalf("WorkdirPolicy() = %v, %v; want nil, nil", policy, err)
		}
	})
	t.Run("an empty allow list refuses everything", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("workdirs:\n  allow: []\n")
		policy, err := store.WorkdirPolicy("/home/me")
		if err != nil || policy == nil {
			t.Fatalf("WorkdirPolicy() = %v, %v; want a policy", policy, err)
		}
		if got := policy.Check("/home/me/git/app").Verdict; got != pathpolicy.NotAllowed {
			t.Fatalf("Check() = %d, want NotAllowed", got)
		}
	})
	t.Run("allows the listed roots", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("workdirs:\n  allow:\n    - path: ~/git\n")
		policy, err := store.WorkdirPolicy("/home/me")
		if err != nil {
			t.Fatal(err)
		}
		if got := policy.Check("/home/me/git/app").Verdict; got != pathpolicy.Allowed {
			t.Fatalf("Check(/home/me/git/app) = %d, want Allowed", got)
		}
		if got := policy.Check("/home/me/other").Verdict; got != pathpolicy.NotAllowed {
			t.Fatalf("Check(/home/me/other) = %d, want NotAllowed", got)
		}
	})
}
