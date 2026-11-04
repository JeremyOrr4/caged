package docker

import (
	"errors"
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/system"
)

func TestPlatformRules(t *testing.T) {
	t.Parallel()

	if got := userArgs(linux); !reflect.DeepEqual(got, []string{"--user", "1000:1001"}) {
		t.Errorf("userArgs(linux) = %q", got)
	}
	if got := userArgs(darwin); got != nil {
		t.Errorf("userArgs(darwin) = %q, want none: Docker Desktop maps ownership", got)
	}
	if got := daemonLauncher(darwin); got != "start Docker Desktop" {
		t.Errorf("daemonLauncher(darwin) = %q", got)
	}
	if got := daemonLauncher(linux); got != "start the Docker daemon" {
		t.Errorf("daemonLauncher(linux) = %q", got)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		if err := checkArch(system.Platform{Arch: arch}); err != nil {
			t.Errorf("checkArch(%s) = %v", arch, err)
		}
	}
	for _, message := range []string{"Mounts denied", "path is NOT SHARED from the host", "mount denied"} {
		if !looksLikeSharedPathError(message) {
			t.Errorf("looksLikeSharedPathError(%q) = false", message)
		}
	}
	plain := errors.New("exit status 1")
	if got := sharedPathHint(darwin, "some other failure", plain); got != nil {
		t.Errorf("sharedPathHint() = %v, want no hint", got)
	}
	if got := sharedPathHint(linux, "Mounts denied", plain); got != nil {
		t.Errorf("sharedPathHint(linux) = %v, want no hint: only Docker Desktop has File Sharing", got)
	}
}
