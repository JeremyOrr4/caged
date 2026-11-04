// Package images describes the caged images: their tags, labels, embedded
// build files, and when an installed image is out of date. It does not run
// Docker itself.
package images

// Repository is the image repository of every image caged builds, the one
// caged clean searches for images older caged versions left behind.
const Repository = "caged-runtime"

// The tags of the images caged builds.
const (
	BaseRef      = Repository + ":latest"
	ExtensionRef = Repository + ":ext"
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
	labelPrefix        = "io.github.jeremyorr4.caged."
	LabelRole          = labelPrefix + "role"
	LabelRuntimeHash   = labelPrefix + "runtime-hash"
	LabelExtensionHash = labelPrefix + "extension-hash"
	LabelBaseID        = labelPrefix + "base-id"
	LabelVersion       = labelPrefix + "version"
)

// VersionsPath is where the image build records the installed CLI versions.
const VersionsPath = "/opt/caged/versions"
