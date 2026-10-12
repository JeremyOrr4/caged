package docker

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func ImageExists(image string) bool {
	cmd := exec.Command("docker", "image", "inspect", image)
	return cmd.Run() == nil
}

// CheckDocker distinguishes a missing CLI from a stopped Docker daemon.
func CheckDocker() error {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return fmt.Errorf("unsupported host architecture %q; caged supports amd64 and arm64 images", runtime.GOARCH)
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("Docker is not installed or not on PATH: %w", err)
	}
	cmd := exec.Command(docker, "info", "--format", "{{.ServerVersion}}")
	if output, err := cmd.CombinedOutput(); err != nil {
		launcher := "start the Docker daemon"
		if runtime.GOOS == "darwin" {
			launcher = "start Docker Desktop"
		}
		return fmt.Errorf("Docker is installed but the daemon is unavailable; %s: %s", launcher, strings.TrimSpace(string(output)))
	}
	return nil
}
