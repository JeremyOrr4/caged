package app

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/images"
	"github.com/JeremyOrr4/caged/internal/plan"
)

func TestBuilderBuild(t *testing.T) {
	t.Parallel()

	boom := errors.New("build failed")
	loadErr := errors.New("~/.config/caged/config.yaml: line 1: did not find expected node content")
	daemonErr := &engine.Error{Kind: engine.KindNotInstalled, Message: "Docker is not installed"}
	runtimeImage := BuiltImage{Role: images.RoleRuntime, Ref: images.BaseRef}
	extImage := BuiltImage{Role: images.RoleExtension, Ref: images.ExtensionRef, Summary: "apt: neovim, tmux"}
	tests := []struct {
		name      string
		exts      fakeExts
		opts      BuildOptions
		daemonErr error
		buildErr  map[string]error
		want      []BuiltImage
		wantErr   error
		wantCalls []string
	}{
		{name: "base only", want: []BuiltImage{runtimeImage}, wantCalls: []string{"load", "daemon", inspectBase, buildBase, "remove sha256:current"}},
		{name: "no cache", opts: BuildOptions{NoCache: true}, want: []BuiltImage{runtimeImage}, wantCalls: []string{"load", "daemon", inspectBase, buildBase, "remove sha256:current"}},
		{
			name: "base and extension", exts: fakeExts{ext: testExtension()}, want: []BuiltImage{runtimeImage, extImage},
			wantCalls: []string{"load", "daemon", inspectBase, buildBase, inspectExt, buildExt, "remove sha256:current"},
		},
		{name: "an invalid extension file builds nothing, not even the base", exts: fakeExts{err: loadErr}, wantErr: loadErr, wantCalls: []string{"load"}},
		{name: "daemon failure builds nothing", daemonErr: daemonErr, wantErr: daemonErr, wantCalls: []string{"load", "daemon"}},
		{name: "build failure returns no images", buildErr: map[string]error{images.BaseRef: boom}, wantErr: boom, wantCalls: []string{"load", "daemon", inspectBase, buildBase}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls []string
			store := &fakeStore{calls: &calls, images: baseImage(currentImage("test")), buildErr: tt.buildErr}
			exts := tt.exts
			exts.calls = &calls
			builder := NewBuilder(&exts, &fakeDaemon{calls: &calls, err: tt.daemonErr}, NewImageEnsurer(store, "test"), io.Discard)
			got, err := builder.Build(context.Background(), tt.opts)
			if err != tt.wantErr { //nolint:errorlint // asserts the error is returned unchanged, not wrapped
				t.Fatalf("Build() error = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Build() = %+v, want %+v", got, tt.want)
			}
			if !reflect.DeepEqual(calls, tt.wantCalls) {
				t.Fatalf("calls = %q, want %q", calls, tt.wantCalls)
			}
			if len(store.builds) > 0 && !reflect.DeepEqual(store.builds[0], images.Base("test").Spec(plan.BuildOptions{NoCache: tt.opts.NoCache})) {
				t.Fatalf("build spec = %+v, want the base target with NoCache %v", store.builds[0], tt.opts.NoCache)
			}
		})
	}
}
