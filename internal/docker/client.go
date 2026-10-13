package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// Executor runs host processes. system.Executor is the production implementation.
type Executor interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, cmd system.Command) error
}

// FileSystem holds the temporary build context. system.FS is the production implementation.
type FileSystem interface {
	MkdirTemp(dir, pattern string) (string, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	RemoveAll(path string) error
}

// Client drives the docker CLI.
type Client struct {
	exec     Executor
	fs       FileSystem
	platform system.Platform
	stdio    system.Stdio
}

// NewClient returns a Client that runs docker through exec, writes build
// contexts through fsys, applies the rules for platform, and connects
// containers to stdio.
func NewClient(exec Executor, fsys FileSystem, platform system.Platform, stdio system.Stdio) *Client {
	return &Client{exec: exec, fs: fsys, platform: platform, stdio: stdio}
}

// Check distinguishes a missing docker CLI from a stopped Docker daemon.
func (c *Client) Check(ctx context.Context) error {
	if err := checkArch(c.platform); err != nil {
		return err
	}
	docker, err := c.exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("Docker is not installed or not on PATH: %w", err)
	}
	var output bytes.Buffer
	info := system.Command{Path: docker, Args: []string{"info", "--format", "{{.ServerVersion}}"}, Stdout: &output, Stderr: &output}
	if err := c.exec.Run(ctx, info); err != nil {
		return fmt.Errorf("Docker is installed but the daemon is unavailable; %s: %s", daemonLauncher(c.platform), strings.TrimSpace(output.String()))
	}
	return nil
}

// ImageExists reports whether image is in the local image store.
func (c *Client) ImageExists(ctx context.Context, image string) bool {
	return c.exec.Run(ctx, system.Command{Path: "docker", Args: []string{"image", "inspect", image}}) == nil
}

// Build writes the build context to a temporary directory, runs docker build,
// and removes the directory again.
func (c *Client) Build(ctx context.Context, spec plan.BuildSpec) error {
	if err := c.Check(ctx); err != nil {
		return err
	}
	docker, err := c.exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("Docker is not installed or not on PATH: %w", err)
	}
	dir, err := c.fs.MkdirTemp("", "caged-runtime-")
	if err != nil {
		return fmt.Errorf("create temporary build directory: %w", err)
	}
	defer c.fs.RemoveAll(dir)
	for _, file := range spec.Files {
		if err := c.fs.WriteFile(filepath.Join(dir, file.Name), file.Data, file.Mode); err != nil {
			return fmt.Errorf("write embedded %s: %w", file.Label, err)
		}
	}
	return c.exec.Run(ctx, system.Command{
		Path:   docker,
		Args:   buildArgs(spec, dir),
		Stdin:  c.stdio.In,
		Stdout: c.stdio.Out,
		Stderr: c.stdio.Err,
	})
}

// Run starts an interactive, disposable container and waits for it to exit.
// A non-zero container exit is returned unchanged so its code can be passed on.
func (c *Client) Run(ctx context.Context, spec plan.RunSpec) error {
	if err := c.Check(ctx); err != nil {
		return err
	}
	docker, err := c.exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found, is Docker installed and running?: %w", err)
	}
	var stderr bytes.Buffer
	err = c.exec.Run(ctx, system.Command{
		Path:   docker,
		Args:   runArgs(spec, c.platform),
		Stdin:  c.stdio.In,
		Stdout: c.stdio.Out,
		Stderr: io.MultiWriter(c.stdio.Err, &stderr),
	})
	if err != nil {
		return explainRunError(c.platform, stderr.String(), err)
	}
	return nil
}
