package project

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/system"
)

func TestNewMountGuardRails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cwd   string
		home  string
		allow bool
		want  string
	}{
		{name: "filesystem root", cwd: "/", home: "/Users/test", want: "filesystem root"},
		{name: "home directory", cwd: "/Users/test", home: "/Users/test", want: "entire home directory"},
		{name: "allowed home directory", cwd: "/Users/test", home: "/Users/test", allow: true},
		{name: "project directory", cwd: "/Users/test/code/caged", home: "/Users/test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mount, err := NewMount(tt.cwd, tt.home, tt.allow)
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("newMount() error = %v, want %q", err, tt.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("newMount() error = %v", err)
			}
			wantPath := filepath.Join("/workspace", filepath.Base(filepath.Clean(tt.cwd)))
			if mount.ContainerPath != wantPath {
				t.Errorf("container path = %q, want %q", mount.ContainerPath, wantPath)
			}
		})
	}
}

func TestResolveProjectMount(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	projectPath := filepath.Join(t.TempDir(), "project")
	if err := os.Mkdir(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(projectPath, "file.txt")
	if err := os.WriteFile(file, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolver := NewResolver(system.FS{}, nil)

	mount, err := resolver.Resolve(projectPath, home, false)
	if err != nil {
		t.Fatalf("resolveProjectMount() error = %v", err)
	}
	resolvedProject, err := filepath.EvalSymlinks(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if mount.HostPath != resolvedProject || mount.ContainerPath != "/workspace/project" {
		t.Fatalf("resolveProjectMount() = %+v, want host %q at /workspace/project", mount, resolvedProject)
	}

	if _, err := resolver.Resolve(file, home, false); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("resolveProjectMount(file) error = %v, want not-a-directory error", err)
	}

	if _, err := resolver.Resolve(home, home, false); err == nil || !strings.Contains(err.Error(), "entire home directory") {
		t.Fatalf("resolveProjectMount(home) error = %v, want broad-mount rejection", err)
	}
}

// fakeFS answers from maps. Paths missing from entries do not exist, and
// paths missing from resolved cannot be resolved.
type fakeFS struct {
	cwd      string
	entries  map[string]bool   // path -> is a directory
	resolved map[string]string // EvalSymlinks results
	absErr   error
}

func (f fakeFS) Abs(path string) (string, error) {
	if f.absErr != nil {
		return "", f.absErr
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(f.cwd, path), nil
}

func (f fakeFS) Stat(name string) (fs.FileInfo, error) {
	isDir, ok := f.entries[name]
	if !ok {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}
	return fakeInfo{name: filepath.Base(name), dir: isDir}, nil
}

func (f fakeFS) EvalSymlinks(path string) (string, error) {
	if resolved, ok := f.resolved[path]; ok {
		return resolved, nil
	}
	return "", &fs.PathError{Op: "lstat", Path: path, Err: fs.ErrNotExist}
}

type fakeInfo struct {
	name string
	dir  bool
}

func (i fakeInfo) Name() string       { return i.name }
func (i fakeInfo) Size() int64        { return 0 }
func (i fakeInfo) ModTime() time.Time { return time.Time{} }
func (i fakeInfo) IsDir() bool        { return i.dir }
func (i fakeInfo) Sys() any           { return nil }
func (i fakeInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}

func TestResolverResolvesRelativePathsAndMacOSSymlinks(t *testing.T) {
	t.Parallel()

	// On macOS /tmp is a symlink to /private/tmp; the mount must use the real path.
	fsys := fakeFS{
		cwd:      "/tmp",
		entries:  map[string]bool{"/tmp/app": true},
		resolved: map[string]string{"/tmp/app": "/private/tmp/app", "/Users/dev": "/Users/dev"},
	}
	mount, err := NewResolver(fsys, nil).Resolve("app", "/Users/dev", false)
	if err != nil {
		t.Fatal(err)
	}
	if mount.HostPath != "/private/tmp/app" || mount.ContainerPath != "/workspace/app" {
		t.Fatalf("Resolve() = %+v", mount)
	}
}

func TestResolverComparesHomeAfterResolvingSymlinks(t *testing.T) {
	t.Parallel()

	fsys := fakeFS{
		entries:  map[string]bool{"/home/dev": true},
		resolved: map[string]string{"/home/dev": "/data/home/dev"},
	}
	if _, err := NewResolver(fsys, nil).Resolve("/home/dev", "/home/dev", false); err == nil || !strings.Contains(err.Error(), "entire home directory") {
		t.Fatalf("Resolve(home) error = %v, want broad-mount rejection", err)
	}
}

func TestResolverErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fsys fakeFS
		want string
	}{
		{name: "abs fails", fsys: fakeFS{absErr: errors.New("boom")}, want: "resolve working directory: boom"},
		{name: "missing directory", fsys: fakeFS{}, want: `inspect working directory "/work/app": stat /work/app: file does not exist`},
		{name: "regular file", fsys: fakeFS{entries: map[string]bool{"/work/app": false}}, want: `working directory "/work/app" is not a directory`},
		{name: "unresolvable directory", fsys: fakeFS{entries: map[string]bool{"/work/app": true}}, want: "resolve working directory symlinks: lstat /work/app: file does not exist"},
		{
			name: "unresolvable home",
			fsys: fakeFS{entries: map[string]bool{"/work/app": true}, resolved: map[string]string{"/work/app": "/work/app"}},
			want: "resolve home directory: lstat /Users/dev: file does not exist",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewResolver(tt.fsys, nil).Resolve("/work/app", "/Users/dev", false); err == nil || err.Error() != tt.want {
				t.Fatalf("Resolve() error = %v, want %q", err, tt.want)
			}
		})
	}
}

