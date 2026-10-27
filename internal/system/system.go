// Package system holds the thin adapters between caged and the host operating
// system. It is the only internal package that imports os, os/exec, or the
// standard runtime package. Other packages declare small interfaces that these
// types satisfy, so they can be tested with fakes.
package system

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
)

// FS is the real filesystem. Most methods are the os or path/filepath call of
// the same name, and their comments state what callers rely on;
// MkdirAllPrivate, CreateFile and ReplaceFile (fsfiles.go) do more.
type FS struct{}

// Abs returns path made absolute against the working directory, cleaned.
func (FS) Abs(path string) (string, error) { return filepath.Abs(path) }

// Stat describes name, following symlinks.
func (FS) Stat(name string) (fs.FileInfo, error) { return os.Stat(name) }

// Lstat describes name itself: a symlink is reported as a symlink.
func (FS) Lstat(name string) (fs.FileInfo, error) { return os.Lstat(name) }

// ReadDir lists the directory name, sorted by file name, so walks and
// write-backs visit entries in a fixed order.
func (FS) ReadDir(name string) ([]fs.DirEntry, error) { return os.ReadDir(name) }

// EvalSymlinks returns path with every symlink in it resolved.
func (FS) EvalSymlinks(path string) (string, error) { return filepath.EvalSymlinks(path) }

// Readlink returns the text of the symlink name, unresolved.
func (FS) Readlink(name string) (string, error) { return os.Readlink(name) }

// WalkDir walks root in lexical order without following symlinks. When fn
// returned nil for a directory it then cannot read, fn is called a second time
// with the error; a root that cannot be inspected gets one call with a nil entry.
func (FS) WalkDir(root string, fn fs.WalkDirFunc) error { return filepath.WalkDir(root, fn) }

// MkdirTemp creates a new directory with mode 0700 (before the umask), named
// from pattern with its last "*" replaced, in dir or the default temporary
// directory, and returns its path.
func (FS) MkdirTemp(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}

// WriteFile writes data to name, following a symlink at name. perm (before
// the umask) applies only when it creates the file; an existing file is
// truncated and keeps its mode.
func (FS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}

// ReadFile returns the contents of name, following symlinks.
func (FS) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }

// notRegularError is ReadRegularFile's error for a path that is not a
// regular file; errors.Is matches it against notRegularError{}.
type notRegularError struct{}

func (notRegularError) Error() string { return "not a regular file" }

// tooLargeError is ReadRegularFile's error for a file over its limit.
type tooLargeError struct{ limit int64 }

func (e tooLargeError) Error() string { return fmt.Sprintf("larger than %d bytes", e.limit) }

// ReadRegularFile returns the contents of name, following symlinks, when it
// is a regular file of at most limit bytes; otherwise it fails with
// notRegularError or tooLargeError. It never blocks on a FIFO: the file is
// checked before it is opened, and opened without blocking (and without
// becoming a controlling terminal) and checked again, so a FIFO put in its
// place in between is refused too. At most limit+1 bytes are read.
func (FS) ReadRegularFile(name string, limit int64) ([]byte, error) {
	if info, err := os.Stat(name); err != nil {
		return nil, err
	} else if !info.Mode().IsRegular() {
		return nil, notRegularError{}
	}
	f, err := os.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() // read-only: nothing to flush
	if info, err := f.Stat(); err != nil {
		return nil, err
	} else if !info.Mode().IsRegular() {
		return nil, notRegularError{}
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1)) // a sparse file can claim any size
	if err == nil && int64(len(data)) > limit {
		return nil, tooLargeError{limit: limit}
	}
	return data, err
}

// RemoveAll removes path and everything below it. A path that does not exist
// is not an error, so cleanup can run twice.
func (FS) RemoveAll(path string) error { return os.RemoveAll(path) }

// Remove removes the file or empty directory name.
func (FS) Remove(name string) error { return os.Remove(name) }

// Mkdir creates the directory name with perm (before the umask). When
// anything already exists there, even a dangling symlink, it fails with an
// error wrapping fs.ErrExist.
func (FS) Mkdir(name string, perm fs.FileMode) error { return os.Mkdir(name, perm) }

// MkdirAll creates name and any missing parents with perm (before the umask),
// leaving existing directories alone.
func (FS) MkdirAll(name string, perm fs.FileMode) error { return os.MkdirAll(name, perm) }

// Chmod sets the permission bits of name, following a symlink at name.
func (FS) Chmod(name string, perm fs.FileMode) error { return os.Chmod(name, perm) }

// Env is the real process environment.
type Env struct{}

// LookupEnv returns the variable key and whether it is set, even when empty.
func (Env) LookupEnv(key string) (string, bool) { return os.LookupEnv(key) }

