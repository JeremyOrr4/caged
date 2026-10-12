package container

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/JeremyOrr4/caged/internal/config"
)

// Options configures a disposable Docker container.
type Options struct {
	Image        string
	Command      []string
	Mount        *Mount
	ConfigMounts []config.ResolvedMount
	EnvVars      []string
	VolumeMounts []VolumeMount
}

// Mount maps a host directory into a container directory.
type Mount struct {
	HostPath      string
	ContainerPath string
}

type VolumeMount struct {
	Name          string
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
	for _, mount := range options.ConfigMounts {
		args = append(args,
			"-v", mount.HostPath+":"+mount.StagedPath+":ro",
		)
	}
	for _, volume := range options.VolumeMounts {
		args = append(args,
			"-v", volume.Name+":"+volume.ContainerPath,
		)
	}
	if goos == "linux" {
		args = append(args, "--user", fmt.Sprintf("%d:%d", uid, gid))
	}
	if len(options.ConfigMounts) > 0 {
		args = append(args, "-e", "CAGED_CONFIG_MAP="+configEnvJSON(options.ConfigMounts))
	}
	for _, env := range options.EnvVars {
		args = append(args, "-e", env)
	}

	args = append(args, options.Image)
	return append(args, options.Command...)
}

func ResetVolumesForAgent(agentName string, yes bool) error {
	return resetVolume(config.StateVolumeName(agentName))
}

func ResetVolumes(all bool, yes bool) error {
	if !all {
		return fmt.Errorf("reset requires --all or an agent name")
	}
	out, err := exec.Command("docker", "volume", "ls", "--format", "{{.Name}}").CombinedOutput()
	if err != nil {
		return fmt.Errorf("list docker volumes: %w", err)
	}
	volumes := splitVolumeNames(string(out))
	for _, volume := range volumes {
		if err := resetVolume(volume); err != nil {
			return err
		}
	}
	return nil
}

func splitVolumeNames(output string) []string {
	lines := make([]string, 0)
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.HasPrefix(trimmed, "caged-") || !strings.HasSuffix(trimmed, "-home") {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines
}

func resetVolume(volumeName string) error {
	if volumeName == "" {
		return nil
	}
	cmd := exec.Command("docker", "volume", "rm", volumeName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("remove docker volume %s: %w: %s", volumeName, err, strings.TrimSpace(string(out)))
	}
	return nil
}
