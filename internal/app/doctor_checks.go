package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/images"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// check is one row of `caged doctor`. A check whose needs ended in FAIL or SKIP
// is skipped without running.
type check struct {
	name  string
	needs []string
	run   func(context.Context, *facts) Result
}

// checks returns every check, in the order doctor prints them.
func (d *Doctor) checks() []check {
	return []check{
		{name: "docker", run: checkDocker},
		{name: "daemon", needs: []string{"docker"}, run: d.checkDaemon},
		{name: "image", needs: []string{"daemon"}, run: d.checkImage},
		{name: "extension", run: checkExtension},
		{name: "ext-image", needs: []string{"extension", "daemon", "image"}, run: d.checkExtensionImage},
		{name: "arch", needs: []string{"daemon"}, run: checkArch},
		{name: "project", needs: []string{"daemon"}, run: d.checkProject},
		{name: "versions", needs: []string{"daemon"}, run: checkVersions},
	}
}

// facts caches what the checks look up, so each docker question is asked once
// however many checks need the answer.
type facts struct {
	doctor *Doctor
	opts   DoctorOptions

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
	if ext, err := f.extension(); err == nil && ext != nil {
		if built := f.image(ctx, images.ExtensionRef); built.found {
			return images.ExtensionRef, built.info, true
		}
	}
	base := f.image(ctx, images.BaseRef)
	return images.BaseRef, base.info, base.found
}

func checkDocker(ctx context.Context, f *facts) Result {
	info, err := f.daemon(ctx)
	switch {
	case err != nil && engineKindIsHostProblem(err):
		return Result{Status: StatusFail, Detail: err.Error()}
	case err != nil:
		return Result{Status: StatusPass, Detail: "installed"}
	default:
		return Result{Status: StatusPass, Detail: info.CLIPath}
	}
}

func (d *Doctor) checkDaemon(ctx context.Context, f *facts) Result {
	info, err := f.daemon(ctx)
	if err != nil {
		return Result{Status: StatusFail, Detail: err.Error()}
	}
	detail := fmt.Sprintf("%s, engine %s, linux/%s", info.OperatingSystem, info.ServerVersion, info.Arch)
	var warnings []string
	switch older, ok := olderThan(info.ServerVersion, minDockerEngine); {
	case !ok:
		warnings = append(warnings, fmt.Sprintf("could not read the Docker Engine version %q", info.ServerVersion))
	case older:
		warnings = append(warnings, fmt.Sprintf("Docker Engine %s is older than the minimum %s caged is tested with; update Docker Desktop", info.ServerVersion, minDockerEngine))
	}
	if d.deps.Host.OS == "darwin" && info.OperatingSystem != "Docker Desktop" {
		warnings = append(warnings, "untested runtime; caged is tested with Docker Desktop on macOS")
	}
	if host := d.deps.Host; host.Arch != info.Arch {
		if host.OS == "darwin" && host.Arch == "amd64" && info.Arch == "arm64" {
			warnings = append(warnings, "caged is an amd64 build running under Rosetta; install the arm64 build")
		} else {
			warnings = append(warnings, fmt.Sprintf("caged is an %s build but Docker runs %s; install the %s build", host.Arch, info.Arch, info.Arch))
		}
	}
	if len(warnings) > 0 {
		return Result{Status: StatusWarn, Detail: detail + "; " + strings.Join(warnings, "; ")}
	}
	return Result{Status: StatusPass, Detail: detail}
}

func (d *Doctor) checkImage(ctx context.Context, f *facts) Result {
	base := images.Base(d.deps.Version)
	got := f.image(ctx, images.BaseRef)
	if got.err != nil {
		return Result{Status: StatusWarn, Detail: fmt.Sprintf("could not inspect %s: %v", images.BaseRef, got.err)}
	}
	freshness := base.Assess(got.info.Labels, got.found)
	switch freshness.State {
	case images.StateCurrent:
		detail := fmt.Sprintf("%s is current (runtime %.12s", images.BaseRef, images.ContextHash(base.Files))
		if freshness.BuiltBy != "" {
			detail += ", built by caged " + freshness.BuiltBy
		}
		return Result{Status: StatusPass, Detail: detail + ")"}
	case images.StateMissing:
		return Result{Status: StatusWarn, Detail: images.BaseRef + " is not built yet; the next run builds it, or run caged build"}
	default:
		return Result{Status: StatusWarn, Detail: images.BaseRef + " is out of date; the next run rebuilds it"}
	}
}

func checkExtension(_ context.Context, f *facts) Result {
	ext, err := f.extension()
	switch {
	case err != nil:
		return Result{Status: StatusFail, Detail: err.Error()}
	case ext == nil:
		return Result{Status: StatusPass, Detail: "no extension file"}
	default:
		return Result{Status: StatusPass, Detail: fmt.Sprintf("%s (apt: %s)", ext.Source, strings.Join(ext.Apt, ", "))}
	}
}

