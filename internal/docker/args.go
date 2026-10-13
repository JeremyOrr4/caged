package docker

import (
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// runArgs translates a run plan into `docker run` arguments.
func runArgs(spec plan.RunSpec, platform system.Platform) []string {
	args := []string{"run", "-it", "--rm"}

	if spec.Mount != nil {
		args = append(args,
			"-v", spec.Mount.HostPath+":"+spec.Mount.ContainerPath,
			"-w", spec.Mount.ContainerPath,
		)
	}
	for _, mount := range spec.ConfigMounts {
		volume := mount.HostPath + ":" + mount.DestPath
		if !mount.Writable {
			volume += ":ro"
		}
		args = append(args, "-v", volume)
		for _, mask := range mount.MaskPaths {
			maskedPath := strings.TrimSuffix(mount.DestPath, "/") + "/" + strings.TrimPrefix(filepath.ToSlash(mask), "/")
			args = append(args, "--tmpfs", maskedPath)
		}
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

// buildArgs translates a build plan into `docker build` arguments for the
// build context written to dir.
func buildArgs(spec plan.BuildSpec, dir string) []string {
	args := []string{"build", "-t", spec.Image}
	if spec.NoCache {
		args = append(args, "--no-cache")
	}
	return append(args, "-f", filepath.Join(dir, spec.Dockerfile), dir)
}
