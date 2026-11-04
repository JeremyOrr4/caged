package docker

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/configstage"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// fakeExecutor records commands instead of running them.
type fakeExecutor struct {
	lookPathErr error
	// run answers a command; nil means success.
	run      func(system.Command) error
	commands []system.Command
}

func (f *fakeExecutor) LookPath(file string) (string, error) {
	if f.lookPathErr != nil {
		return "", errors.New(`exec: "` + file + `": executable file not found in $PATH`)
	}
	return "/usr/local/bin/" + file, nil
}

func (f *fakeExecutor) Run(_ context.Context, cmd system.Command) error {
	f.commands = append(f.commands, cmd)
	if f.run != nil {
		return f.run(cmd)
	}
	return nil
}

// failingExec returns an executor whose every command writes stderr and then
// fails with err.
func failingExec(stderr string, err error) *fakeExecutor {
	return &fakeExecutor{run: func(cmd system.Command) error {
		if _, writeErr := io.WriteString(cmd.Stderr, stderr); writeErr != nil {
			return writeErr
		}
		return err
	}}
}

// fakeFS records the temporary build context instead of touching the disk.
type fakeFS struct {
	mkdirErr  error
	writeErr  map[string]error // by file name
	removeErr error
	readErr   error  // returned by ReadFile
	idFile    string // what ReadFile returns; "" means "sha256:fake-id\n"
	written   map[string]writtenFile
	removed   []string
}

type writtenFile struct {
	data string
	mode fs.FileMode
}

func (f *fakeFS) MkdirTemp(_, pattern string) (string, error) {
	if f.mkdirErr != nil {
		return "", f.mkdirErr
	}
	return filepath.Join("/tmp", pattern+"123"), nil
}

func (f *fakeFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if err := f.writeErr[name]; err != nil {
		return err
	}
	if f.written == nil {
		f.written = map[string]writtenFile{}
	}
	f.written[name] = writtenFile{data: string(data), mode: perm}
	return nil
}

func (f *fakeFS) ReadFile(_ string) ([]byte, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	if f.idFile != "" {
		return []byte(f.idFile), nil
	}
	return []byte("sha256:fake-id\n"), nil
}

func (f *fakeFS) RemoveAll(path string) error {
	f.removed = append(f.removed, path)
	return f.removeErr
}

var (
	linux  = system.Platform{OS: "linux", Arch: "amd64", UID: 1000, GID: 1001}
	darwin = system.Platform{OS: "darwin", Arch: "arm64", UID: 501, GID: 20}
)

// newClient returns a Client that stages no config mounts.
func newClient(exec Executor, fsys FileSystem, platform system.Platform, stdio system.Stdio) *Client {
	return NewClient(Deps{Exec: exec, FS: fsys, Platform: platform, Stdio: stdio})
}

// exitError stands in for *exec.ExitError: it carries an exit code.
type exitError struct{ code int }

func (e exitError) Error() string { return "exit status 125" }
func (e exitError) ExitCode() int { return e.code }

type failingStager struct{ err error }

func (s failingStager) Stage([]plan.ConfigMount) (*configstage.Session, error) { return nil, s.err }

// removeFailFS is the real filesystem, except that RemoveAll, after removing
// for real, reports err: a staging session's cleanup then fails.
type removeFailFS struct {
	system.FS
	err error
}

func (f removeFailFS) RemoveAll(path string) error {
	if err := f.FS.RemoveAll(path); err != nil {
		return err
	}
	return f.err
}
