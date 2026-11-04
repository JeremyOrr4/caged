package docker

import (
	"fmt"
	"strings"

	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// classifyRunError turns a failed `docker run` into the most helpful error it
// can: the Docker Desktop File Sharing hint, or a port-in-use message. Any
// other failure is returned unchanged, so its exit code passes through.
func classifyRunError(platform system.Platform, spec plan.RunSpec, stderr string, err error) error {
	if hinted := sharedPathHint(platform, stderr, err); hinted != nil {
		return hinted
	}
	if len(spec.Ports) > 0 && looksLikePortInUse(stderr) {
		return &engine.Error{Kind: engine.KindPortInUse, Message: portInUseMessage(spec.Ports, stderr), Err: err}
	}
	return err
}

// looksLikePortInUse recognizes the stderr of Docker Engine and Docker Desktop
// when a host port is taken.
func looksLikePortInUse(stderr string) bool {
	stderr = strings.ToLower(stderr)
	return strings.Contains(stderr, "address already in use") ||
		strings.Contains(stderr, "port is already allocated") ||
		strings.Contains(stderr, "ports are not available")
}

// portInUseMessage names the published host port that docker reported, or
// says that one of them is taken when stderr does not name it.
func portInUseMessage(ports []plan.Port, stderr string) string {
	for _, port := range ports {
		if mentionsHostPort(stderr, port) {
			alternative := port.HostPort + 1
			if alternative > plan.MaxPort {
				alternative = port.HostPort - 1
			}
			return fmt.Sprintf("host port %d is already in use on %s; stop the process using it or pick another host port, e.g. -p %d:%d",
				port.HostPort, port.HostIP, alternative, port.ContainerPort)
		}
	}
	return "a published host port is already in use"
}

// mentionsHostPort reports whether stderr contains "<ip>:<port>" for the
// port, and not just as the start of a longer port number.
func mentionsHostPort(stderr string, port plan.Port) bool {
	needle := fmt.Sprintf("%s:%d", port.HostIP, port.HostPort)
	for rest := stderr; ; {
		i := strings.Index(rest, needle)
		if i < 0 {
			return false
		}
		end := i + len(needle)
		if end >= len(rest) || rest[end] < '0' || rest[end] > '9' {
			return true
		}
		rest = rest[i+1:]
	}
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
