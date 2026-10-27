package app

import (
	"context"
	"errors"
	"strings"

	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/images"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// ImageEnsurer keeps caged's images current: it builds a missing or stale
// image, and removes the image the new one replaced.
type ImageEnsurer struct {
	store   ImageStore
	version string
}

// NewImageEnsurer returns an ImageEnsurer that works on store and labels the
// images it builds with the caged version.
func NewImageEnsurer(store ImageStore, version string) *ImageEnsurer {
	return &ImageEnsurer{store: store, version: version}
}

// ensureImages builds the runtime image, and the extension image on top of it when
// ext is not nil, whenever either is missing or out of date. A failed build is
// returned; a stale image is never used instead. The image to run is
// images.RefFor(ext).
func (e *ImageEnsurer) ensureImages(ctx context.Context, ext *plan.Extension, rep *Reporter) error {
	return e.refresh(ctx, ext, false, plan.BuildOptions{}, rep)
}

// rebuildImages rebuilds the runtime image, and the extension image when ext is not
// nil, whatever their state, and returns what it built.
func (e *ImageEnsurer) rebuildImages(ctx context.Context, ext *plan.Extension, opts BuildOptions, rep *Reporter) ([]BuiltImage, error) {
	if err := e.refresh(ctx, ext, true, plan.BuildOptions{NoCache: opts.NoCache}, rep); err != nil {
		return nil, err
	}
	built := []BuiltImage{{Role: images.RoleRuntime, Ref: images.BaseRef}}
	if ext != nil {
		built = append(built, BuiltImage{Role: images.RoleExtension, Ref: images.ExtensionRef, Summary: "apt: " + strings.Join(ext.Apt, ", ")})
	}
	return built, nil
}

// refresh brings the runtime image, and the extension image when ext is not
// nil, up to date (or rebuilds them when force is set), then removes the
// images the builds replaced.
func (e *ImageEnsurer) refresh(ctx context.Context, ext *plan.Extension, force bool, opts plan.BuildOptions, rep *Reporter) error {
	baseID, baseReplaced, err := e.ensureTarget(ctx, images.Base(e.version), force, opts, rep)
	if err != nil {
		return err
	}
	if ext == nil {
		e.cleanup(ctx, rep, baseReplaced)
		return nil
	}
	_, extReplaced, err := e.ensureTarget(ctx, images.Extension(*ext, baseID, e.version), force, opts, rep)
	if err != nil {
		return err
	}
	// The extension goes first: a base image cannot be removed while the old
	// extension image still builds on it.
	e.cleanup(ctx, rep, extReplaced, baseReplaced)
	return nil
}

// ensureTarget looks at the image with the target's tag and builds it unless it is
// current (or force is set). It returns the ID of the image to use, and the ID
// of the image the build replaced, or "" when nothing was replaced.
func (e *ImageEnsurer) ensureTarget(ctx context.Context, t images.Target, force bool, opts plan.BuildOptions, rep *Reporter) (id, replaced string, err error) {
	info, err := e.store.InspectImage(ctx, t.Ref)
	found := err == nil
	if err != nil && !engine.IsKind(err, engine.KindImageNotFound) {
		return "", "", err
	}
	freshness := t.Assess(info.Labels, found)
	if !force && freshness.State == images.StateCurrent {
		rep.detail("image: %s", t.Describe())
		return info.ID, "", nil
	}
	if !force {
		rep.Notice("%s", t.Notice(freshness))
	}
	newID, err := e.store.BuildImage(ctx, t.Spec(opts))
	if err != nil {
		return "", "", t.WrapBuildError(err)
	}
	if found && info.ID != newID {
		replaced = info.ID
	}
	return newID, replaced, nil
}

// cleanup removes the images that were replaced, in the order given. It is
// best effort and never fails the run: an image that a container or another
// tag still uses is kept, and every failure is only reported with --verbose.
func (e *ImageEnsurer) cleanup(ctx context.Context, rep *Reporter, replaced ...string) {
	for _, id := range replaced {
		if id == "" {
			continue
		}
		err := e.store.RemoveImage(ctx, id)
		if err == nil || engine.IsKind(err, engine.KindImageNotFound) {
			continue // removed, or already gone (the engine can drop a replaced image itself)
		}
		reason := err.Error()
		var engineErr *engine.Error
		if errors.As(err, &engineErr) && engineErr.Kind == engine.KindImageInUse {
			reason = engineErr.Message
		}
		rep.detail("image: kept old image %s (%s)", shortID(id), reason)
	}
}

// shortID shortens an image ID to the 12 characters docker shows, without the
// "sha256:" prefix.
func shortID(id string) string {
	const prefix = "sha256:"
	if len(id) > len(prefix) && id[:len(prefix)] == prefix {
		id = id[len(prefix):]
	}
	if len(id) > 12 {
		id = id[:12]
	}
	return id
}