// UserHomeDir returns the home directory ($HOME on Linux and macOS).
func (Env) UserHomeDir() (string, error) { return os.UserHomeDir() }

// Getwd returns the current working directory.
func (Env) Getwd() (string, error) { return os.Getwd() }

// Command describes one process to run. Nil streams are connected to the null device.
type Command struct {
	Path   string
	Args   []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// CatchSignals keeps caged running through SIGINT, SIGHUP and SIGTERM
	// while the command runs, unless caged ignores them already. They are not
	// passed on: a terminal already sends SIGINT and SIGHUP to the whole
	// foreground group. Run waits for the command as usual and then returns a
	// *SignalError. A command that a signal ended is reported as a
	// *KilledError.
	CatchSignals bool
}

// SignalError reports that caged received Signal while a command with
// CatchSignals ran. Unwrap gives the command's own result, nil when it
// succeeded.
type SignalError struct {
	Signal syscall.Signal
	err    error
}

func (e *SignalError) Error() string { return "received signal: " + e.Signal.String() }

// Unwrap returns the command's own result.
func (e *SignalError) Unwrap() error { return e.err }

// ExitCode is the status a shell reports for a process the signal ended,
// which is how caged exited before it caught signals.
func (e *SignalError) ExitCode() int { return 128 + int(e.Signal) }

// KilledError reports that a command with CatchSignals was ended by Signal
// instead of exiting. Err is the *exec.ExitError, whose message it keeps.
type KilledError struct {
	Signal syscall.Signal
	Err    error
}

func (e *KilledError) Error() string { return e.Err.Error() }

// Unwrap returns the command's own result.
func (e *KilledError) Unwrap() error { return e.Err }

// ExitCode is the status a shell reports for the killed command.
func (e *KilledError) ExitCode() int { return 128 + int(e.Signal) }

// Executor runs real processes.
type Executor struct{}

// LookPath finds the executable file: in the directories of PATH, or directly
// when file contains a slash. Callers treat any error as "not installed".
func (Executor) LookPath(file string) (string, error) { return exec.LookPath(file) }

// Run starts the command and waits for it. A non-zero exit status is returned
// as an *exec.ExitError so callers can pass the exit code through. With
// c.CatchSignals, a signal caught while it ran is returned as a *SignalError
// around that result, and a command a signal ended as a *KilledError; the
// handler is removed before Run returns.
func (Executor) Run(ctx context.Context, c Command) error {
	cmd := exec.CommandContext(ctx, c.Path, c.Args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.Stdin, c.Stdout, c.Stderr
	if !c.CatchSignals {
		return cmd.Run()
	}
	caught := make(chan os.Signal, 1)
	if watched := catchable(); len(watched) > 0 { // Notify with no signals would catch every one
		signal.Notify(caught, watched...)
		defer signal.Stop(caught)
	}
	err := cmd.Run()
	select {
	case sig := <-caught:
		return &SignalError{Signal: sig.(syscall.Signal), err: err} // Notify delivers only the syscall.Signal values it was given
	default:
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return &KilledError{Signal: status.Signal(), Err: err}
		}
	}
	return err
}

// catchable returns the signals CatchSignals catches: SIGINT, SIGHUP and
// SIGTERM, except those caged ignores, as under nohup, so they stay ignored
// for caged and the command.
func catchable() []os.Signal {
	var watched []os.Signal
	for _, sig := range []os.Signal{syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM} {
		if !signal.Ignored(sig) {
			watched = append(watched, sig)
		}
	}
	return watched
}

// Platform describes the host operating system, CPU architecture, and user.
type Platform struct {
	OS   string // a GOOS value such as "linux" or "darwin"
	Arch string // a GOARCH value such as "amd64" or "arm64"
	UID  int
	GID  int
}

// FoldsCase reports whether the platform's default file system ignores case
// in names: macOS's APFS and HFS+ do, so ~/.SSH is ~/.ssh there.
func (p Platform) FoldsCase() bool { return p.OS == "darwin" }

// CurrentPlatform reports the platform of the running process.
func CurrentPlatform() Platform {
	return Platform{OS: runtime.GOOS, Arch: runtime.GOARCH, UID: os.Getuid(), GID: os.Getgid()}
}

// Stdio holds the standard streams of the process.
type Stdio struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// CurrentStdio returns the real standard streams. They stay *os.File values so
// child processes inherit the terminal directly, which `docker run -it` needs.
func CurrentStdio() Stdio {
	return Stdio{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}
}

// IsTerminal reports whether r is a terminal, so a person can answer a
// question read from it. Pipes, files and anything that is not an *os.File
// are not terminals.
func IsTerminal(r io.Reader) bool {
	file, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
