package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/configstage"
	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// Executor runs host processes. system.Executor is the production implementation.
type Executor interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, cmd system.Command) error
}

// FileSystem holds temporary build contexts and mask files. system.FS is the
// production implementation.
type FileSystem interface {
	MkdirTemp(dir, pattern string) (string, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	ReadFile(name string) ([]byte, error)
	RemoveAll(path string) error
}

// ConfigStager runs staged config mounts from copies, and writes the
// container's changes back afterwards. *configstage.Stager implements it.
type ConfigStager interface {
	Stage(mounts []plan.ConfigMount) (*configstage.Session, error)
}

// Deps are the collaborators of a Client.
type Deps struct {
	Exec     Executor
	FS       FileSystem
	Stager   ConfigStager // nil stages nothing
	Platform system.Platform
	Stdio    system.Stdio // connected to builds and containers
}

// Client drives the docker CLI.
type Client struct {
	exec     Executor
	fs       FileSystem
	stager   ConfigStager
	platform system.Platform
	stdio    system.Stdio
}

// NewClient returns a Client that runs docker through deps.Exec, writes
// temporary files through deps.FS, stages config mounts with deps.Stager,
// applies the rules for deps.Platform, and connects containers to deps.Stdio.
func NewClient(deps Deps) *Client {
	stager := deps.Stager
	if stager == nil {
		stager = noStaging{}
	}
	return &Client{exec: deps.Exec, fs: deps.FS, stager: stager, platform: deps.Platform, stdio: deps.Stdio}
}

// noStaging is the ConfigStager of a Client built without one.
type noStaging struct{}

func (noStaging) Stage(mounts []plan.ConfigMount) (*configstage.Session, error) {
	return configstage.Unstaged(mounts), nil
}

// dockerPath resolves the docker CLI. Every docker call goes through it, so a
// missing CLI always produces the same error.
func (c *Client) dockerPath() (string, error) {
	docker, err := c.exec.LookPath("docker")
	if err != nil {
		return "", &engine.Error{Kind: engine.KindNotInstalled, Message: "Docker is not installed or not on PATH", Err: err}
	}
	return docker, nil
}

// Daemon checks the host, the docker CLI, and the daemon, and returns what the
// daemon reports. It distinguishes an unsupported host, a missing docker CLI,
// and a stopped daemon.
func (c *Client) Daemon(ctx context.Context) (engine.DaemonInfo, error) {
	if err := checkArch(c.platform); err != nil {
		return engine.DaemonInfo{}, err
	}
	docker, err := c.dockerPath()
	if err != nil {
		return engine.DaemonInfo{}, err
	}
	var stdout, stderr bytes.Buffer
	info := system.Command{Path: docker, Args: []string{"info", "--format", infoFormat}, Stdout: &stdout, Stderr: &stderr}
	if err := c.exec.Run(ctx, info); err != nil {
		output := strings.TrimSpace(stderr.String())
		if output == "" {
			output = strings.TrimSpace(stdout.String())
		}
		cause := err
		if output != "" {
			cause = errors.New(output)
		}
		return engine.DaemonInfo{}, &engine.Error{
			Kind:    engine.KindDaemonDown,
			Message: "Docker is installed but the daemon is unavailable; " + daemonLauncher(c.platform),
			Err:     cause,
		}
	}
	return parseInfo(docker, stdout.String())
}

// InspectImage returns the facts about a local image. A missing image is an
// engine.KindImageNotFound error.
func (c *Client) InspectImage(ctx context.Context, ref string) (engine.ImageInfo, error) {
	docker, err := c.dockerPath()
	if err != nil {
		return engine.ImageInfo{}, err
	}
	var stdout, stderr bytes.Buffer
	inspect := system.Command{Path: docker, Args: []string{"image", "inspect", ref}, Stdout: &stdout, Stderr: &stderr}
	if err := c.exec.Run(ctx, inspect); err != nil {
		if looksLikeMissingImage(stderr.String()) {
			return engine.ImageInfo{}, &engine.Error{Kind: engine.KindImageNotFound, Message: "image " + ref + " not found", Err: err}
		}
		return engine.ImageInfo{}, fmt.Errorf("inspect image %s: %s: %w", ref, strings.TrimSpace(stderr.String()), err)
	}
	return parseInspect(stdout.Bytes())
}

// BuildImage writes the build context to a temporary directory, runs docker
// build with the spec's labels, and removes the directory again. It returns the
// ID of the new image, which docker writes to an --iidfile. A failure to remove
// the directory is returned when the build itself succeeded.
func (c *Client) BuildImage(ctx context.Context, spec plan.BuildSpec) (id string, err error) {
	docker, err := c.dockerPath()
	if err != nil {
		return "", err
	}
	dir, err := c.fs.MkdirTemp("", spec.ContextName+"-")
	if err != nil {
		return "", fmt.Errorf("create temporary build directory: %w", err)
	}
	defer func() {
		if removeErr := c.fs.RemoveAll(dir); removeErr != nil && err == nil {
			err = fmt.Errorf("remove temporary build directory: %w", removeErr)
		}
		if err != nil {
			id = ""
		}
	}()
	for _, file := range spec.Files {
		if err := c.fs.WriteFile(filepath.Join(dir, file.Name), file.Data, file.Mode); err != nil {
			return "", fmt.Errorf("write build file %s: %w", file.Label, err)
		}
	}
	idFile := filepath.Join(dir, "image-id")
	err = c.exec.Run(ctx, system.Command{
		Path:   docker,
		Args:   buildArgs(spec, dir, idFile),
		Stdin:  c.stdio.In,
		Stdout: c.stdio.Out,
		Stderr: c.stdio.Err,
	})
	if err != nil {
		return "", err
	}
	data, err := c.fs.ReadFile(idFile)
	if err != nil {
		return "", fmt.Errorf("read the new image ID: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// RemoveImage deletes a local image by ID, or removes one tag by
// repository:tag, never with -f. A missing image is an
// engine.KindImageNotFound error, and an image that a container or another
// tag still uses is an engine.KindImageInUse error.
func (c *Client) RemoveImage(ctx context.Context, id string) error {
	docker, err := c.dockerPath()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	remove := system.Command{Path: docker, Args: []string{"image", "rm", id}, Stderr: &stderr}
	if err := c.exec.Run(ctx, remove); err != nil {
		message := strings.TrimSpace(stderr.String())
		if looksLikeMissingImage(message) {
			return &engine.Error{Kind: engine.KindImageNotFound, Message: message, Err: err}
		}
		if looksLikeImageInUse(message) {
			return &engine.Error{Kind: engine.KindImageInUse, Message: message, Err: err}
		}
		return fmt.Errorf("remove image %s: %s: %w", id, message, err)
	}
	return nil
}

// ReadImageFile returns the contents of a file inside a local image. It runs
// `docker run --rm --entrypoint cat <ref> <path>`, which starts no agent.
func (c *Client) ReadImageFile(ctx context.Context, ref, path string) ([]byte, error) {
	docker, err := c.dockerPath()
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	read := system.Command{Path: docker, Args: []string{"run", "--rm", "--entrypoint", "cat", ref, path}, Stdout: &stdout, Stderr: &stderr}
	if err := c.exec.Run(ctx, read); err != nil {
		return nil, fmt.Errorf("read %s from image %s: %s: %w", path, ref, strings.TrimSpace(stderr.String()), err)
	}
	return stdout.Bytes(), nil
}

// ProbeMount checks that docker can mount m read-only into a container, the way
// a run would, by starting a container that does nothing. On macOS, a path
// that Docker Desktop does not share gets the File Sharing hint.
func (c *Client) ProbeMount(ctx context.Context, ref string, m plan.Mount) error {
	docker, err := c.dockerPath()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	args := []string{"run", "--rm", "-v", m.HostPath + ":" + m.ContainerPath + ":ro", "--entrypoint", "true", ref}
	if err := c.exec.Run(ctx, system.Command{Path: docker, Args: args, Stderr: &stderr}); err != nil {
		if hinted := sharedPathHint(c.platform, stderr.String(), err); hinted != nil {
			return hinted
		}
		return fmt.Errorf("%s: %w", strings.TrimSpace(stderr.String()), err)
	}
	return nil
}

// ListImages lists local images, either those with a label (q.Label is "key"
// or "key=value") or those in a repository (q.Reference).
func (c *Client) ListImages(ctx context.Context, q engine.ImageQuery) ([]engine.ImageSummary, error) {
	docker, err := c.dockerPath()
	if err != nil {
		return nil, err
	}
	args := []string{"image", "ls"}
	if q.Label != "" {
		args = append(args, "--filter", "label="+q.Label)
	}
	if q.Reference != "" {
		args = append(args, q.Reference)
	}
	args = append(args, "--format", "{{json .}}")
	var stdout, stderr bytes.Buffer
	if err := c.exec.Run(ctx, system.Command{Path: docker, Args: args, Stdout: &stdout, Stderr: &stderr}); err != nil {
		return nil, fmt.Errorf("list images: %s: %w", strings.TrimSpace(stderr.String()), err)
	}
	return parseImageList(stdout.Bytes())
}
