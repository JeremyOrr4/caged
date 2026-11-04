package docker

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

func TestBuildImageNamesTheTempDirAfterTheContext(t *testing.T) {
	t.Parallel()

	spec := testBuildSpec(false)
	spec.ContextName = "caged-extension"
	fsys := &fakeFS{}
	if _, err := newClient(&fakeExecutor{}, fsys, linux, system.Stdio{}).BuildImage(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if want := []string{"/tmp/caged-extension-123"}; !reflect.DeepEqual(fsys.removed, want) {
		t.Fatalf("temporary directories = %v, want %v", fsys.removed, want)
	}
}

func testBuildSpec(noCache bool) plan.BuildSpec {
	return plan.BuildSpec{
		Image:       "caged-runtime:latest",
		Dockerfile:  "Dockerfile",
		ContextName: "caged-runtime",
		Labels:      []plan.Label{{Key: "a.role", Value: "runtime"}, {Key: "a.version", Value: "0.1.0"}},
		NoCache:     noCache,
		Files: []plan.File{
			{Name: "Dockerfile", Label: "Dockerfile", Data: []byte("FROM scratch\n"), Mode: 0o644},
			{Name: "caged-entrypoint.sh", Label: "entrypoint", Data: []byte("#!/bin/sh\n"), Mode: 0o755},
		},
	}
}

func TestBuildImageWritesContextRunsDockerBuildAndCleansUp(t *testing.T) {
	t.Parallel()

	for _, noCache := range []bool{false, true} {
		exec, fsys := &fakeExecutor{}, &fakeFS{}
		stdio := system.Stdio{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
		id, err := newClient(exec, fsys, linux, stdio).BuildImage(context.Background(), testBuildSpec(noCache))
		if err != nil {
			t.Fatal(err)
		}
		if id != "sha256:fake-id" {
			t.Fatalf("BuildImage() id = %q, want the trimmed contents of the --iidfile", id)
		}
		wantFiles := map[string]writtenFile{
			"/tmp/caged-runtime-123/Dockerfile":          {data: "FROM scratch\n", mode: 0o644},
			"/tmp/caged-runtime-123/caged-entrypoint.sh": {data: "#!/bin/sh\n", mode: 0o755},
		}
		if !reflect.DeepEqual(fsys.written, wantFiles) {
			t.Fatalf("written = %+v, want %+v", fsys.written, wantFiles)
		}
		if len(exec.commands) != 1 || exec.commands[0].Args[0] != "build" {
			t.Fatalf("commands = %+v, want only docker build (the daemon check is the caller's job)", exec.commands)
		}
		build := exec.commands[0]
		wantArgs := []string{
			"build", "-t", "caged-runtime:latest",
			"--label", "a.role=runtime", "--label", "a.version=0.1.0",
			"--iidfile", "/tmp/caged-runtime-123/image-id",
			"-f", "/tmp/caged-runtime-123/Dockerfile", "/tmp/caged-runtime-123",
		}
		if noCache {
			wantArgs = []string{
				"build", "-t", "caged-runtime:latest",
				"--label", "a.role=runtime", "--label", "a.version=0.1.0",
				"--iidfile", "/tmp/caged-runtime-123/image-id",
				"--no-cache", "-f", "/tmp/caged-runtime-123/Dockerfile", "/tmp/caged-runtime-123",
			}
		}
		if build.Path != "/usr/local/bin/docker" || !reflect.DeepEqual(build.Args, wantArgs) {
			t.Fatalf("build = %+v, want args %q", build, wantArgs)
		}
		if build.Stdin != stdio.In || build.Stdout != stdio.Out || build.Stderr != stdio.Err {
			t.Fatal("docker build is not connected to the client's stdio")
		}
		if !reflect.DeepEqual(fsys.removed, []string{"/tmp/caged-runtime-123"}) {
			t.Fatalf("removed = %v, want the build directory", fsys.removed)
		}
	}
}

func TestBuildImageErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	t.Run("temporary directory", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{}
		_, err := newClient(exec, &fakeFS{mkdirErr: boom}, linux, system.Stdio{}).BuildImage(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != "create temporary build directory: boom" || len(exec.commands) != 0 {
			t.Fatalf("BuildImage() error = %v, commands = %+v", err, exec.commands)
		}
	})
	t.Run("write uses the file label", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{writeErr: map[string]error{"/tmp/caged-runtime-123/caged-entrypoint.sh": boom}}
		_, err := newClient(&fakeExecutor{}, fsys, linux, system.Stdio{}).BuildImage(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != "write build file entrypoint: boom" || len(fsys.removed) != 1 {
			t.Fatalf("BuildImage() error = %v, removed = %v", err, fsys.removed)
		}
	})
	t.Run("docker build failure is returned unchanged", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{}
		exec := &fakeExecutor{run: func(cmd system.Command) error {
			if cmd.Args[0] == "build" {
				return boom
			}
			return nil
		}}
		if _, err := newClient(exec, fsys, linux, system.Stdio{}).BuildImage(context.Background(), testBuildSpec(false)); err != boom || len(fsys.removed) != 1 { //nolint:errorlint // asserts the error is returned unchanged, not wrapped
			t.Fatalf("BuildImage() error = %v, removed = %v", err, fsys.removed)
		}
	})
	t.Run("a missing image ID file is an error", func(t *testing.T) {
		t.Parallel()
		id, err := newClient(&fakeExecutor{}, &fakeFS{readErr: boom}, linux, system.Stdio{}).BuildImage(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != "read the new image ID: boom" || id != "" {
			t.Fatalf("BuildImage() = %q, %v", id, err)
		}
	})
	t.Run("cleanup failure is returned after a successful build", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{removeErr: boom}
		_, err := newClient(&fakeExecutor{}, fsys, linux, system.Stdio{}).BuildImage(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != "remove temporary build directory: boom" || !errors.Is(err, boom) {
			t.Fatalf("BuildImage() error = %v, want the cleanup error", err)
		}
	})
	t.Run("cleanup failure does not hide a build failure", func(t *testing.T) {
		t.Parallel()
		buildErr := errors.New("build failed")
		exec := &fakeExecutor{run: func(cmd system.Command) error {
			if cmd.Args[0] == "build" {
				return buildErr
			}
			return nil
		}}
		_, err := newClient(exec, &fakeFS{removeErr: boom}, linux, system.Stdio{}).BuildImage(context.Background(), testBuildSpec(false))
		if err != buildErr { //nolint:errorlint // asserts the error is returned unchanged, not wrapped
			t.Fatalf("BuildImage() error = %v, want the build error unchanged", err)
		}
	})
	t.Run("missing CLI creates nothing", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{}
		_, err := newClient(&fakeExecutor{lookPathErr: boom}, fsys, linux, system.Stdio{}).BuildImage(context.Background(), testBuildSpec(false))
		if !engine.IsKind(err, engine.KindNotInstalled) || fsys.written != nil || fsys.removed != nil {
			t.Fatalf("BuildImage() error = %v, fs = %+v", err, fsys)
		}
	})
}
