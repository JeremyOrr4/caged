package app

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/images"
	"github.com/JeremyOrr4/caged/internal/plan"
)

const (
	inspectBase = "inspect caged-runtime:latest"
	inspectExt  = "inspect caged-runtime:ext"
	buildBase   = "build caged-runtime:latest"
	buildExt    = "build caged-runtime:ext"
)

func TestEnsurerEnsureBase(t *testing.T) {
	t.Parallel()

	const version = "0.2.0"
	boom := errors.New("boom")
	inUse := &engine.Error{Kind: engine.KindImageInUse, Message: "conflict: image is being used by running container abc"}
	staleLabels := map[string]string{images.LabelRole: "runtime", images.LabelRuntimeHash: "old", images.LabelVersion: "0.1.0"}
	sameVersionStale := map[string]string{images.LabelRole: "runtime", images.LabelRuntimeHash: "old", images.LabelVersion: version}
	const unlabeled = "Runtime image caged-runtime:latest was built by an older caged; rebuilding it once (this can take a few minutes)...\n"

	tests := []struct {
		name       string
		store      fakeStore
		verbose    bool
		wantCalls  []string
		wantErr    error
		wantNotice string
		wantDetail string
	}{
		{
			name:       "current image is not rebuilt",
			store:      fakeStore{images: baseImage(currentImage(version))},
			wantCalls:  []string{inspectBase},
			wantDetail: "image: " + images.Base(version).Describe() + "\n",
			verbose:    true,
		},
		{
			name:      "current image says nothing without --verbose",
			store:     fakeStore{images: baseImage(currentImage(version))},
			wantCalls: []string{inspectBase},
		},
		{
			name:       "missing image is built, and nothing is removed",
			wantCalls:  []string{inspectBase, buildBase},
			wantNotice: "Runtime image caged-runtime:latest not found; building it first...\n",
		},
		{
			name:       "unlabeled image is rebuilt and the old one removed",
			store:      fakeStore{images: baseImage(engine.ImageInfo{ID: "sha256:old"})},
			wantCalls:  []string{inspectBase, buildBase, "remove sha256:old"},
			wantNotice: unlabeled,
		},
		{
			name:       "image from another caged version names both versions",
			store:      fakeStore{images: baseImage(engine.ImageInfo{ID: "sha256:old", Labels: staleLabels})},
			wantCalls:  []string{inspectBase, buildBase, "remove sha256:old"},
			wantNotice: "Runtime image caged-runtime:latest was built by caged 0.1.0; rebuilding it for caged 0.2.0 (this can take a few minutes)...\n",
		},
		{
			name:       "changed files with the same version",
			store:      fakeStore{images: baseImage(engine.ImageInfo{ID: "sha256:old", Labels: sameVersionStale})},
			wantCalls:  []string{inspectBase, buildBase, "remove sha256:old"},
			wantNotice: "Runtime image caged-runtime:latest is out of date (its runtime files changed); rebuilding...\n",
		},
		{
			name:       "an unchanged image ID is not removed",
			store:      fakeStore{images: baseImage(engine.ImageInfo{ID: "sha256:same"}), newIDs: map[string]string{images.BaseRef: "sha256:same"}},
			wantCalls:  []string{inspectBase, buildBase},
			wantNotice: unlabeled,
		},
		{
			name:       "a build failure keeps the stale image and removes nothing",
			store:      fakeStore{images: baseImage(engine.ImageInfo{ID: "sha256:old"}), buildErr: map[string]error{images.BaseRef: boom}},
			wantErr:    boom,
			wantCalls:  []string{inspectBase, buildBase},
			wantNotice: unlabeled,
		},
		{
			name:      "an unexpected inspect error is returned, with no build",
			store:     fakeStore{inspectErr: boom},
			wantErr:   boom,
			wantCalls: []string{inspectBase},
		},
		{
			name:       "an image still in use is kept, and the run goes on",
			store:      fakeStore{images: baseImage(engine.ImageInfo{ID: "sha256:0123456789abcdef0123"}), removeErr: inUse},
			verbose:    true,
			wantCalls:  []string{inspectBase, buildBase, "remove sha256:0123456789abcdef0123"},
			wantNotice: unlabeled,
			wantDetail: "image: kept old image 0123456789ab (conflict: image is being used by running container abc)\n",
		},
		{
			name:       "an old image that is already gone is not reported",
			store:      fakeStore{images: baseImage(engine.ImageInfo{ID: "sha256:old"}), removeErr: &engine.Error{Kind: engine.KindImageNotFound, Message: "No such image"}},
			verbose:    true,
			wantCalls:  []string{inspectBase, buildBase, "remove sha256:old"},
			wantNotice: unlabeled,
		},
		{
			name:       "any other removal failure is only reported",
			store:      fakeStore{images: baseImage(engine.ImageInfo{ID: "sha256:old"}), removeErr: boom},
			verbose:    true,
			wantCalls:  []string{inspectBase, buildBase, "remove sha256:old"},
			wantNotice: unlabeled,
			wantDetail: "image: kept old image old (boom)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls []string
			var out strings.Builder
			store := tt.store
			store.calls = &calls
			err := NewImageEnsurer(&store, version).ensureImages(context.Background(), nil, NewReporter(&out, tt.verbose))
			if err != tt.wantErr { //nolint:errorlint // asserts the error is returned unchanged, not wrapped
				t.Fatalf("ensureImages() error = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(calls, tt.wantCalls) {
				t.Fatalf("calls = %q, want %q", calls, tt.wantCalls)
			}
			if want := tt.wantNotice + tt.wantDetail; out.String() != want {
				t.Fatalf("output = %q, want %q", out.String(), want)
			}
			if len(store.builds) == 1 {
				if want := images.Base(version).Spec(plan.BuildOptions{}); !reflect.DeepEqual(store.builds[0], want) {
					t.Fatalf("build spec = %+v, want %+v", store.builds[0], want)
				}
			}
		})
	}
}