// fakePolicies returns a fixed workdir policy and records the home it was asked about.
type fakePolicies struct {
	policy  *pathpolicy.Policy
	err     error
	gotHome string
}

func (f *fakePolicies) WorkdirPolicy(home string) (*pathpolicy.Policy, error) {
	f.gotHome = home
	return f.policy, f.err
}

func workdirPolicy(t *testing.T, home string, allow ...string) *pathpolicy.Policy {
	t.Helper()
	cfg := pathpolicy.Config{Home: home, Source: "~/.config/caged/config.yaml", Field: "workdirs.allow"}
	for _, path := range allow {
		cfg.Allow = append(cfg.Allow, pathpolicy.Root{Path: path})
	}
	policy, err := pathpolicy.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func TestResolverAppliesTheWorkdirPolicy(t *testing.T) {
	t.Parallel()

	fsys := fakeFS{
		entries: map[string]bool{"/Users/dev/git/app": true, "/Users/dev/other/app": true},
		resolved: map[string]string{
			"/Users/dev/git/app":   "/Users/dev/git/app",
			"/Users/dev/other/app": "/Users/dev/other/app",
			"/Users/dev":           "/Users/dev",
		},
	}
	policy := workdirPolicy(t, "/Users/dev", "/Users/dev/git")

	t.Run("a directory under workdirs.allow is mounted", func(t *testing.T) {
		t.Parallel()
		policies := &fakePolicies{policy: policy}
		mount, err := NewResolver(fsys, policies).Resolve("/Users/dev/git/app", "/Users/dev", false)
		if err != nil {
			t.Fatal(err)
		}
		if mount.HostPath != "/Users/dev/git/app" || policies.gotHome != "/Users/dev" {
			t.Fatalf("Resolve() = %+v for home %q", mount, policies.gotHome)
		}
	})
	t.Run("a directory outside workdirs.allow is refused with a hint", func(t *testing.T) {
		t.Parallel()
		_, err := NewResolver(fsys, &fakePolicies{policy: policy}).Resolve("/Users/dev/other/app", "/Users/dev", false)
		want := `refusing to mount /Users/dev/other/app: it is not under workdirs.allow; add "- path: ~/other/app" under workdirs.allow in ~/.config/caged/config.yaml`
		if err == nil || err.Error() != want {
			t.Fatalf("Resolve() error = %v, want %q", err, want)
		}
	})
	t.Run("--allow-broad-mount does not bypass it", func(t *testing.T) {
		t.Parallel()
		_, err := NewResolver(fsys, &fakePolicies{policy: policy}).Resolve("/Users/dev/other/app", "/Users/dev", true)
		if err == nil || !strings.Contains(err.Error(), "workdirs.allow") {
			t.Fatalf("Resolve() error = %v, want a workdirs.allow refusal", err)
		}
	})
	t.Run("no policy allows every directory", func(t *testing.T) {
		t.Parallel()
		if _, err := NewResolver(fsys, &fakePolicies{}).Resolve("/Users/dev/other/app", "/Users/dev", false); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("a policy error is passed through", func(t *testing.T) {
		t.Parallel()
		policyErr := errors.New("~/.config/caged/config.yaml: invalid YAML: line 1")
		_, err := NewResolver(fsys, &fakePolicies{err: policyErr}).Resolve("/Users/dev/git/app", "/Users/dev", false)
		if !errors.Is(err, policyErr) {
			t.Fatalf("Resolve() error = %v, want %v", err, policyErr)
		}
	})
}

func TestResolverAsksThePolicyAboutTheHomeAsGiven(t *testing.T) {
	t.Parallel()

	fsys := fakeFS{
		entries:  map[string]bool{"/home/dev/app": true},
		resolved: map[string]string{"/home/dev/app": "/data/home/dev/app", "/home/dev": "/data/home/dev"},
	}
	policies := &fakePolicies{policy: workdirPolicy(t, "/home/dev", "/data/home/dev")}
	if _, err := NewResolver(fsys, policies).Resolve("/home/dev/app", "/home/dev", false); err != nil {
		t.Fatal(err)
	}
	if policies.gotHome != "/home/dev" {
		t.Fatalf("policy was asked about home %q, want the home as given, /home/dev", policies.gotHome)
	}
}
