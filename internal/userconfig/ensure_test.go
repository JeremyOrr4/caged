package userconfig

import (
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

func TestEnsureCreatesTheStarterFileOnFirstUse(t *testing.T) {
	t.Parallel()

	fsys := &fakeFS{}
	store := NewStore(fsys, testkit.Env{Home: "/home/me"})
	if created, err := store.Ensure(); err != nil || !created {
		t.Fatalf("Ensure() = %v, %v; want the file created", created, err)
	}
	if got := fsys.files[testPath]; got != starterConfig {
		t.Errorf("created %q; want the starter config", got)
	}
	if want := []string{"/home/me/.config/caged"}; !reflect.DeepEqual(fsys.dirs, want) {
		t.Errorf("created directories %v; want %v", fsys.dirs, want)
	}
	status, err := store.Status()
	if err != nil || !status.Found || !status.Created {
		t.Errorf("Status() = %+v, %v; want Found and Created", status, err)
	}
	if ext, err := store.Load(); ext != nil || err != nil {
		t.Errorf("Load() = %+v, %v; want the defaults", ext, err)
	}
}

// An absolute XDG_CONFIG_HOME may live outside the home directory, so a home
// that is missing, or cannot be found at all, does not stop Ensure.
func TestEnsureUsesAnAbsoluteXDGConfigHome(t *testing.T) {
	t.Parallel()

	xdg := map[string]string{"XDG_CONFIG_HOME": "/xdg"}
	tests := []struct {
		name string
		fsys *fakeFS
		env  testkit.Env
	}{
		{name: "home directory exists", fsys: &fakeFS{}, env: testkit.Env{Home: "/home/me", Vars: xdg}},
		{name: "home directory missing", fsys: &fakeFS{noHome: true}, env: testkit.Env{Home: "/home/me", Vars: xdg}},
		{name: "no home directory known", fsys: &fakeFS{}, env: testkit.Env{HomeErr: errors.New("unset"), Vars: xdg}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if created, err := NewStore(tt.fsys, tt.env).Ensure(); err != nil || !created {
				t.Fatalf("Ensure() = %v, %v; want the file created", created, err)
			}
			if got := tt.fsys.files["/xdg/caged/config.yaml"]; got != starterConfig || len(tt.fsys.files) != 1 {
				t.Errorf("files = %v; want only the starter config at /xdg/caged/config.yaml", tt.fsys.files)
			}
			if want := []string{"/xdg/caged"}; !reflect.DeepEqual(tt.fsys.dirs, want) {
				t.Errorf("created directories %v; want %v", tt.fsys.dirs, want)
			}
		})
	}
}

func TestEnsureNeverTouchesAnExistingFile(t *testing.T) {
	t.Parallel()

	store, fsys := storeWith("image:\n  apt: [tmux]\n")
	if created, err := store.Ensure(); err != nil || created {
		t.Fatalf("Ensure() = %v, %v; want nothing created", created, err)
	}
	if fsys.creates != 0 || len(fsys.dirs) != 0 {
		t.Errorf("Ensure made %d create and %d mkdir calls; want none for an existing file", fsys.creates, len(fsys.dirs))
	}
	if got := fsys.files[testPath]; got != "image:\n  apt: [tmux]\n" {
		t.Errorf("file = %q; want it unchanged", got)
	}
	if status, _ := store.Status(); status.Created {
		t.Error("Status().Created = true for a file that already existed")
	}
}

func TestEnsureLosesTheRaceQuietly(t *testing.T) {
	t.Parallel()

	fsys := &fakeFS{raceLost: true}
	store := NewStore(fsys, testkit.Env{Home: "/home/me"})
	if created, err := store.Ensure(); err != nil || created {
		t.Fatalf("Ensure() = %v, %v; want false, nil when another process created the file first", created, err)
	}
	if status, _ := store.Status(); status.Created {
		t.Error("Status().Created = true after losing the race")
	}
}

