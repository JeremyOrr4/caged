package docker

import (
	"errors"
	"testing"

	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// exitError stands in for *exec.ExitError: it carries an exit code.
type exitError struct{ code int }

func (e exitError) Error() string { return "exit status 125" }
func (e exitError) ExitCode() int { return e.code }

func TestClassifyRunError(t *testing.T) {
	t.Parallel()

	ports := []plan.Port{
		{HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 3000},
		{HostIP: "127.0.0.1", HostPort: 80, ContainerPort: 80},
	}
	const (
		linuxBind    = "docker: Error response from daemon: failed to set up container networking: driver failed programming external connectivity: failed to bind host port 127.0.0.1:8080/tcp: address already in use"
		allocated    = "docker: Error response from daemon: driver failed programming external connectivity on endpoint x: Bind for 127.0.0.1:8080 failed: port is already allocated"
		unavailable  = "docker: Error response from daemon: Ports are not available: exposing port TCP 127.0.0.1:8080 -> 127.0.0.1:0: listen tcp4 127.0.0.1:8080: bind: address already in use"
		inUseMessage = "host port 8080 is already in use on 127.0.0.1; stop the process using it or pick another host port, e.g. -p 8081:3000: exit status 125"
		unnamed      = "a published host port is already in use: exit status 125"
	)
	tests := []struct {
		name     string
		platform system.Platform
		ports    []plan.Port
		stderr   string
		want     string // "" means the error is returned unchanged
		kind     engine.Kind
	}{
		{name: "Linux bind message", platform: linux, ports: ports, stderr: linuxBind, want: inUseMessage, kind: engine.KindPortInUse},
		{name: "already allocated message", platform: linux, ports: ports, stderr: allocated, want: inUseMessage, kind: engine.KindPortInUse},
		{name: "Docker Desktop message on macOS", platform: darwin, ports: ports, stderr: unavailable, want: inUseMessage, kind: engine.KindPortInUse},
		{name: "second port is the one that is taken", platform: linux, ports: ports, stderr: "failed to bind host port 127.0.0.1:80/tcp: address already in use", want: "host port 80 is already in use on 127.0.0.1; stop the process using it or pick another host port, e.g. -p 81:80: exit status 125", kind: engine.KindPortInUse},
		{name: "a longer port number is not a match", platform: linux, ports: ports[1:], stderr: "failed to bind host port 127.0.0.1:8080/tcp: address already in use", want: unnamed, kind: engine.KindPortInUse},
		{name: "no port named", platform: linux, ports: ports, stderr: "address already in use", want: unnamed, kind: engine.KindPortInUse},
		{name: "highest port suggests the one below", platform: linux, ports: []plan.Port{{HostIP: "127.0.0.1", HostPort: 65535, ContainerPort: 1}}, stderr: "127.0.0.1:65535 address already in use", want: "host port 65535 is already in use on 127.0.0.1; stop the process using it or pick another host port, e.g. -p 65534:1: exit status 125", kind: engine.KindPortInUse},
		{name: "port error text without published ports is not ours", platform: linux, stderr: linuxBind},
		{name: "unrelated failure", platform: linux, ports: ports, stderr: "docker: Error response from daemon: pull access denied"},
		{name: "shared path hint wins on macOS", platform: darwin, ports: ports, stderr: "Mounts denied: " + linuxBind, want: "Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: exit status 125"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			original := exitError{code: 125}
			got := classifyRunError(tt.platform, plan.RunSpec{Ports: tt.ports}, tt.stderr, original)
			if tt.want == "" {
				if got != error(original) {
					t.Fatalf("classifyRunError() = %v, want the original error", got)
				}
				return
			}
			if got.Error() != tt.want {
				t.Fatalf("classifyRunError() = %q, want %q", got, tt.want)
			}
			if tt.kind != engine.KindUnknown && !engine.IsKind(got, tt.kind) {
				t.Fatalf("classifyRunError() kind is not %v: %v", tt.kind, got)
			}
			var coder interface{ ExitCode() int }
			if !errors.As(got, &coder) || coder.ExitCode() != 125 {
				t.Fatalf("classifyRunError() lost the exit code: %v", got)
			}
		})
	}
}
