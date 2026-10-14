package app

import (
	"context"

	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/images"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// The fakes below implement the interfaces in interfaces.go. Each one appends
// to a shared calls log, so tests can assert the order of calls across them.

// fakeDaemon is a Daemon with a fixed answer. A zero info means a Docker
// Desktop 27.0.0 engine on amd64.
type fakeDaemon struct {
	calls *[]string
	err   error
	info  engine.DaemonInfo
}

func (f *fakeDaemon) Daemon(context.Context) (engine.DaemonInfo, error) {
	*f.calls = append(*f.calls, "daemon")
	if f.err != nil {
		return engine.DaemonInfo{}, f.err
	}
	if f.info == (engine.DaemonInfo{}) {
		return engine.DaemonInfo{CLIPath: "/usr/local/bin/docker", ServerVersion: "27.0.0", OperatingSystem: "Docker Desktop", Arch: "amd64"}, nil
	}
	return f.info, nil
}

// fakeStore is an ImageStore. An image that is not in images is not found. A
// build produces the ID in newIDs for its tag, or "sha256:new-<tag>".
type fakeStore struct {
	calls      *[]string
	images     map[string]engine.ImageInfo // by tag
	inspectErr error                       // fails every InspectImage when set
	newIDs     map[string]string           // by tag
	builds     []plan.BuildSpec
	buildErr   map[string]error // by tag
	removeErr  error
	removed    []string
}

// baseImage is a store content with only the runtime image, as given.
func baseImage(info engine.ImageInfo) map[string]engine.ImageInfo {
	return map[string]engine.ImageInfo{images.BaseRef: info}
}

func (f *fakeStore) InspectImage(_ context.Context, ref string) (engine.ImageInfo, error) {
	*f.calls = append(*f.calls, "inspect "+ref)
	if f.inspectErr != nil {
		return engine.ImageInfo{}, f.inspectErr
	}
	info, ok := f.images[ref]
	if !ok {
		return engine.ImageInfo{}, &engine.Error{Kind: engine.KindImageNotFound, Message: "image " + ref + " not found"}
	}
	return info, nil
}

func (f *fakeStore) BuildImage(_ context.Context, spec plan.BuildSpec) (string, error) {
	*f.calls = append(*f.calls, "build "+spec.Image)
	f.builds = append(f.builds, spec)
	if err := f.buildErr[spec.Image]; err != nil {
		return "", err
	}
	if id := f.newIDs[spec.Image]; id != "" {
		return id, nil
	}
	return "sha256:new-" + spec.Image, nil
}

func (f *fakeStore) RemoveImage(_ context.Context, id string) error {
	*f.calls = append(*f.calls, "remove "+id)
	f.removed = append(f.removed, id)
	return f.removeErr
}

// fakeExts is an Extensions with a fixed answer. It logs "load".
type fakeExts struct {
	calls     *[]string
	ext       *plan.Extension
	err       error
	source    string
	sourceErr error
}

func (f *fakeExts) Load() (*plan.Extension, error) {
	*f.calls = append(*f.calls, "load")
	return f.ext, f.err
}

func (f *fakeExts) Source() (string, error) { return f.source, f.sourceErr }

// fakeRunner is a ContainerRunner that records the spec it was given.
type fakeRunner struct {
	calls  *[]string
	ran    *plan.RunSpec
	runErr error
}

func (f *fakeRunner) RunContainer(_ context.Context, spec plan.RunSpec) error {
	*f.calls = append(*f.calls, "run")
	f.ran = &spec
	return f.runErr
}

// fakePlanner is a RunPlanner that returns a fixed spec or error.
type fakePlanner struct {
	calls    *[]string
	spec     plan.RunSpec
	err      error
	gotReq   Request
	gotImage string
	gotRep   *Reporter
}

func (f *fakePlanner) Plan(request Request, imageRef string, rep *Reporter) (plan.RunSpec, error) {
	*f.calls = append(*f.calls, "plan")
	f.gotReq, f.gotImage, f.gotRep = request, imageRef, rep
	return f.spec, f.err
}

// currentImage is an image whose labels match the runtime image that caged
// version builds.
func currentImage(version string) engine.ImageInfo {
	labels := map[string]string{images.LabelVersion: version}
	for _, label := range images.Base(version).Identity {
		labels[label.Key] = label.Value
	}
	return engine.ImageInfo{ID: "sha256:current", OS: "linux", Arch: "amd64", Labels: labels}
}

// currentExtensionImage is an extension image that matches ext built on the
// base image baseID by caged version.
func currentExtensionImage(ext plan.Extension, baseID, version string) engine.ImageInfo {
	labels := map[string]string{images.LabelVersion: version}
	for _, label := range images.Extension(ext, baseID, version).Identity {
		labels[label.Key] = label.Value
	}
	return engine.ImageInfo{ID: "sha256:current-ext", OS: "linux", Arch: "amd64", Labels: labels}
}

// testExtension is a small extension used by several tests.
func testExtension() *plan.Extension {
	return &plan.Extension{Source: "~/.config/caged/image.json", Apt: []string{"neovim", "tmux"}}
}

// fakeProber is a Prober with fixed answers. It logs "read <ref> <path>" and
// "probe <ref> <host path>".
type fakeProber struct {
	calls    *[]string
	versions string
	readErr  error
	probeErr error
}

func (f *fakeProber) ReadImageFile(_ context.Context, ref, path string) ([]byte, error) {
	*f.calls = append(*f.calls, "read "+ref+" "+path)
	return []byte(f.versions), f.readErr
}

func (f *fakeProber) ProbeMount(_ context.Context, ref string, m plan.Mount) error {
	*f.calls = append(*f.calls, "probe "+ref+" "+m.HostPath)
	return f.probeErr
}

// fakeLister is an ImageLister that answers by query. It logs "list <query>".
type fakeLister struct {
	calls *[]string
	by    map[string][]engine.ImageSummary // key: the query's Label, else its Reference
	err   error
}

func (f *fakeLister) ListImages(_ context.Context, q engine.ImageQuery) ([]engine.ImageSummary, error) {
	key := q.Label
	if key == "" {
		key = q.Reference
	}
	*f.calls = append(*f.calls, "list "+key)
	return f.by[key], f.err
}
