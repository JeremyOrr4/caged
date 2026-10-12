package container

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/config"
)

//go:embed runtime/Dockerfile
var runtimeDockerfile []byte

//go:embed runtime/caged-entrypoint.sh
var runtimeEntrypoint []byte

func ImageExists(image string) bool {
	cmd := exec.Command("docker", "image", "inspect", image)
	return cmd.Run() == nil
}

func BuildRuntimeImage(noCache bool) error {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found, is Docker installed and running?: %w", err)
	}

	dir, err := os.MkdirTemp("", "caged-runtime-")
	if err != nil {
		return fmt.Errorf("create temp build dir: %w", err)
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

	args := []string{"build", "-t", config.RuntimeImage}
	if noCache {
		args = append(args, "--no-cache")
	}
	args = append(args, "-f", dockerfilePath, dir)

	cmd := exec.Command(docker, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func configEnvJSON(mounts []config.ResolvedMount) string {
	if len(mounts) == 0 {
		return ""
	}
	payload := make([]map[string]any, 0, len(mounts))
	for _, mount := range mounts {
		payload = append(payload, map[string]any{
			"agent":    mount.Agent,
			"name":     mount.Name,
			"host":     mount.HostPath,
			"staged":   mount.StagedPath,
			"dest":     mount.DestPath,
			"excludes": mount.Excludes,
		})
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}
