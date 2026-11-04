package app

import (
	"context"
	"fmt"

	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/images"
)

// CleanOptions are the settings of one clean.
type CleanOptions struct{ DryRun bool }

// CleanOutcome says what happened to one image.
type CleanOutcome int

// The outcomes of cleaning one image.
const (
	Removed CleanOutcome = iota
	WouldRemove
	Kept
)

// CleanResult is one image caged found, and what happened to it.
type CleanResult struct {
	Ref     string // repository:tag, or "<untagged> sha256:<12>"
	Size    string
	Outcome CleanOutcome
	Reason  string // why a Kept image was kept
}

// keptReason is what `caged clean` says about an image that docker refused to
// remove.
const keptReason = "in use by a container (remove it with docker rm first)"

// Cleaner is the clean use case.
type Cleaner struct {
	daemon Daemon
	lister ImageLister
	store  ImageStore
}

// NewCleaner returns a Cleaner that finds images with lister and removes them
// through store.
func NewCleaner(daemon Daemon, lister ImageLister, store ImageStore) *Cleaner {
	return &Cleaner{daemon: daemon, lister: lister, store: store}
}

// Clean removes caged's images, or with DryRun reports what it would remove.
// It finds extension images, then runtime images, then unlabeled images from
// older caged versions, and removes them in that order, because a base image
// cannot be removed while an extension built on it exists. An image that has
// several tags is removed tag by tag. It never forces a
// removal: an image that a container still uses is kept and reported. It does
// not touch containers, volumes, the build cache, or any config.
func (c *Cleaner) Clean(ctx context.Context, opts CleanOptions) ([]CleanResult, error) {
	if _, err := c.daemon.Daemon(ctx); err != nil {
		return nil, err
	}
	queries := []engine.ImageQuery{
		{Label: images.LabelRole + "=" + string(images.RoleExtension)},
		{Label: images.LabelRole + "=" + string(images.RoleRuntime)},
		{Reference: images.Repository},
	}
	// A tagged image is removed by its tag and an untagged one by its ID. An
	// image with several tags is removed one tag at a time, which Docker allows;
	// removing it by ID would be refused as "referenced in multiple repositories".
	var found []engine.ImageSummary
	seen := map[string]bool{}
	for _, query := range queries {
		listed, err := c.lister.ListImages(ctx, query)
		if err != nil {
			return nil, err
		}
		for _, image := range listed {
			if target := removalTarget(image); !seen[target] {
				seen[target] = true
				found = append(found, image)
			}
		}
	}
	var results []CleanResult
	for _, image := range found {
		result := CleanResult{Ref: describeImage(image), Size: image.Size, Outcome: WouldRemove}
		if !opts.DryRun {
			result.Outcome = Removed
			if err := c.store.RemoveImage(ctx, removalTarget(image)); err != nil && !engine.IsKind(err, engine.KindImageNotFound) {
				if !engine.IsKind(err, engine.KindImageInUse) {
					return results, fmt.Errorf("remove image %s: %w", result.Ref, err)
				}
				result.Outcome, result.Reason = Kept, keptReason
			}
		}
		results = append(results, result)
	}
	return results, nil
}

// removalTarget is what `docker image rm` is given for an image: its
// repository:tag, or its ID when it has no tag.
func removalTarget(image engine.ImageSummary) string {
	if image.Repository == "" || image.Repository == "<none>" || image.Tag == "" || image.Tag == "<none>" {
		return image.ID
	}
	return image.Repository + ":" + image.Tag
}

// describeImage names an image by repository and tag, or, when it has none, by
// its short ID.
func describeImage(image engine.ImageSummary) string {
	if image.Repository == "" || image.Repository == "<none>" {
		return "<untagged> sha256:" + shortID(image.ID)
	}
	if image.Tag == "" || image.Tag == "<none>" {
		return image.Repository
	}
	return image.Repository + ":" + image.Tag
}