func (d *Doctor) checkExtensionImage(ctx context.Context, f *facts) Result {
	ext, _ := f.extension()
	built := f.image(ctx, images.ExtensionRef)
	if built.err != nil {
		return Result{Status: StatusWarn, Detail: fmt.Sprintf("could not inspect %s: %v", images.ExtensionRef, built.err)}
	}
	if ext == nil {
		if built.found {
			return Result{Status: StatusWarn, Detail: images.ExtensionRef + " is no longer used (there is no extension file); caged clean removes it"}
		}
		return Result{Status: StatusPass, Detail: "not used"}
	}
	base := f.image(ctx, images.BaseRef)
	if !built.found || !base.found {
		return Result{Status: StatusWarn, Detail: images.ExtensionRef + " is not built yet; the next run builds it"}
	}
	target := images.Extension(*ext, base.info.ID, d.deps.Version)
	if target.Assess(built.info.Labels, true).State != images.StateCurrent {
		return Result{Status: StatusWarn, Detail: images.ExtensionRef + " is out of date; the next run rebuilds it"}
	}
	return Result{Status: StatusPass, Detail: fmt.Sprintf("%s is current (apt: %s)", images.ExtensionRef, strings.Join(ext.Apt, ", "))}
}

func checkArch(ctx context.Context, f *facts) Result {
	info, _ := f.daemon(ctx)
	_, image, found := f.runImage(ctx)
	if !found {
		return Result{Status: StatusSkip, Detail: "no image built yet"}
	}
	if image.Arch != info.Arch {
		return Result{Status: StatusFail, Detail: fmt.Sprintf("image linux/%s does not match Docker (%s); it runs under emulation; run caged build --no-cache", image.Arch, info.Arch)}
	}
	return Result{Status: StatusPass, Detail: fmt.Sprintf("image linux/%s matches the Docker VM (%s)", image.Arch, info.Arch)}
}

func (d *Doctor) checkProject(ctx context.Context, f *facts) Result {
	workDir := f.opts.WorkDir
	if workDir == "" {
		cwd, err := d.deps.Env.Getwd()
		if err != nil {
			return Result{Status: StatusFail, Detail: fmt.Sprintf("get current working directory: %v", err)}
		}
		workDir = cwd
	}
	home, err := d.deps.Env.UserHomeDir()
	if err != nil {
		return Result{Status: StatusFail, Detail: fmt.Sprintf("find home directory: %v", err)}
	}
	// Resolve with broad mounts allowed, so that the only errors left are real
	// problems with the directory; broad mounts are reported as a warning.
	mount, err := d.deps.Project.Resolve(workDir, home, true)
	if err != nil {
		return Result{Status: StatusFail, Detail: err.Error()}
	}
	if clean := filepath.Clean(mount.HostPath); clean == filepath.Clean(home) || clean == string(filepath.Separator) {
		return Result{Status: StatusWarn, Detail: fmt.Sprintf("%s is the home directory or /; caged refuses to mount it without --allow-broad-mount (not probed)", mount.HostPath)}
	}
	ref, _, found := f.runImage(ctx)
	if !found {
		return Result{Status: StatusSkip, Detail: "no image built yet to probe with"}
	}
	if err := d.deps.Prober.ProbeMount(ctx, ref, *mount); err != nil {
		return Result{Status: StatusFail, Detail: err.Error()}
	}
	if d.deps.Host.OS == "darwin" {
		return Result{Status: StatusPass, Detail: mount.HostPath + " is shared with Docker Desktop"}
	}
	return Result{Status: StatusPass, Detail: mount.HostPath + " can be mounted"}
}

func checkVersions(ctx context.Context, f *facts) Result {
	ref, _, found := f.runImage(ctx)
	if !found {
		return Result{Status: StatusSkip, Detail: "no image built yet"}
	}
	data, err := f.doctor.deps.Prober.ReadImageFile(ctx, ref, images.VersionsPath)
	if err != nil {
		return Result{Status: StatusWarn, Detail: fmt.Sprintf("no versions file in %s (the image was built before this release)", ref)}
	}
	versions := parseVersions(string(data))
	if len(versions) == 0 {
		return Result{Status: StatusWarn, Detail: fmt.Sprintf("the versions file in %s is empty", ref)}
	}
	return Result{Status: StatusPass, Detail: strings.Join(versions, ", ")}
}

// parseVersions turns the image's name=version lines into "name version"
// entries sorted by name.
func parseVersions(text string) []string {
	var entries []string
	for _, line := range strings.Split(text, "\n") {
		name, version, found := strings.Cut(strings.TrimSpace(line), "=")
		if found && name != "" {
			entries = append(entries, name+" "+version)
		}
	}
	sort.Strings(entries)
	return entries
}
