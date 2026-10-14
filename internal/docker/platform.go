package docker

import (
	"fmt"
	"strings"

	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/system"
)

// This file holds every platform-specific Docker rule. The platform is
// injected, so the Linux and macOS branches are both tested on any host.

// checkArch rejects hosts that have no matching runtime image architecture.
func checkArch(platform system.Platform) error {
	if platform.Arch != "amd64" && platform.Arch != "arm64" {
		return &engine.Error{Kind: engine.KindUnsupportedHost, Message: fmt.Sprintf("unsupported host architecture %q; caged supports amd64 and arm64 images", platform.Arch)}
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

// sharedPathHint adds the Docker Desktop File Sharing hint on macOS when the
// project path is not shared with Docker Desktop's VM. It returns nil when
// the hint does not apply.
func sharedPathHint(platform system.Platform, stderr string, err error) error {
	if platform.OS == "darwin" && looksLikeSharedPathError(stderr) {
		return &engine.Error{
			Kind:    engine.KindUnknown,
			Message: "Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing",
			Err:     err,
		}
	}
	return nil
}

func looksLikeSharedPathError(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "mounts denied") || strings.Contains(message, "not shared") || strings.Contains(message, "mount denied")
}

// infoFormat asks `docker info` for the three facts caged needs, separated by
// "|".
const infoFormat = "{{.ServerVersion}}|{{.OperatingSystem}}|{{.Architecture}}"

// parseInfo reads the reply to infoFormat. The architecture is normalized to
// the GOARCH names caged uses.
func parseInfo(docker, output string) (engine.DaemonInfo, error) {
	fields := strings.Split(strings.TrimSpace(output), "|")
	if len(fields) != 3 {
		return engine.DaemonInfo{}, fmt.Errorf("unexpected docker info output %q", strings.TrimSpace(output))
	}
	arch := fields[2]
	switch arch {
	case "x86_64":
		arch = "amd64"
	case "aarch64":
		arch = "arm64"
	}
	return engine.DaemonInfo{CLIPath: docker, ServerVersion: fields[0], OperatingSystem: fields[1], Arch: arch}, nil
}

// looksLikeMissingImage recognizes docker's "no such image" reply.
func looksLikeMissingImage(stderr string) bool {
	return strings.Contains(strings.ToLower(stderr), "no such image")
}

// looksLikeImageInUse recognizes docker's refusal to remove an image that a
// container or another tag still uses.
func looksLikeImageInUse(stderr string) bool {
	stderr = strings.ToLower(stderr)
	return strings.Contains(stderr, "conflict") || strings.Contains(stderr, "being used") || strings.Contains(stderr, "is using")
}
