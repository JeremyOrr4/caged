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

func TestRunnerRun(t *testing.T) {
	t.Parallel()

	const version = "test"
	loadErr := errors.New("~/.config/caged/image.json: unknown field \"npm\"; only \"apt\" is supported")
	planErr := errors.New("plan failed")
	daemonErr := &engine.Error{Kind: engine.KindDaemonDown, Message: "daemon unavailable"}
	buildErr := errors.New("build failed")
	inspectErr := errors.New("inspect failed")
	runErr := errors.New("exit status 3")
	spec := plan.RunSpec{Image: images.BaseRef, Command: []string{"codex", "exec", "fix $(this) verbatim"}}
	ext := testExtension()
	const missingNotice = "Runtime image caged-runtime:latest not found; building it first...\n"

	tests := []struct {
		name       string
		exts       fakeExts
		store      fakeStore
		planErr    error
		daemonErr  error
		runErr     error
		verbose    bool
		wantErr    error
		wantCalls  []string
		wantStderr string
		wantImage  string // the image the planner is told to use; "" means it is not reached
	}{
		{
			name:      "no extension, image present: load, plan, daemon, inspect, run",
			store:     fakeStore{images: baseImage(currentImage(version))},
			wantCalls: []string{"load", "plan", "daemon", inspectBase, "run"},
			wantImage: images.BaseRef,
		},
		{
			name:       "no extension, image missing: build before running",
			wantCalls:  []string{"load", "plan", "daemon", inspectBase, buildBase, "run"},
			wantStderr: missingNotice,
			wantImage:  images.BaseRef,
		},
		{
			name:      "an invalid extension file fails before anything else",
			exts:      fakeExts{err: loadErr},
			wantErr:   loadErr,
			wantCalls: []string{"load"},
		},
		{
			name:      "plan failure stops before any engine call",
			planErr:   planErr,
			wantErr:   planErr,
			wantCalls: []string{"load", "plan"},
			wantImage: images.BaseRef,
		},
		{
			name:      "daemon failure stops before the image lookup",
			daemonErr: daemonErr,
			wantErr:   daemonErr,
			wantCalls: []string{"load", "plan", "daemon"},
			wantImage: images.BaseRef,
		},
		{
			name:      "other inspect failure is not treated as a missing image",
			store:     fakeStore{inspectErr: inspectErr},
			wantErr:   inspectErr,
			wantCalls: []string{"load", "plan", "daemon", inspectBase},
			wantImage: images.BaseRef,
		},
		{
			name:       "build failure stops before running",
			store:      fakeStore{buildErr: map[string]error{images.BaseRef: buildErr}},
			wantErr:    buildErr,
			wantCalls:  []string{"load", "plan", "daemon", inspectBase, buildBase},
			wantStderr: missingNotice,
			wantImage:  images.BaseRef,
		},
		{
			name:      "run error is returned unchanged",
			store:     fakeStore{images: baseImage(currentImage(version))},
			runErr:    runErr,
			wantErr:   runErr,
			wantCalls: []string{"load", "plan", "daemon", inspectBase, "run"},
			wantImage: images.BaseRef,
		},
		{
			name:       "an extension plans against the extension image and builds it",
			exts:       fakeExts{ext: ext},
			store:      fakeStore{images: baseImage(currentImage(version))},
			wantCalls:  []string{"load", "plan", "daemon", inspectBase, inspectExt, buildExt, "run"},
			wantStderr: "Building image extension from ~/.config/caged/image.json (apt: neovim, tmux)...\n",
			wantImage:  images.ExtensionRef,
		},
		{
			name:      "a current extension is not rebuilt",
			exts:      fakeExts{ext: ext},
			store:     fakeStore{images: map[string]engine.ImageInfo{images.BaseRef: currentImage(version), images.ExtensionRef: currentExtensionImage(*ext, "sha256:current", version)}},
			wantCalls: []string{"load", "plan", "daemon", inspectBase, inspectExt, "run"},
			wantImage: images.ExtensionRef,
		},
		{
			name:       "verbose says when there is no extension",
			exts:       fakeExts{source: "~/.config/caged/image.json"},
			store:      fakeStore{images: baseImage(currentImage(version))},
			verbose:    true,
			wantCalls:  []string{"load", "plan", "daemon", inspectBase, "run"},
			wantStderr: "image: no image.apt packages in ~/.config/caged/image.json\nimage: " + images.Base(version).Describe() + "\n",
			wantImage:  images.BaseRef,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls []string
			var stderr strings.Builder
			store, exts := tt.store, tt.exts
			store.calls, exts.calls = &calls, &calls
			runner := &fakeRunner{calls: &calls, runErr: tt.runErr}
			planner := &fakePlanner{calls: &calls, spec: spec, err: tt.planErr}
			daemon := &fakeDaemon{calls: &calls, err: tt.daemonErr}
			request := Request{NoMount: true, NoConfig: true, Verbose: tt.verbose}

			err := NewRunner(&exts, planner, daemon, NewImageEnsurer(&store, version), runner, &stderr).Run(context.Background(), request)
			if err != tt.wantErr {
				t.Fatalf("Run() error = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(calls, tt.wantCalls) {
				t.Fatalf("calls = %q, want %q", calls, tt.wantCalls)
			}
			if stderr.String() != tt.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tt.wantStderr)
			}
			if tt.wantErr == nil && !reflect.DeepEqual(runner.ran, &spec) {
				t.Fatalf("ran %+v, want the planned spec", runner.ran)
			}
			if planner.gotImage != tt.wantImage {
				t.Fatalf("planner image = %q, want %q", planner.gotImage, tt.wantImage)
			}
		})
	}
}

func TestRunnerPassesRequestAndVerboseReporterToPlanner(t *testing.T) {
	t.Parallel()

	var calls []string
	var stderr strings.Builder
	planner := &fakePlanner{calls: &calls}
	prompt := "hello"
	request := Request{Prompt: &prompt, Verbose: true, WorkDir: "/work"}
	store := &fakeStore{calls: &calls, images: baseImage(currentImage("test"))}
	runner := NewRunner(&fakeExts{calls: &calls}, planner, &fakeDaemon{calls: &calls}, NewImageEnsurer(store, "test"), &fakeRunner{calls: &calls}, &stderr)
	if err := runner.Run(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(planner.gotReq, request) {
		t.Fatalf("planner request = %+v, want %+v", planner.gotReq, request)
	}
	stderr.Reset()
	planner.gotRep.Detail("detail line")
	if stderr.String() != "detail line\n" {
		t.Fatalf("stderr = %q; the planner's reporter must be verbose and write to stderr", stderr.String())
	}
}
