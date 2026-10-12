package runtime

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed Dockerfile
var runtimeDockerfile []byte

//go:embed caged-entrypoint.sh
var runtimeEntrypoint []byte

// BuildRuntimeImage creates the local image from its embedded runtime recipe.
func BuildRuntimeImage(noCache bool, checkDocker func() error) error {
	if checkDocker == nil {
		return fmt.Errorf("Docker availability check is required")
	}
	if err := checkDocker(); err != nil {
		return err
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("Docker is not installed or not on PATH: %w", err)
	}
	dir, err := os.MkdirTemp("", "caged-runtime-")
	if err != nil {
		return fmt.Errorf("create temporary build directory: %w", err)
	}
	defer os.RemoveAll(dir)
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, runtimeDockerfile, 0o644); err != nil {
		return fmt.Errorf("write embedded Dockerfile: %w", err)
	}
	entrypointPath := filepath.Join(dir, "caged-entrypoint.sh")
	if err := os.WriteFile(entrypointPath, runtimeEntrypoint, 0o755); err != nil {
		return fmt.Errorf("write embedded entrypoint: %w", err)
	}
	args := []string{"build", "-t", Image}
	if noCache {
		args = append(args, "--no-cache")
	}
	args = append(args, "-f", dockerfilePath, dir)
	cmd := exec.Command(docker, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