func TestEnsurerEnsureExtension(t *testing.T) {
	t.Parallel()

	const version = "0.2.0"
	const baseID = "sha256:current"
	ext := testExtension()
	boom := errors.New("exit status 100")
	buildNotice := "Building image extension from ~/.config/caged/config.yaml (apt: neovim, tmux)...\n"
	staleExt := engine.ImageInfo{ID: "sha256:old-ext", Labels: map[string]string{
		images.LabelRole: "extension", images.LabelExtensionHash: "old", images.LabelBaseID: baseID, images.LabelVersion: version,
	}}
	currentExt := currentExtensionImage(*ext, baseID, version)
	both := func(base, ext engine.ImageInfo) map[string]engine.ImageInfo {
		return map[string]engine.ImageInfo{images.BaseRef: base, images.ExtensionRef: ext}
	}

	tests := []struct {
		name       string
		store      fakeStore
		verbose    bool
		wantCalls  []string
		wantErr    string
		wantOut    string
		wantBaseID string // the base-id label of the extension build, if one happens
	}{
		{
			name:       "first build of the extension on a current base",
			store:      fakeStore{images: baseImage(currentImage(version))},
			wantCalls:  []string{inspectBase, inspectExt, buildExt},
			wantOut:    buildNotice,
			wantBaseID: baseID,
		},
		{
			name:      "current base and current extension: nothing is built",
			store:     fakeStore{images: both(currentImage(version), currentExt)},
			verbose:   true,
			wantCalls: []string{inspectBase, inspectExt},
			wantOut:   "image: " + images.Base(version).Describe() + "\nimage: extension from ~/.config/caged/config.yaml (apt: neovim, tmux) -> caged-runtime:ext\n",
		},
		{
			name:       "current base, stale extension: only the extension is rebuilt and the old one removed",
			store:      fakeStore{images: both(currentImage(version), staleExt)},
			wantCalls:  []string{inspectBase, inspectExt, buildExt, "remove sha256:old-ext"},
			wantOut:    buildNotice,
			wantBaseID: baseID,
		},
		{
			name: "rebuilt base: the extension is rebuilt on the new base, extension image removed first",
			store: fakeStore{
				images: both(engine.ImageInfo{ID: "sha256:old-base"}, currentExtensionImage(*ext, "sha256:old-base", version)),
			},
			wantCalls: []string{inspectBase, buildBase, inspectExt, buildExt, "remove sha256:current-ext", "remove sha256:old-base"},
			wantOut: "Runtime image caged-runtime:latest was built by an older caged; rebuilding it once (this can take a few minutes)...\n" +
				buildNotice,
			wantBaseID: "sha256:new-caged-runtime:latest",
		},
		{
			name:      "an extension build failure is explained, keeps the old images, and removes nothing",
			store:     fakeStore{images: both(currentImage(version), staleExt), buildErr: map[string]error{images.ExtensionRef: boom}},
			wantCalls: []string{inspectBase, inspectExt, buildExt},
			wantOut:   buildNotice,
			wantErr:   "building the image extension from ~/.config/caged/config.yaml failed; fix or remove that file: exit status 100",
		},
		{
			name:      "a base build failure stops before the extension",
			store:     fakeStore{buildErr: map[string]error{images.BaseRef: boom}},
			wantCalls: []string{inspectBase, buildBase},
			wantOut:   "Runtime image caged-runtime:latest not found; building it first...\n",
			wantErr:   "exit status 100",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls []string
			var out strings.Builder
			store := tt.store
			store.calls = &calls
			err := NewImageEnsurer(&store, version).ensureImages(context.Background(), ext, NewReporter(&out, tt.verbose))
			if (err == nil) != (tt.wantErr == "") || (err != nil && err.Error() != tt.wantErr) {
				t.Fatalf("ensureImages() error = %v, want %q", err, tt.wantErr)
			}
			if !reflect.DeepEqual(calls, tt.wantCalls) {
				t.Fatalf("calls = %q, want %q", calls, tt.wantCalls)
			}
			if out.String() != tt.wantOut {
				t.Fatalf("output = %q, want %q", out.String(), tt.wantOut)
			}
			if tt.wantBaseID != "" {
				want := images.Extension(*ext, tt.wantBaseID, version).Spec(plan.BuildOptions{})
				if got := store.builds[len(store.builds)-1]; !reflect.DeepEqual(got, want) {
					t.Fatalf("extension build spec = %+v, want %+v", got, want)
				}
			}
		})
	}
}

