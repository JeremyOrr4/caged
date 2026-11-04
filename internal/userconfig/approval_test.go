package userconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/testkit"
)

type fixedPath string

func (p fixedPath) Path() (string, error) { return string(p), nil }

func newApprovals(state *fakeFS, env testkit.Env, config string) *Approvals {
	return NewApprovals(state, env, fixedPath(config))
}

func TestApprovalsStartEmptyThenRememberWhatWasApproved(t *testing.T) {
	t.Parallel()

	state := &fakeFS{}
	approvals := newApprovals(state, testkit.Env{Home: "/home/me"}, testPath)
	if _, ok, err := approvals.Approved(); ok || err != nil {
		t.Fatalf("Approved() = _, %v, %v; want nothing yet", ok, err)
	}
	if err := approvals.Approve([]byte("v1")); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := approvals.Approved(); !ok || err != nil || string(got) != "v1" {
		t.Fatalf("Approved() = %q, %v, %v; want v1", got, ok, err)
	}
	if err := approvals.Approve([]byte("v2")); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := approvals.Approved(); string(got) != "v2" {
		t.Errorf("Approved() = %q; want the latest approval v2", got)
	}
}

func TestApprovalsLiveInTheStateDirectoryNotTheConfigDirectory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		env        testkit.Env
		wantPrefix string
	}{
		{name: "default", env: testkit.Env{Home: "/home/me"}, wantPrefix: "/home/me/.local/state/caged/config-"},
		{name: "XDG_STATE_HOME", env: testkit.Env{Home: "/home/me", Vars: map[string]string{"XDG_STATE_HOME": "/state"}}, wantPrefix: "/state/caged/config-"},
		{name: "relative XDG_STATE_HOME is ignored", env: testkit.Env{Home: "/home/me", Vars: map[string]string{"XDG_STATE_HOME": "state"}}, wantPrefix: "/home/me/.local/state/caged/config-"},
		{name: "empty XDG_STATE_HOME is ignored", env: testkit.Env{Home: "/home/me", Vars: map[string]string{"XDG_STATE_HOME": ""}}, wantPrefix: "/home/me/.local/state/caged/config-"},
		{name: "XDG_STATE_HOME needs no home directory", env: testkit.Env{HomeErr: errors.New("unset"), Vars: map[string]string{"XDG_STATE_HOME": "/state"}}, wantPrefix: "/state/caged/config-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			state := &fakeFS{}
			if err := newApprovals(state, tt.env, testPath).Approve([]byte("x")); err != nil {
				t.Fatal(err)
			}
			if len(state.files) != 1 {
				t.Fatalf("wrote %d files; want 1", len(state.files))
			}
			for name := range state.files {
				if !strings.HasPrefix(name, tt.wantPrefix) || strings.Contains(name, ".config") {
					t.Errorf("approved copy at %s; want it under %s and outside the config directory", name, tt.wantPrefix)
				}
			}
		})
	}
}

func TestApprovalsAreKeptPerConfigFile(t *testing.T) {
	t.Parallel()

	state := &fakeFS{}
	env := testkit.Env{Home: "/home/me"}
	if err := newApprovals(state, env, "/a/caged/config.yaml").Approve([]byte("a")); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := newApprovals(state, env, "/b/caged/config.yaml").Approved(); ok {
		t.Error("a different config file shares the approval of /a")
	}
}

// The approved copy holds the user's config, which may name private paths, so
// only the user may read it, like the config file itself.
func TestApprovalsAreWrittenPrivately(t *testing.T) {
	t.Parallel()

	state := &fakeFS{}
	if err := newApprovals(state, testkit.Env{Home: "/home/me"}, testPath).Approve([]byte("v1")); err != nil {
		t.Fatal(err)
	}
	if len(state.modes) != 1 {
		t.Fatalf("Approve() wrote %v; want one file", state.modes)
	}
	for name, mode := range state.modes {
		if mode != 0o600 {
			t.Errorf("Approve() wrote %s with mode %v; want -rw-------", name, mode)
		}
	}
}

func TestApprovalsReportFailures(t *testing.T) {
	t.Parallel()

	env := testkit.Env{Home: "/home/me"}
	if _, _, err := newApprovals(&fakeFS{err: errors.New("boom")}, env, testPath).Approved(); err == nil {
		t.Error("Approved() hid a read error")
	}
	if err := newApprovals(&fakeFS{writeErr: errors.New("boom")}, env, testPath).Approve(nil); err == nil || !strings.Contains(err.Error(), "record approved config") {
		t.Errorf("Approve() error = %v; want a record approved config error", err)
	}
	mkdirFails := &fakeFS{mkdirErr: errors.New("permission denied")}
	if err := newApprovals(mkdirFails, env, testPath).Approve([]byte("x")); err == nil || err.Error() != "record approved config: permission denied" {
		t.Errorf("Approve() error = %v; want the mkdir error", err)
	}
	if len(mkdirFails.files) != 0 {
		t.Errorf("Approve() wrote %v after the directory could not be made; want nothing", mkdirFails.files)
	}
	if _, _, err := newApprovals(&fakeFS{}, testkit.Env{HomeErr: errors.New("unset")}, testPath).Approved(); err == nil {
		t.Error("Approved() succeeded without a home directory or XDG_STATE_HOME")
	}
}

// The copy's name is part of the approval-state format: renaming it would
// make every user approve their config again.
func TestApprovalsFileNameIsStable(t *testing.T) {
	t.Parallel()

	// the first 6 bytes of sha256("/home/me/.config/caged/config.yaml"), computed with sha256sum
	const want = "/home/me/.local/state/caged/config-bcd4ff88d00b.approved.yaml"
	for _, config := range []string{testPath, "/home/me/.config/caged/../caged/config.yaml"} {
		state := &fakeFS{}
		if err := newApprovals(state, testkit.Env{Home: "/home/me"}, config).Approve([]byte("x")); err != nil {
			t.Fatal(err)
		}
		if _, ok := state.files[want]; !ok || len(state.files) != 1 {
			t.Errorf("approving %s wrote %v; want only %s", config, state.files, want)
		}
	}
}

func TestApprovalsFailWhenTheConfigPathFails(t *testing.T) {
	t.Parallel()

	// the config file cannot be located, while the state directory can
	config := NewStore(&fakeFS{}, testkit.Env{HomeErr: errors.New("no home for the config")})
	state := &fakeFS{}
	approvals := NewApprovals(state, testkit.Env{Home: "/home/me"}, config)
	const want = "find home directory: no home for the config"
	if _, ok, err := approvals.Approved(); ok || err == nil || err.Error() != want {
		t.Errorf("Approved() = _, %v, %v; want %q", ok, err, want)
	}
	if err := approvals.Approve([]byte("x")); err == nil || err.Error() != want {
		t.Errorf("Approve() error = %v; want %q", err, want)
	}
	if len(state.files) != 0 {
		t.Errorf("wrote %v; want nothing", state.files)
	}
}
