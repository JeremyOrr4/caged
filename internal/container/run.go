package container

import (
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
func Run(options Options) error {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found, is Docker installed and running?: %w", err)
	}

	args, err := buildArgs(options, runtime.GOOS, os.Getuid(), os.Getgid())
	if err != nil {
		return err
	}
	cmd := exec.Command(docker, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func buildArgs(options Options, goos string, uid, gid int) ([]string, error) {
	args := []string{"run", "-it", "--rm"}

	if options.Mount != nil {
		args = append(args,
			"-v", options.Mount.HostPath+":"+options.Mount.ContainerPath,
			"-w", options.Mount.ContainerPath,
		)
	}
	hasWritableConfig := false
	for _, mount := range options.ConfigMounts {
		if mount.Writable {
			args = append(args, "-v", mount.HostPath+":"+mount.DestPath)
			hasWritableConfig = true
			continue
		}
		args = append(args,
			"-v", mount.HostPath+":"+mount.StagedPath+":ro",
		)
	}
	if goos == "linux" || (goos == "darwin" && hasWritableConfig) {
		args = append(args, "--user", fmt.Sprintf("%d:%d", uid, gid))
	}
	stagedMounts := make([]config.ResolvedMount, 0, len(options.ConfigMounts))
	for _, mount := range options.ConfigMounts {
		if !mount.Writable {
			stagedMounts = append(stagedMounts, mount)
		}
	}
	if len(stagedMounts) > 0 {
		configMap, err := configEnvJSON(stagedMounts)
		if err != nil {
			return nil, err
		}
		args = append(args, "-e", "CAGED_CONFIG_MAP="+configMap)
	}
	for _, env := range options.EnvVars {
		args = append(args, "-e", env)
	}
	if options.ReturnToShell {
		args = append(args, "-e", "CAGED_RETURN_TO_SHELL=1")
	}

	args = append(args, options.Image)
	return append(args, options.Command...), nil
}
