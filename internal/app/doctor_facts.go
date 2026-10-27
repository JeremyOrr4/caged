package app

import (
	"context"

	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/images"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// facts caches what the checks look up (docker, the extension, the home
// directory), so each question is asked once however many checks need the
// answer.
type facts struct {
	doctor *Doctor
	opts   DoctorOptions
	home   homeLookup

	daemonDone bool
	daemonInfo engine.DaemonInfo
	daemonErr  error

	extDone bool
	ext     *plan.Extension
	extErr  error

	inspected map[string]inspection
}

// inspection is the cached answer to InspectImage for one reference.
type inspection struct {
	info  engine.ImageInfo
	found bool
	err   error // a failure other than "not found"
}

func (f *facts) daemon(ctx context.Context) (engine.DaemonInfo, error) {
	if !f.daemonDone {
		f.daemonDone = true
		f.daemonInfo, f.daemonErr = f.doctor.deps.Daemon.Daemon(ctx)
	}
	return f.daemonInfo, f.daemonErr
}

func (f *facts) extension() (*plan.Extension, error) {
	if !f.extDone {
		f.extDone = true
		f.ext, f.extErr = f.doctor.deps.Extensions.Load()
	}
	return f.ext, f.extErr
}

func (f *facts) image(ctx context.Context, ref string) inspection {
	if got, ok := f.inspected[ref]; ok {
		return got
	}
	var result inspection
	info, err := f.doctor.deps.Store.InspectImage(ctx, ref)
	switch {
	case err == nil:
		result = inspection{info: info, found: true}
	case engine.IsKind(err, engine.KindImageNotFound):
		result = inspection{}
	default:
		result = inspection{err: err}
	}
	if f.inspected == nil {
		f.inspected = map[string]inspection{}
	}
	f.inspected[ref] = result
	return result
}

// runImage is the image a run would use: the extension image when an extension
// is configured and built, else the runtime image. found is false when that
// image does not exist yet.
func (f *facts) runImage(ctx context.Context) (ref string, info engine.ImageInfo, found bool) {
	// An extension that cannot load is the config check's FAIL; the checks
	// that do not need config still look at the runtime image.
	if ext, err := f.extension(); err == nil && ext != nil {
		if built := f.image(ctx, images.ExtensionRef); built.found {
			return images.ExtensionRef, built.info, true
		}
	}
	base := f.image(ctx, images.BaseRef)
	return images.BaseRef, base.info, base.found
}
