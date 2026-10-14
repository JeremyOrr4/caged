// Package images describes the caged images: their tags, labels, embedded
// build files, and when an installed image is out of date. It does not run
// Docker itself.
package images

// The tags of the images caged builds.
const (
	BaseRef      = "caged-runtime:latest"
	ExtensionRef = "caged-runtime:ext"
)

// Role says what an image is for.
type Role string

// The roles of the images caged builds.
const (
	RoleRuntime   Role = "runtime"
	RoleExtension Role = "extension"
)

// The labels caged writes on its images. They use a reverse-DNS prefix, as the
// OCI and Docker conventions recommend for distributed software.
const (
	LabelPrefix        = "io.github.jeremyorr4.caged."
	LabelRole          = LabelPrefix + "role"
	LabelRuntimeHash   = LabelPrefix + "runtime-hash"
	LabelExtensionHash = LabelPrefix + "extension-hash"
	LabelBaseID        = LabelPrefix + "base-id"
	LabelVersion       = LabelPrefix + "version"
)

// VersionsPath is where the image build records the installed CLI versions.
const VersionsPath = "/opt/caged/versions"