func TestEnsureConcurrentCallersCreateOneFile(t *testing.T) {
	t.Parallel()

	fsys := &fakeFS{}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := NewStore(fsys, testkit.Env{Home: "/home/me"}).Ensure(); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if got := fsys.files[testPath]; got != starterConfig {
		t.Errorf("file = %q; want the starter config", got)
	}
}

func TestEnsureErrorsAreActionable(t *testing.T) {
	t.Parallel()

	denied := &fs.PathError{Op: "mkdir", Path: "/home/me/.config/caged", Err: fs.ErrPermission}
	tests := []struct {
		name string
		fsys *fakeFS
		want string
	}{
		{name: "directory", fsys: &fakeFS{mkdirErr: denied}, want: "create ~/.config/caged: "},
		{name: "file", fsys: &fakeFS{createErr: errors.New("disk full")}, want: "create ~/.config/caged/config.yaml: disk full"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewStore(tt.fsys, testkit.Env{Home: "/home/me"}).Ensure()
			if err == nil || !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), "XDG_CONFIG_HOME") {
				t.Fatalf("Ensure() error = %v; want %q and a pointer to XDG_CONFIG_HOME", err, tt.want)
			}
		})
	}
}

func TestEnsureFailsWithoutAHomeDirectory(t *testing.T) {
	t.Parallel()

	_, err := NewStore(&fakeFS{}, testkit.Env{HomeErr: errors.New("unset")}).Ensure()
	if err == nil || !strings.Contains(err.Error(), "find home directory") {
		t.Fatalf("Ensure() error = %v; want the home directory error", err)
	}
}

func TestEnsureForgetsAnEarlierMissingRead(t *testing.T) {
	t.Parallel()

	fsys := &fakeFS{}
	store := NewStore(fsys, testkit.Env{Home: "/home/me"})
	if status, _ := store.Status(); status.Found {
		t.Fatal("file reported found before it exists")
	}
	if _, err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	if status, _ := store.Status(); !status.Found {
		t.Error("Status() still says the file is missing after Ensure created it")
	}
}

func TestEnsureForgetsTheBytesOfAFileThatIsGone(t *testing.T) {
	t.Parallel()

	store, fsys := storeWith("image:\n  apt: [tmux]\n")
	if _, _, err := store.Content(); err != nil {
		t.Fatal(err)
	}
	delete(fsys.files, testPath)
	fsys.noHome = true // so Ensure leaves the missing file missing
	if created, err := store.Ensure(); err != nil || created {
		t.Fatalf("Ensure() = %v, %v; want nothing created", created, err)
	}
	if data, found, err := store.Content(); data != nil || found || err != nil {
		t.Fatalf("Content() = %q, %v, %v; want nothing for a file that is gone", data, found, err)
	}
}

func TestContentReturnsTheBytesTheStoreActsOn(t *testing.T) {
	t.Parallel()

	store, fsys := storeWith("image:\n  apt: [tmux]\n")
	data, found, err := store.Content()
	if err != nil || !found || string(data) != "image:\n  apt: [tmux]\n" {
		t.Fatalf("Content() = %q, %v, %v; want the file", data, found, err)
	}
	fsys.files[testPath] = "image:\n  apt: [evil]\n" // edited after the store read it
	if ext, _ := store.Load(); !reflect.DeepEqual(ext, &plan.Extension{Source: testSource, Apt: []string{"tmux"}}) {
		t.Errorf("Load() = %+v; want the bytes that Content returned", ext)
	}
}

func TestContentReportsAMissingFileAndInvalidFiles(t *testing.T) {
	t.Parallel()

	if data, found, err := NewStore(&fakeFS{}, testkit.Env{Home: "/home/me"}).Content(); data != nil || found || err != nil {
		t.Errorf("Content() = %q, %v, %v; want nothing for a missing file", data, found, err)
	}
	store, _ := storeWith("bogus: 1\n")
	if _, _, err := store.Content(); err == nil {
		t.Error("Content() succeeded for an invalid file; want the parse error")
	}
}

