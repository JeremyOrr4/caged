package docker

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// Options configures a disposable Docker container.
type Options = plan.RunSpec

// Mount maps a host directory into a container directory.
type Mount = plan.Mount

// Run starts an interactive, disposable Docker container.
func Run(options Options) error {
	if err := CheckDocker(); err != nil {
		return err
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found, is Docker installed and running?: %w", err)
	}

	args := buildArgs(options, runtime.GOOS, os.Getuid(), os.Getgid())
	cmd := exec.Command(docker, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	var stderr bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	if err := cmd.Run(); err != nil {
		if runtime.GOOS == "darwin" && looksLikeSharedPathError(stderr.String()) {
			return fmt.Errorf("Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: %w", err)
		}
		return err
	}
	return nil
}

func looksLikeSharedPathError(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "mounts denied") || strings.Contains(message, "not shared") || strings.Contains(message, "mount denied")
}

func buildArgs(options Options, goos string, uid, gid int) []string {
	args := []string{"run", "-it", "--rm"}

	if options.Mount != nil {
		args = append(args,
			"-v", options.Mount.HostPath+":"+options.Mount.ContainerPath,
			"-w", options.Mount.ContainerPath,
		)
	}
	for _, mount := range options.ConfigMounts {
		volume := mount.HostPath + ":" + mount.DestPath
		if !mount.Writable {
			volume += ":ro"
		}
		args = append(args, "-v", volume)
	}
	if goos == "linux" {
		args = append(args, "--user", fmt.Sprintf("%d:%d", uid, gid))
	}
	for _, env := range options.EnvVars {
		args = append(args, "-e", env)
	}
	if options.ReturnToShell {
		args = append(args, "-e", "CAGED_RETURN_TO_SHELL=1")
	}

	args = append(args, options.Image)
	return append(args, options.Command...)
}
