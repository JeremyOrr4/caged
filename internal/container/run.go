package container

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/JeremyOrr4/caged/internal/config"
)

// Options configures a disposable Docker container.
type Options struct {
	Image         string
	Command       []string
	Mount         *Mount
	ConfigMounts  []config.ResolvedMount
	EnvVars       []string
	ReturnToShell bool
}

// Mount maps a host directory into a container directory.
type Mount struct {
	HostPath      string
	ContainerPath string
}

// Run starts an interactive, disposable Docker container.
func Run(options Options) (runErr error) {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found, is Docker installed and running?: %w", err)
	}

	configMounts, cleanup, err := stageConfigs(options.ConfigMounts)
	if err != nil {
		return err
	}
	defer func() {
		if err := cleanup(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("remove staged configs: %w", err))
		}
	}()
	options.ConfigMounts = configMounts

	args := buildArgs(options, runtime.GOOS, os.Getuid(), os.Getgid())
	cmd := exec.Command(docker, args...)
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
	hasConfigMount := len(options.ConfigMounts) > 0
	for _, mount := range options.ConfigMounts {
		volume := mount.HostPath + ":" + mount.DestPath
		if !mount.Writable {
			volume += ":ro"
		}
		args = append(args, "-v", volume)
	}
	if goos == "linux" || (goos == "darwin" && hasConfigMount) {
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