func TestEnsurerBuildAll(t *testing.T) {
	t.Parallel()

	const version = "0.2.0"
	ext := testExtension()
	boom := errors.New("boom")
	runtimeImage := BuiltImage{Role: images.RoleRuntime, Ref: images.BaseRef}
	extImage := BuiltImage{Role: images.RoleExtension, Ref: images.ExtensionRef, Summary: "apt: neovim, tmux"}
	tests := []struct {
		name      string
		ext       *plan.Extension
		opts      BuildOptions
		store     fakeStore
		wantCalls []string
		wantErr   bool
		want      []BuiltImage
	}{
		{
			name:      "builds even though the image is current, with no notice",
			store:     fakeStore{images: baseImage(currentImage(version))},
			wantCalls: []string{inspectBase, buildBase, "remove sha256:current"},
			want:      []BuiltImage{runtimeImage},
		},
		{
			name:      "no cache is passed to the build",
			opts:      BuildOptions{NoCache: true},
			wantCalls: []string{inspectBase, buildBase},
			want:      []BuiltImage{runtimeImage},
		},
		{
			name:      "build failure returns no images and removes nothing",
			store:     fakeStore{images: baseImage(currentImage(version)), buildErr: map[string]error{images.BaseRef: boom}},
			wantCalls: []string{inspectBase, buildBase},
			wantErr:   true,
		},
		{
			name:      "base and extension are both built, with the same options",
			ext:       ext,
			opts:      BuildOptions{NoCache: true},
			store:     fakeStore{images: map[string]engine.ImageInfo{images.BaseRef: currentImage(version), images.ExtensionRef: currentExtensionImage(*ext, "sha256:current", version)}},
			wantCalls: []string{inspectBase, buildBase, inspectExt, buildExt, "remove sha256:current-ext", "remove sha256:current"},
			want:      []BuiltImage{runtimeImage, extImage},
		},
		{
			name:      "an extension build failure returns no images",
			ext:       ext,
			store:     fakeStore{buildErr: map[string]error{images.ExtensionRef: boom}},
			wantCalls: []string{inspectBase, buildBase, inspectExt, buildExt},
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls []string
			var out strings.Builder
			store := tt.store
			store.calls = &calls
			got, err := NewImageEnsurer(&store, version).rebuildImages(context.Background(), tt.ext, tt.opts, NewReporter(&out, false))
			if (err != nil) != tt.wantErr {
				t.Fatalf("rebuildImages() error = %v, want error: %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(calls, tt.wantCalls) {
				t.Fatalf("calls = %q, want %q", calls, tt.wantCalls)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("rebuildImages() = %+v, want %+v", got, tt.want)
			}
			if out.String() != "" {
				t.Fatalf("output = %q, want nothing: a requested build prints no notice, and details need --verbose", out.String())
			}
			for _, spec := range store.builds {
				if spec.NoCache != tt.opts.NoCache {
					t.Fatalf("%s: NoCache = %v, want %v", spec.Image, spec.NoCache, tt.opts.NoCache)
				}
			}
		})
	}
}
