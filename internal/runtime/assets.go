package runtime

import (
	_ "embed"

	"github.com/JeremyOrr4/caged/internal/plan"
)

//go:embed Dockerfile
var runtimeDockerfile string

//go:embed caged-entrypoint.sh
var runtimeEntrypoint string

// BuildSpec describes how to build the runtime image from the embedded
// Dockerfile and entrypoint.
func BuildSpec(noCache bool) plan.BuildSpec {
	return plan.BuildSpec{
		Image:      Image,
		Dockerfile: "Dockerfile",
		NoCache:    noCache,
		Files: []plan.File{
			{Name: "Dockerfile", Label: "Dockerfile", Data: []byte(runtimeDockerfile), Mode: 0o644},
			{Name: "caged-entrypoint.sh", Label: "entrypoint", Data: []byte(runtimeEntrypoint), Mode: 0o755},
		},
	}
}
