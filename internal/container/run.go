package container

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// Options configures a disposable Docker container.
type Options struct {
	Image   string
	Command []string
	Mount   *Mount
}

// Mount maps a host directory into a container directory.
type Mount struct {
	HostPath      string
	ContainerPath string
}

// Run starts an interactive, disposable Docker container.
func Run(options Options) error {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found, is Docker installed and running?: %w", err)
	}

	cmd := exec.Command(docker, buildArgs(options, runtime.GOOS, os.Getuid(), os.Getgid())...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func buildArgs(options Options, goos string, uid, gid int) []string {
	args := []string{"run", "-it", "--rm"}

	if options.Mount != nil {
		args = append(args,
			"-v", options.Mount.HostPath+":"+options.Mount.ContainerPath,
			"-w", options.Mount.ContainerPath,
		)
	}
	if goos == "linux" {
		args = append(args, "--user", fmt.Sprintf("%d:%d", uid, gid))
	}

	args = append(args, options.Image)
	return append(args, options.Command...)
}
