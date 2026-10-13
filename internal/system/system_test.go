package system

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFSUsesTheRealFilesystem(t *testing.T) {
	t.Parallel()

	var fsys FS
	root, err := fsys.MkdirTemp(t.TempDir(), "system-test-")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "file.txt")
	if err := fsys.WriteFile(file, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if info, err := fsys.Stat(link); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("Stat(link) = %v, %v; want the regular target file", info, err)
	}
	if info, err := fsys.Lstat(link); err != nil || info.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("Lstat(link) = %v, %v; want the symlink itself", info, err)
	}
	if entries, err := fsys.ReadDir(root); err != nil || len(entries) != 2 {
		t.Fatalf("ReadDir() = %v, %v; want two entries", entries, err)
	}
	resolvedFile, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := fsys.EvalSymlinks(link); err != nil || got != resolvedFile {
		t.Fatalf("EvalSymlinks(link) = %q, %v; want %q", got, err, resolvedFile)
	}
	if got, err := fsys.Abs(file); err != nil || got != file {
		t.Fatalf("Abs(%q) = %q, %v", file, got, err)
	}
	visited := 0
	if err := fsys.WalkDir(root, func(string, fs.DirEntry, error) error { visited++; return nil }); err != nil || visited != 3 {
		t.Fatalf("WalkDir() visited %d entries, err %v; want 3", visited, err)
	}
	if err := fsys.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("RemoveAll() left %q behind: %v", root, err)
	}
}

func TestEnvReadsTheProcessEnvironment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CAGED_SYSTEM_TEST", "value")

	var env Env
	if got, ok := env.LookupEnv("CAGED_SYSTEM_TEST"); !ok || got != "value" {
		t.Fatalf("LookupEnv() = %q, %v", got, ok)
	}
	if runtime.GOOS != "windows" {
		if got, err := env.UserHomeDir(); err != nil || got != home {
			t.Fatalf("UserHomeDir() = %q, %v; want %q", got, err, home)
		}
	}
	want, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := env.Getwd(); err != nil || got != want {
		t.Fatalf("Getwd() = %q, %v; want %q", got, err, want)
	}
}

func TestExecutorRunsProcessesAndReportsExitCodes(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell")
	}

	var executor Executor
	sh, err := executor.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err = executor.Run(context.Background(), Command{
		Path:   sh,
		Args:   []string{"-c", "read line; echo \"out:$line\"; echo err >&2; exit 3"},
		Stdin:  bytes.NewBufferString("input\n"),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 {
		t.Fatalf("Run() error = %v, want exit status 3", err)
	}
	if stdout.String() != "out:input\n" || stderr.String() != "err\n" {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := executor.Run(ctx, Command{Path: sh, Args: []string{"-c", "exit 0"}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() with a cancelled context = %v, want context.Canceled", err)
	}
}

func TestCurrentPlatformAndStdioDescribeThisProcess(t *testing.T) {
	t.Parallel()

	want := Platform{OS: runtime.GOOS, Arch: runtime.GOARCH, UID: os.Getuid(), GID: os.Getgid()}
	if got := CurrentPlatform(); got != want {
		t.Fatalf("CurrentPlatform() = %+v, want %+v", got, want)
	}
	stdio := CurrentStdio()
	// docker run -it needs the real terminal file, not a copy through a pipe.
	if stdin, ok := stdio.In.(*os.File); !ok || stdin != os.Stdin {
		t.Fatal("CurrentStdio().In is not os.Stdin")
	}
	if stdout, ok := stdio.Out.(*os.File); !ok || stdout != os.Stdout {
		t.Fatal("CurrentStdio().Out is not os.Stdout")
	}
	if stderr, ok := stdio.Err.(*os.File); !ok || stderr != os.Stderr {
		t.Fatal("CurrentStdio().Err is not os.Stderr")
	}
}
