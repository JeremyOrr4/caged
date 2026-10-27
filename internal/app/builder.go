package app

import (
	"context"
	"io"

	"github.com/JeremyOrr4/caged/internal/images"
)

// BuildOptions are the settings of one build.
type BuildOptions struct {
	NoCache     bool
	TrustConfig bool // approve a changed config.yaml without asking
}

// BuiltImage describes one image a build produced.
type BuiltImage struct {
	Role    images.Role
	Ref     string
	Summary string // "" or a short description of what was added
}

// Builder is the build use case.
type Builder struct {
	exts    Extensions
	daemon  Daemon
	ensurer *ImageEnsurer
	stderr  io.Writer
}

// NewBuilder returns a Builder that reads the image extension from exts,
// checks the engine with daemon, builds through ensurer, and reports progress
// on stderr.
func NewBuilder(exts Extensions, daemon Daemon, ensurer *ImageEnsurer, stderr io.Writer) *Builder {
	return &Builder{exts: exts, daemon: daemon, ensurer: ensurer, stderr: stderr}
}

// Build reads the image extension, checks the engine, rebuilds the runtime
// image and the extension image if there is one, and returns what it built.
func (b *Builder) Build(ctx context.Context, opts BuildOptions) ([]BuiltImage, error) {
	ext, err := b.exts.Load()
	if err != nil {
		return nil, err
	}
	if _, err := b.daemon.Daemon(ctx); err != nil {
		return nil, err
	}
	return b.ensurer.rebuildImages(ctx, ext, opts, NewReporter(b.stderr, false))
}
