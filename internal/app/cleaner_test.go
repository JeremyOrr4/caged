package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/images"
)

func TestCleanerClean(t *testing.T) {
	t.Parallel()

	const (
		listExt     = "list io.github.jeremyorr4.caged.role=extension"
		listRuntime = "list io.github.jeremyorr4.caged.role=runtime"
		listOlder   = "list caged-runtime"
	)
	ext := engine.ImageSummary{ID: "sha256:ext", Repository: "caged-runtime", Tag: "ext", Size: "412MB"}
	runtime := engine.ImageSummary{ID: "sha256:runtime", Repository: "caged-runtime", Tag: "latest", Size: "3.1GB"}
	dangling := engine.ImageSummary{ID: "sha256:1a2b3c4d5e6f7890", Repository: "<none>", Tag: "<none>", Size: "3.0GB"}
	older := engine.ImageSummary{ID: "sha256:older", Repository: "caged-runtime", Tag: "old", Size: "2.9GB"}
	inUse := &engine.Error{Kind: engine.KindImageInUse, Message: "conflict: image is being used by running container x"}
	boom := errors.New("boom")
	down := &engine.Error{Kind: engine.KindDaemonDown, Message: "daemon unavailable"}

	tests := []struct {
		name      string
		by        map[string][]engine.ImageSummary
		dryRun    bool
		removeErr error
		daemonErr error
		listErr   error
		want      []CleanResult
		wantErr   error
		wantCalls []string
	}{
		{
			name: "extension, then runtime, then older unlabeled images",
			by: map[string][]engine.ImageSummary{
				"io.github.jeremyorr4.caged.role=extension": {ext},
				"io.github.jeremyorr4.caged.role=runtime":   {runtime, dangling},
				"caged-runtime": {runtime, ext, older},
			},
			want: []CleanResult{
				{Ref: "caged-runtime:ext", ID: "sha256:ext", Size: "412MB", Outcome: Removed},
				{Ref: "caged-runtime:latest", ID: "sha256:runtime", Size: "3.1GB", Outcome: Removed},
				{Ref: "<untagged> sha256:1a2b3c4d5e6f", ID: "sha256:1a2b3c4d5e6f7890", Size: "3.0GB", Outcome: Removed},
				{Ref: "caged-runtime:old", ID: "sha256:older", Size: "2.9GB", Outcome: Removed},
			},
			wantCalls: []string{"daemon", listExt, listRuntime, listOlder, "remove caged-runtime:ext", "remove caged-runtime:latest", "remove sha256:1a2b3c4d5e6f7890", "remove caged-runtime:old"},
		},
		{
			name:      "an image found by two queries is removed once",
			by:        map[string][]engine.ImageSummary{"io.github.jeremyorr4.caged.role=runtime": {runtime}, "caged-runtime": {runtime}},
			want:      []CleanResult{{Ref: "caged-runtime:latest", ID: "sha256:runtime", Size: "3.1GB", Outcome: Removed}},
			wantCalls: []string{"daemon", listExt, listRuntime, listOlder, "remove caged-runtime:latest"},
		},
		{
			name: "an image with several tags is removed tag by tag",
			by: map[string][]engine.ImageSummary{
				"io.github.jeremyorr4.caged.role=runtime": {runtime},
				"caged-runtime": {runtime, {ID: "sha256:runtime", Repository: "caged-runtime", Tag: "keep", Size: "3.1GB"}},
			},
			want: []CleanResult{
				{Ref: "caged-runtime:latest", ID: "sha256:runtime", Size: "3.1GB", Outcome: Removed},
				{Ref: "caged-runtime:keep", ID: "sha256:runtime", Size: "3.1GB", Outcome: Removed},
			},
			wantCalls: []string{"daemon", listExt, listRuntime, listOlder, "remove caged-runtime:latest", "remove caged-runtime:keep"},
		},
		{
			name:      "nothing to clean",
			want:      nil,
			wantCalls: []string{"daemon", listExt, listRuntime, listOlder},
		},
		{
			name:      "dry run removes nothing",
			by:        map[string][]engine.ImageSummary{"io.github.jeremyorr4.caged.role=extension": {ext}, "caged-runtime": {older}},
			dryRun:    true,
			want:      []CleanResult{{Ref: "caged-runtime:ext", ID: "sha256:ext", Size: "412MB", Outcome: WouldRemove}, {Ref: "caged-runtime:old", ID: "sha256:older", Size: "2.9GB", Outcome: WouldRemove}},
			wantCalls: []string{"daemon", listExt, listRuntime, listOlder},
		},
		{
			name:      "an image in use is kept and the rest are still removed",
			by:        map[string][]engine.ImageSummary{"io.github.jeremyorr4.caged.role=extension": {ext}, "io.github.jeremyorr4.caged.role=runtime": {runtime}},
			removeErr: inUse,
			want: []CleanResult{
				{Ref: "caged-runtime:ext", ID: "sha256:ext", Size: "412MB", Outcome: Kept, Reason: "in use by a container (remove it with docker rm first)"},
				{Ref: "caged-runtime:latest", ID: "sha256:runtime", Size: "3.1GB", Outcome: Kept, Reason: "in use by a container (remove it with docker rm first)"},
			},
			wantCalls: []string{"daemon", listExt, listRuntime, listOlder, "remove caged-runtime:ext", "remove caged-runtime:latest"},
		},
		{
			name:      "an image that vanished before its removal counts as removed",
			by:        map[string][]engine.ImageSummary{"io.github.jeremyorr4.caged.role=extension": {ext}},
			removeErr: &engine.Error{Kind: engine.KindImageNotFound, Message: "No such image"},
			want:      []CleanResult{{Ref: "caged-runtime:ext", ID: "sha256:ext", Size: "412MB", Outcome: Removed}},
			wantCalls: []string{"daemon", listExt, listRuntime, listOlder, "remove caged-runtime:ext"},
		},
		{
			name:      "another removal failure stops the clean and keeps what was done",
			by:        map[string][]engine.ImageSummary{"io.github.jeremyorr4.caged.role=extension": {ext}},
			removeErr: boom,
			wantErr:   boom,
			wantCalls: []string{"daemon", listExt, listRuntime, listOlder, "remove caged-runtime:ext"},
		},
		{
			name:      "daemon down: nothing is listed",
			daemonErr: down,
			wantErr:   down,
			wantCalls: []string{"daemon"},
		},
		{
			name:      "a listing failure is returned",
			listErr:   boom,
			wantErr:   boom,
			wantCalls: []string{"daemon", listExt},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls []string
			daemon := &fakeDaemon{calls: &calls, err: tt.daemonErr}
			lister := &fakeLister{calls: &calls, by: tt.by, err: tt.listErr}
			store := &fakeStore{calls: &calls, removeErr: tt.removeErr}
			got, err := NewCleaner(daemon, lister, store).Clean(context.Background(), CleanOptions{DryRun: tt.dryRun})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Clean() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if tt.wantErr == nil && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Clean() =\n%+v\nwant\n%+v", got, tt.want)
			}
			if !reflect.DeepEqual(calls, tt.wantCalls) {
				t.Fatalf("calls = %q, want %q", calls, tt.wantCalls)
			}
			if tt.dryRun && len(store.removed) != 0 {
				t.Fatalf("a dry run removed %v", store.removed)
			}
		})
	}
}

func TestCleanerQueriesNameTheLabelsCagedWrites(t *testing.T) {
	t.Parallel()

	if images.LabelRole != "io.github.jeremyorr4.caged.role" {
		t.Fatalf("LabelRole = %q: the cleaner test above hard-codes the key", images.LabelRole)
	}
}
