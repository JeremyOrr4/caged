package docker

import (
	"fmt"
	"strings"

	"github.com/JeremyOrr4/caged/internal/system"
)

// This file holds every platform-specific Docker rule. The platform is
// injected, so the Linux and macOS branches are both tested on any host.

// checkArch rejects hosts that have no matching runtime image architecture.
func checkArch(platform system.Platform) error {
	if platform.Arch != "amd64" && platform.Arch != "arm64" {
		return fmt.Errorf("unsupported host architecture %q; caged supports amd64 and arm64 images", platform.Arch)
	}
	return nil
}

// userArgs runs the container as the host user on Linux, so files created in
// bind mounts belong to that user. Docker Desktop on macOS translates
// bind-mount ownership itself, so the image's own user is kept there.
func userArgs(platform system.Platform) []string {
	if platform.OS == "linux" {
		return []string{"--user", fmt.Sprintf("%d:%d", platform.UID, platform.GID)}
	}
	return nil
}

// daemonLauncher names what the user has to start when the daemon is down.
func daemonLauncher(platform system.Platform) string {
	if platform.OS == "darwin" {
		return "start Docker Desktop"
	}
	return "start the Docker daemon"
}

// explainRunError adds the Docker Desktop File Sharing hint on macOS when the
// project path is not shared with Docker Desktop's VM.
func explainRunError(platform system.Platform, stderr string, err error) error {
	if platform.OS == "darwin" && looksLikeSharedPathError(stderr) {
		return fmt.Errorf("Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: %w", err)
	}
	return err
}

func looksLikeSharedPathError(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "mounts denied") || strings.Contains(message, "not shared") || strings.Contains(message, "mount denied")
}
