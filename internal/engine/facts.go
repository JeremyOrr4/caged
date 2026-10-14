package engine

// DaemonInfo describes the container engine that answered `docker info`.
type DaemonInfo struct {
	CLIPath         string // the resolved docker CLI
	ServerVersion   string
	OperatingSystem string
	Arch            string // normalized: amd64 or arm64
}

// ImageInfo describes one local image.
type ImageInfo struct {
	ID, OS, Arch string
	Labels       map[string]string
}

// ImageSummary is one row of an image listing.
type ImageSummary struct {
	ID, Repository, Tag, Size string
	Labels                    map[string]string
}

// ImageQuery selects images to list. Exactly one field is set.
type ImageQuery struct{ Label, Reference string }