func TestDirIsTheDirectoryOfThePath(t *testing.T) {
	t.Parallel()

	store := NewStore(&fakeFS{}, testkit.Env{Home: "/home/me", Vars: map[string]string{"XDG_CONFIG_HOME": "/xdg"}})
	if dir, err := store.Dir(); err != nil || dir != "/xdg/caged" {
		t.Errorf("Dir() = %q, %v; want /xdg/caged", dir, err)
	}
}

func TestEnsureDoesNotBuildAMissingHomeDirectory(t *testing.T) {
	t.Parallel()

	fsys := &fakeFS{noHome: true}
	if _, err := NewStore(fsys, testkit.Env{Home: "/home/me"}).Ensure(); err != nil {
		t.Fatalf("Ensure() error = %v; want a quiet no-op for a home that does not exist", err)
	}
	if len(fsys.dirs) != 0 || fsys.creates != 0 {
		t.Errorf("created %v and %d files; want nothing", fsys.dirs, fsys.creates)
	}
}

// missingDir is how app asks whether Ensure failed before the folder existed.
type missingDir interface{ MissingDir() bool }

// TestEnsureSaysWhenTheConfigFolderIsMissing pins WB-T4 (bugs.md B-APP-2,
// D-55): a folder Ensure could not create is told apart from a file it could
// not write, because only a missing folder leaves nothing to mount.
func TestEnsureSaysWhenTheConfigFolderIsMissing(t *testing.T) {
	t.Parallel()

	denied := &fs.PathError{Op: "mkdir", Path: "/home/me/.config/caged", Err: fs.ErrPermission}
	_, err := NewStore(&fakeFS{mkdirErr: denied}, testkit.Env{Home: "/home/me"}).Ensure()
	var missing missingDir
	if !errors.As(err, &missing) || !missing.MissingDir() || !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("Ensure() error = %v; want a missing-folder error wrapping the mkdir failure", err)
	}
	_, err = NewStore(&fakeFS{createErr: errors.New("disk full")}, testkit.Env{Home: "/home/me"}).Ensure()
	if err == nil || errors.As(err, &missing) {
		t.Fatalf("Ensure() error = %v; want a file error that is not a missing folder", err)
	}
	// A dangling symlink at the folder makes mkdir fail with "file exists";
	// its target could appear where a container writes, so that stays fatal.
	for _, mkdirErr := range []error{&fs.PathError{Op: "mkdir", Path: "/home/me/.config/caged", Err: fs.ErrExist}, errors.New("not a directory")} {
		_, err = NewStore(&fakeFS{mkdirErr: mkdirErr}, testkit.Env{Home: "/home/me"}).Ensure()
		if err == nil || errors.As(err, &missing) || !strings.Contains(err.Error(), "create ~/.config/caged: ") {
			t.Fatalf("Ensure() error = %v; want a creation error that is not a missing folder", err)
		}
	}
}

// TestEnsureRefusesAConfigPathThatCannotBeRead pins WB-T4 (bugs.md Q-APP-6):
// a config.yaml that exists but cannot be read, such as a dangling symlink,
// is an error, not "another process created it": the folder would otherwise
// be mounted with nothing approved, and a file the container put there would
// be approved without a question on the next run.
func TestEnsureRefusesAConfigPathThatCannotBeRead(t *testing.T) {
	t.Parallel()

	fsys := &fakeFS{createErr: &fs.PathError{Op: "create", Path: testPath, Err: fs.ErrExist}} // and Stat finds nothing
	created, err := NewStore(fsys, testkit.Env{Home: "/home/me"}).Ensure()
	var missing missingDir
	if created || err == nil || !strings.HasPrefix(err.Error(), "~/.config/caged/config.yaml exists but cannot be read: ") || errors.As(err, &missing) {
		t.Fatalf("Ensure() = %v, %v; want an error naming the unreadable file", created, err)
	}
}
