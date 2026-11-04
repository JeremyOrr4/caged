package app

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/JeremyOrr4/caged/internal/images"
)

// check is one row of `caged doctor`. A check whose needs failed, were
// skipped, or have not run yet is skipped without running.
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
		{name: "config", run: d.checkConfig},
		{name: "links", needs: []string{"config"}, run: d.checkLinks},
		{name: "ext-image", needs: []string{"config", "daemon", "image"}, run: d.checkExtensionImage},
		{name: "arch", needs: []string{"daemon"}, run: checkArch},
		{name: "project", needs: []string{"daemon", "config"}, run: d.checkProject},
		{name: "versions", needs: []string{"daemon"}, run: checkVersions},
	}
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

func (d *Doctor) checkConfig(_ context.Context, f *facts) Result {
	status, err := d.deps.Config.Status()
	if err != nil {
		return Result{Status: StatusFail, Detail: err.Error()}
	}
	if _, err := f.extension(); err != nil {
		return Result{Status: StatusFail, Detail: err.Error()}
	}
	detail := "no config file (" + status.Source + ")"
	if status.Found {
		var parts []string
		if len(status.Apt) > 0 {
			parts = append(parts, "apt: "+strings.Join(status.Apt, ", "))
		}
		parts = append(parts, fmt.Sprintf("links.allow: %d", status.LinkAllow))
		if status.Workdirs {
			parts = append(parts, fmt.Sprintf("workdirs.allow: %d", status.WorkdirAllow))
		}
		if status.Created {
			parts = append(parts, "starter file created just now")
		}
		detail = status.Source + " (" + strings.Join(parts, "; ") + ")"
	}
	if status.Legacy != "" {
		return Result{Status: StatusWarn, Detail: detail + "; " + status.Legacy + " is no longer read: move its packages to image.apt in " + status.Source}
	}
	return Result{Status: StatusPass, Detail: detail}
}

func (d *Doctor) checkLinks(_ context.Context, f *facts) Result {
	home, err := f.home.get()
	if err != nil {
		return Result{Status: StatusFail, Detail: err.Error()}
	}
	report, err := d.deps.Configs.Resolve(home)
	if err != nil {
		return Result{Status: StatusFail, Detail: err.Error()}
	}
	var refused []string
	for _, refusal := range report.Refused {
		if refusal.Hint != "" {
			refused = append(refused, refusal.String())
		}
	}
	if len(refused) > 0 {
		return Result{Status: StatusWarn, Detail: strings.Join(refused, "; ")}
	}
	followed := 0
	for _, mount := range report.Mounts {
		if mount.Link != "" {
			followed++
		}
	}
	if followed == 0 {
		return Result{Status: StatusPass, Detail: "no symlinked config"}
	}
	return Result{Status: StatusPass, Detail: fmt.Sprintf("%d symlinked config mounts allowed", followed)}
}

func (d *Doctor) checkExtensionImage(ctx context.Context, f *facts) Result {
	ext, _ := f.extension() // cannot fail: this check needs config, which fails when the extension does
	built := f.image(ctx, images.ExtensionRef)
	if built.err != nil {
		return Result{Status: StatusWarn, Detail: fmt.Sprintf("could not inspect %s: %v", images.ExtensionRef, built.err)}
	}
	if ext == nil {
		if built.found {
			return Result{Status: StatusWarn, Detail: images.ExtensionRef + " is no longer used (config.yaml lists no image.apt packages); caged clean removes it"}
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
	info, _ := f.daemon(ctx) // cannot fail: this check needs daemon, which fails when the daemon does
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
	home, err := f.home.get()
	if err != nil {
		return Result{Status: StatusFail, Detail: err.Error()}
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
	for line := range strings.SplitSeq(text, "\n") {
		name, version, found := strings.Cut(strings.TrimSpace(line), "=")
		if found && name != "" {
			entries = append(entries, name+" "+version)
		}
	}
	slices.Sort(entries)
	return entries
}
