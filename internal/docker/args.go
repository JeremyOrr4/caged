package docker

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// runArgs translates a run plan into `docker run` arguments. Masked config
// files are covered with fileMaskSource, an empty file; "" means /dev/null.
func runArgs(spec plan.RunSpec, platform system.Platform, fileMaskSource string) []string {
	if fileMaskSource == "" {
		fileMaskSource = "/dev/null"
	}
	args := []string{"run", "-it", "--rm"}
	for _, port := range spec.Ports {
		args = append(args, "-p", fmt.Sprintf("%s:%d:%d", port.HostIP, port.HostPort, port.ContainerPort))
	}

	if spec.Mount != nil {
		args = append(args,
			"-v", spec.Mount.HostPath+":"+spec.Mount.ContainerPath,
			"-w", spec.Mount.ContainerPath,
		)
	}
	for _, mount := range spec.ConfigMounts {
		args = append(args, configMountArgs(mount, fileMaskSource)...)
	}
	args = append(args, userArgs(platform)...)
	for _, env := range spec.EnvVars {
		args = append(args, "-e", env)
	}
	if spec.ReturnToShell {
		args = append(args, "-e", "CAGED_RETURN_TO_SHELL=1")
	}

	args = append(args, spec.Image)
	return append(args, spec.Command...)
}

// configMountArgs mounts one config mount and then hides its masks inside
// it: directories under a tmpfs, files under a read-only bind of
// fileMaskSource. Docker orders mounts by destination depth itself; the
// argument order here is kept only because the golden command lines pin it.
func configMountArgs(mount plan.ConfigMount, fileMaskSource string) []string {
	volume := mount.HostPath + ":" + mount.DestPath
	if !mount.Writable {
		volume += ":ro"
	}
	args := []string{"-v", volume}
	for _, mask := range mount.MaskPaths {
		args = append(args, "--tmpfs", maskTarget(mount.DestPath, mask))
	}
	for _, mask := range mount.MaskFiles {
		args = append(args, "-v", fileMaskSource+":"+maskTarget(mount.DestPath, mask)+":ro")
	}
	return args
}

// maskTarget is the container path of mask, a path relative to the mount
// destination dest; a leading slash on mask is ignored.
func maskTarget(dest, mask string) string {
	return strings.TrimSuffix(dest, "/") + "/" + strings.TrimPrefix(filepath.ToSlash(mask), "/")
}

// buildArgs translates a build plan into `docker build` arguments for the
// build context written to dir. idFile is where docker writes the new image ID.
func buildArgs(spec plan.BuildSpec, dir, idFile string) []string {
	args := []string{"build", "-t", spec.Image}
	for _, label := range spec.Labels {
		args = append(args, "--label", label.Key+"="+label.Value)
	}
	args = append(args, "--iidfile", idFile)
	if spec.NoCache {
		args = append(args, "--no-cache")
	}
	return append(args, "-f", filepath.Join(dir, spec.Dockerfile), dir)
}
