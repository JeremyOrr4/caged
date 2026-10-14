package images

import (
	_ "embed"

	"github.com/JeremyOrr4/caged/internal/plan"
)

//go:embed Dockerfile
var runtimeDockerfile string

//go:embed caged-entrypoint.sh
var runtimeEntrypoint string

// baseFiles returns the build context of the runtime image: the embedded
// Dockerfile and entrypoint. Every call returns fresh data.
func baseFiles() []plan.File {
	return []plan.File{
		{Name: "Dockerfile", Label: "Dockerfile", Data: []byte(runtimeDockerfile), Mode: 0o644},
		{Name: "caged-entrypoint.sh", Label: "entrypoint", Data: []byte(runtimeEntrypoint), Mode: 0o755},
	}
}
