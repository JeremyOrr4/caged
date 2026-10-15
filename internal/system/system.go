// Package system holds the thin adapters between caged and the host operating
// system. It is the only internal package that imports os, os/exec, or the
// standard runtime package. Other packages declare small interfaces that these
// types satisfy, so they can be tested with fakes.
package system

import (
	"context"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// FS is the real filesystem.
type FS struct{}

func (FS) Abs(path string) (string, error)              { return filepath.Abs(path) }
func (FS) Stat(name string) (fs.FileInfo, error)        { return os.Stat(name) }
func (FS) Lstat(name string) (fs.FileInfo, error)       { return os.Lstat(name) }
func (FS) ReadDir(name string) ([]fs.DirEntry, error)   { return os.ReadDir(name) }
func (FS) EvalSymlinks(path string) (string, error)     { return filepath.EvalSymlinks(path) }
func (FS) Readlink(name string) (string, error)         { return os.Readlink(name) }
func (FS) WalkDir(root string, fn fs.WalkDirFunc) error { return filepath.WalkDir(root, fn) }
func (FS) MkdirTemp(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}
func (FS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}
func (FS) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }
func (FS) RemoveAll(path string) error          { return os.RemoveAll(path) }

// Env is the real process environment.
type Env struct{}

func (Env) LookupEnv(key string) (string, bool) { return os.LookupEnv(key) }
func (Env) UserHomeDir() (string, error)        { return os.UserHomeDir() }
func (Env) Getwd() (string, error)              { return os.Getwd() }

// Command describes one process to run. Nil streams are connected to the null device.
type Command struct {
	Path   string
	Args   []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Executor runs real processes.
type Executor struct{}

func (Executor) LookPath(file string) (string, error) { return exec.LookPath(file) }

// Run starts the command and waits for it. A non-zero exit status is returned
// as an *exec.ExitError so callers can pass the exit code through.
func (Executor) Run(ctx context.Context, c Command) error {
	cmd := exec.CommandContext(ctx, c.Path, c.Args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.Stdin, c.Stdout, c.Stderr
	return cmd.Run()
}

// Platform describes the host operating system, CPU architecture, and user.
type Platform struct {
	OS   string // a GOOS value such as "linux" or "darwin"
	Arch string // a GOARCH value such as "amd64" or "arm64"
	UID  int
	GID  int
}

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
