package userconfig

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

const (
	testPath   = "/home/me/.config/caged/config.yaml"
	testSource = "~/.config/caged/config.yaml"
)

func storeWith(body string) (*Store, *fakeFS) {
	fsys := &fakeFS{files: map[string]string{testPath: body}}
	return NewStore(fsys, testkit.Env{Home: "/home/me"}), fsys
}

func TestStorePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		env         testkit.Env
		want        string
		wantDisplay string
		wantErr     string
	}{
		{name: "default", env: testkit.Env{Home: "/home/me"}, want: "/home/me/.config/caged/config.yaml", wantDisplay: "~/.config/caged/config.yaml"},
		{name: "XDG_CONFIG_HOME wins when absolute", env: testkit.Env{Home: "/home/me", Vars: map[string]string{"XDG_CONFIG_HOME": "/xdg"}}, want: "/xdg/caged/config.yaml", wantDisplay: "/xdg/caged/config.yaml"},
		{name: "XDG_CONFIG_HOME under the home directory is shown with ~", env: testkit.Env{Home: "/home/me", Vars: map[string]string{"XDG_CONFIG_HOME": "/home/me/cfg"}}, want: "/home/me/cfg/caged/config.yaml", wantDisplay: "~/cfg/caged/config.yaml"},
		{name: "relative XDG_CONFIG_HOME is ignored", env: testkit.Env{Home: "/home/me", Vars: map[string]string{"XDG_CONFIG_HOME": "cfg"}}, want: "/home/me/.config/caged/config.yaml", wantDisplay: "~/.config/caged/config.yaml"},
		{name: "empty XDG_CONFIG_HOME is ignored", env: testkit.Env{Home: "/home/me", Vars: map[string]string{"XDG_CONFIG_HOME": ""}}, want: "/home/me/.config/caged/config.yaml", wantDisplay: "~/.config/caged/config.yaml"},
		{name: "no home directory", env: testkit.Env{HomeErr: errors.New("unset")}, wantErr: "find home directory: unset"},
		{name: "XDG needs no home directory", env: testkit.Env{HomeErr: errors.New("unset"), Vars: map[string]string{"XDG_CONFIG_HOME": "/xdg"}}, want: "/xdg/caged/config.yaml", wantDisplay: "/xdg/caged/config.yaml"},
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
				if _, err := store.Source(); err == nil || err.Error() != tt.wantErr {
					t.Fatalf("Source() error = %v, want %q", err, tt.wantErr)
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
		store := NewStore(&fakeFS{}, testkit.Env{Home: "/home/me"})
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
		store := NewStore(&fakeFS{err: errors.New("permission denied")}, testkit.Env{Home: "/home/me"})
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
		_, err := NewStore(&fakeFS{}, testkit.Env{HomeErr: errors.New("unset")}).Load()
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
		store := NewStore(&fakeFS{files: map[string]string{"/home/me/.config/caged/image.json": `{"apt":["jq"]}`}}, testkit.Env{Home: "/home/me"})
		status, err := store.Status()
		if err != nil || status.Legacy != "~/.config/caged/image.json" || status.Found {
			t.Fatalf("Status() = %+v, %v; want Legacy ~/.config/caged/image.json and no config.yaml", status, err)
		}
	})
	t.Run("a leftover image.json next to an XDG config is reported", func(t *testing.T) {
		t.Parallel()
		store := NewStore(&fakeFS{files: map[string]string{"/xdg/caged/image.json": `{}`}}, testkit.Env{Home: "/home/me", Vars: map[string]string{"XDG_CONFIG_HOME": "/xdg"}})
		if status, err := store.Status(); err != nil || status.Legacy != "/xdg/caged/image.json" || status.Source != "/xdg/caged/config.yaml" {
			t.Fatalf("Status() = %+v, %v; want Legacy /xdg/caged/image.json", status, err)
		}
	})
	t.Run("no home directory", func(t *testing.T) {
		t.Parallel()
		status, err := NewStore(&fakeFS{}, testkit.Env{HomeErr: errors.New("unset")}).Status()
		if err == nil || err.Error() != "find home directory: unset" || status.Source != "" {
			t.Fatalf("Status() = %+v, %v; want no source and the home directory error", status, err)
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

// TestStateDirFollowsXDGStateHome: StateDir names the folder Approvals
// writes to, so the resolver can keep the caged mount away from it (WB-B1).
func TestStateDirFollowsXDGStateHome(t *testing.T) {
	t.Parallel()

	for env, want := range map[*testkit.Env]string{
		{Home: "/home/me"}: "/home/me/.local/state/caged",
		{Home: "/home/me", Vars: map[string]string{"XDG_STATE_HOME": "/state"}}:   "/state/caged",
		{Home: "/home/me", Vars: map[string]string{"XDG_STATE_HOME": "relative"}}: "/home/me/.local/state/caged",
	} {
		if got, err := NewStore(&fakeFS{}, *env).StateDir(); err != nil || got != want {
			t.Errorf("StateDir() with %v = %q, %v; want %q", env.Vars, got, err, want)
		}
	}
	if _, err := NewStore(&fakeFS{}, testkit.Env{HomeErr: errors.New("unset")}).StateDir(); err == nil {
		t.Error("StateDir() succeeded without a home directory or XDG_STATE_HOME")
	}
}
