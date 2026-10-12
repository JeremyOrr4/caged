package container

import "testing"

func TestEmbeddedDockerfileIsPresent(t *testing.T) {
	t.Parallel()

	if len(runtimeDockerfile) == 0 {
		t.Fatal("runtime Dockerfile is empty")
	}
	if len(runtimeDockerfile) < 200 {
		t.Fatalf("runtime Dockerfile unexpectedly short: %d bytes", len(runtimeDockerfile))
	}
}
