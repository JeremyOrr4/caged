package app

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/images"
	"github.com/JeremyOrr4/caged/internal/plan"
)

func TestOlderThan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version           string
		wantOlder, wantOK bool
	}{
		{version: "27.3.1", wantOlder: false, wantOK: true},
		{version: "25.0", wantOlder: false, wantOK: true},
		{version: "25.0.0", wantOlder: false, wantOK: true},
		{version: "24.0.7", wantOlder: true, wantOK: true},
		{version: "25.0-beta", wantOlder: false, wantOK: true},
		{version: "28.0.0-rc.1", wantOlder: false, wantOK: true},
		{version: "9.9", wantOlder: true, wantOK: true},
		{version: "100.0", wantOlder: false, wantOK: true},
		{version: "garbage", wantOK: false},
		{version: "", wantOK: false},
		{version: "27", wantOK: false},
		{version: "27.", wantOK: false},
		{version: ".5", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.version), func(t *testing.T) {
			t.Parallel()
			older, ok := olderThan(tt.version, minDockerEngine)
			if older != tt.wantOlder || ok != tt.wantOK {
				t.Fatalf("olderThan(%q, %q) = %v, %v; want %v, %v", tt.version, minDockerEngine, older, ok, tt.wantOlder, tt.wantOK)
			}
		})
	}
	if older, ok := olderThan("25.0", "garbage"); older || ok {
		t.Fatalf("olderThan with an unreadable minimum = %v, %v; want false, false", older, ok)
	}
}

func TestStatusAndReport(t *testing.T) {
	t.Parallel()

	for status, want := range map[Status]string{StatusPass: "PASS", StatusWarn: "WARN", StatusFail: "FAIL", StatusSkip: "SKIP"} {
		if got := status.String(); got != want {
			t.Errorf("Status(%d).String() = %q, want %q", status, got, want)
		}
	}
	report := DoctorReport{Results: []Result{{Status: StatusPass}, {Status: StatusFail}, {Status: StatusWarn}, {Status: StatusFail}, {Status: StatusSkip}}}
	if got := report.Failed(); got != 2 {
		t.Errorf("Failed() = %d, want 2", got)
	}
	if got := (&ChecksFailedError{Failed: 2}).Error(); got != "doctor: 2 checks failed" {
		t.Errorf("ChecksFailedError = %q", got)
	}
}

// doctorCase is one scenario for Diagnose. The zero value is a healthy Linux
// machine with a current runtime image and no config file.
type doctorCase struct {
	host      Host
	daemon    fakeDaemon
	images    map[string]engine.ImageInfo // nil means only a current runtime image
	exts      fakeExts
	status    *plan.ConfigStatus // nil means no config file
	statusErr error
	configs   *fakeConfigs // nil means no config mounts
	versions  string
	readErr   error
	probeErr  error
	workDir   string
	project   *fakeProject
	env       *fakeEnv
	version   string
}

const healthyVersions = "claude=2.1.3\ncodex=0.46.0\ngemini=0.9.0\ncopilot=0.0.330\npi=0.12.0\ngh=2.81.0\n"

// diagnose runs the doctor on the case and returns "NAME STATUS detail" lines
// plus the calls the fakes saw.
func (c doctorCase) diagnose(t *testing.T) (DoctorReport, []string) {
	t.Helper()
	version := c.version
	if version == "" {
		version = "0.1.0"
	}
	if c.host == (Host{}) {
		c.host = Host{OS: "linux", Arch: "amd64"}
	}
	var calls []string
	stored := c.images
	if stored == nil {
		stored = baseImage(currentImage(version))
	}
	store := &fakeStore{calls: &calls, images: stored}
	versions := c.versions
	if versions == "" && c.readErr == nil {
		versions = healthyVersions
	}
	prober := &fakeProber{calls: &calls, versions: versions, readErr: c.readErr, probeErr: c.probeErr}
	daemon := c.daemon
	daemon.calls = &calls
	exts := c.exts
	exts.calls = &calls
	project := c.project
	if project == nil {
		project = &fakeProject{}
	}
	env := c.env
	if env == nil {
		env = &fakeEnv{home: "/home/me", wd: "/home/me/code/app"}
	}
	status := plan.ConfigStatus{Source: "~/.config/caged/config.yaml"}
	if c.status != nil {
		status = *c.status
	}
	configFile := &fakeConfigFile{status: status, err: c.statusErr}
	configs := c.configs
	if configs == nil {
		configs = &fakeConfigs{}
	}
	doctor := NewDoctor(DoctorDeps{Daemon: &daemon, Store: store, Prober: prober, Extensions: &exts, Config: configFile, Configs: configs, Project: project, Env: env, Host: c.host, Version: version})
	report := doctor.Diagnose(context.Background(), DoctorOptions{WorkDir: c.workDir})
	return report, calls
}

func lines(report DoctorReport) []string {
	var out []string
	for _, result := range report.Results {
		out = append(out, fmt.Sprintf("%s %s %s", result.Name, result.Status, result.Detail))
	}
	return out
}

func TestDoctorHealthyMachine(t *testing.T) {
	t.Parallel()

	report, calls := doctorCase{daemon: fakeDaemon{info: engine.DaemonInfo{CLIPath: "/usr/local/bin/docker", ServerVersion: "27.3.1", OperatingSystem: "Docker Desktop", Arch: "amd64"}}}.diagnose(t)
	hash := images.ContextHash(images.Base("0.1.0").Files)[:12]
	want := []string{
		"docker PASS /usr/local/bin/docker",
		"daemon PASS Docker Desktop, engine 27.3.1, linux/amd64",
		"image PASS caged-runtime:latest is current (runtime " + hash + ", built by caged 0.1.0)",
		"config PASS no config file (~/.config/caged/config.yaml)",
		"links PASS no symlinked config",
		"ext-image PASS not used",
		"arch PASS image linux/amd64 matches the Docker VM (amd64)",
		"project PASS /home/me/code/app can be mounted",
		"versions PASS claude 2.1.3, codex 0.46.0, copilot 0.0.330, gemini 0.9.0, gh 2.81.0, pi 0.12.0",
	}
	if got := lines(report); !reflect.DeepEqual(got, want) {
		t.Fatalf("Diagnose() =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if report.Header != "caged 0.1.0 (linux/amd64)" || report.Failed() != 0 {
		t.Fatalf("header = %q, failed = %d", report.Header, report.Failed())
	}
	wantCalls := []string{"daemon", "inspect caged-runtime:latest", "load", "inspect caged-runtime:ext", "probe caged-runtime:latest /home/me/code/app", "read caged-runtime:latest /opt/caged/versions"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %q, want %q (each question asked once)", calls, wantCalls)
	}
}

func TestDoctorRows(t *testing.T) {
	t.Parallel()

	const version = "0.1.0"
	notFoundErr := errors.New("boom")
	ext := testExtension()
	staleBase := engine.ImageInfo{ID: "sha256:old", OS: "linux", Arch: "amd64", Labels: map[string]string{images.LabelRole: "runtime", images.LabelRuntimeHash: "old", images.LabelVersion: "0.0.1"}}
	staleExt := engine.ImageInfo{ID: "sha256:old-ext", OS: "linux", Arch: "amd64", Labels: map[string]string{images.LabelRole: "extension", images.LabelExtensionHash: "old", images.LabelBaseID: "sha256:current", images.LabelVersion: version}}
	both := func(ext engine.ImageInfo) map[string]engine.ImageInfo {
		return map[string]engine.ImageInfo{images.BaseRef: currentImage(version), images.ExtensionRef: ext}
	}
	old := engine.DaemonInfo{CLIPath: "/d", ServerVersion: "24.0.7", OperatingSystem: "Docker Desktop", Arch: "amd64"}
	tests := []struct {
		name  string
		c     doctorCase
		check string // which row to look at
		want  string // "STATUS detail"
	}{
		// docker
		{name: "docker missing", c: doctorCase{daemon: fakeDaemon{err: &engine.Error{Kind: engine.KindNotInstalled, Message: "Docker is not installed or not on PATH", Err: errors.New("not found")}}}, check: "docker", want: "FAIL Docker is not installed or not on PATH: not found"},
		{name: "unsupported host", c: doctorCase{daemon: fakeDaemon{err: &engine.Error{Kind: engine.KindUnsupportedHost, Message: `unsupported host architecture "386"`}}}, check: "docker", want: `FAIL unsupported host architecture "386"`},
		{name: "docker is fine when only the daemon is down", c: doctorCase{daemon: fakeDaemon{err: &engine.Error{Kind: engine.KindDaemonDown, Message: "Docker is installed but the daemon is unavailable; start Docker Desktop"}}}, check: "docker", want: "PASS installed"},

		// daemon
		{name: "daemon down", c: doctorCase{daemon: fakeDaemon{err: &engine.Error{Kind: engine.KindDaemonDown, Message: "Docker is installed but the daemon is unavailable; start Docker Desktop", Err: errors.New("no socket")}}}, check: "daemon", want: "FAIL Docker is installed but the daemon is unavailable; start Docker Desktop: no socket"},
		{name: "old engine", c: doctorCase{daemon: fakeDaemon{info: old}}, check: "daemon", want: "WARN Docker Desktop, engine 24.0.7, linux/amd64; Docker Engine 24.0.7 is older than the minimum 25.0 caged is tested with; update Docker Desktop"},
		{name: "engine exactly at the minimum", c: doctorCase{daemon: fakeDaemon{info: engine.DaemonInfo{CLIPath: "/d", ServerVersion: "25.0", OperatingSystem: "Docker Desktop", Arch: "amd64"}}}, check: "daemon", want: "PASS Docker Desktop, engine 25.0, linux/amd64"},
		{name: "unreadable engine version", c: doctorCase{daemon: fakeDaemon{info: engine.DaemonInfo{CLIPath: "/d", ServerVersion: "garbage", OperatingSystem: "Docker Desktop", Arch: "amd64"}}}, check: "daemon", want: `WARN Docker Desktop, engine garbage, linux/amd64; could not read the Docker Engine version "garbage"`},
		{name: "another runtime on macOS", c: doctorCase{host: Host{OS: "darwin", Arch: "arm64"}, daemon: fakeDaemon{info: engine.DaemonInfo{CLIPath: "/d", ServerVersion: "27.0.0", OperatingSystem: "OrbStack", Arch: "arm64"}}}, check: "daemon", want: "WARN OrbStack, engine 27.0.0, linux/arm64; untested runtime; caged is tested with Docker Desktop on macOS"},
		{name: "another runtime on Linux is fine", c: doctorCase{daemon: fakeDaemon{info: engine.DaemonInfo{CLIPath: "/d", ServerVersion: "27.0.0", OperatingSystem: "Ubuntu 24.04", Arch: "amd64"}}}, check: "daemon", want: "PASS Ubuntu 24.04, engine 27.0.0, linux/amd64"},
		{name: "amd64 caged under Rosetta", c: doctorCase{host: Host{OS: "darwin", Arch: "amd64"}, daemon: fakeDaemon{info: engine.DaemonInfo{CLIPath: "/d", ServerVersion: "27.0.0", OperatingSystem: "Docker Desktop", Arch: "arm64"}}}, check: "daemon", want: "WARN Docker Desktop, engine 27.0.0, linux/arm64; caged is an amd64 build running under Rosetta; install the arm64 build"},
		{name: "architecture mismatch elsewhere", c: doctorCase{host: Host{OS: "linux", Arch: "arm64"}, daemon: fakeDaemon{info: engine.DaemonInfo{CLIPath: "/d", ServerVersion: "27.0.0", OperatingSystem: "Docker Desktop", Arch: "amd64"}}}, check: "daemon", want: "WARN Docker Desktop, engine 27.0.0, linux/amd64; caged is an arm64 build but Docker runs amd64; install the amd64 build"},

		// image
		{name: "image missing", c: doctorCase{images: map[string]engine.ImageInfo{}}, check: "image", want: "WARN caged-runtime:latest is not built yet; the next run builds it, or run caged build"},
		{name: "image stale", c: doctorCase{images: baseImage(staleBase)}, check: "image", want: "WARN caged-runtime:latest is out of date; the next run rebuilds it"},
		{name: "image unlabeled", c: doctorCase{images: baseImage(engine.ImageInfo{ID: "sha256:x", Arch: "amd64"})}, check: "image", want: "WARN caged-runtime:latest is out of date; the next run rebuilds it"},

		// config
		{name: "invalid config file", c: doctorCase{statusErr: errors.New(`~/.config/caged/config.yaml: line 1: unknown field "imgae"`)}, check: "config", want: `FAIL ~/.config/caged/config.yaml: line 1: unknown field "imgae"`},
		{name: "config file the extension cannot load", c: doctorCase{exts: fakeExts{err: errors.New("~/.config/caged/config.yaml: bad apt")}}, check: "config", want: "FAIL ~/.config/caged/config.yaml: bad apt"},
		{name: "config with packages and links", c: doctorCase{exts: fakeExts{ext: ext}, status: &plan.ConfigStatus{Source: "~/.config/caged/config.yaml", Found: true, Apt: []string{"neovim", "tmux"}, LinkAllow: 1}}, check: "config", want: "PASS ~/.config/caged/config.yaml (apt: neovim, tmux; links.allow: 1)"},
		{name: "config with workdirs", c: doctorCase{status: &plan.ConfigStatus{Source: "~/.config/caged/config.yaml", Found: true, Workdirs: true, WorkdirAllow: 2}}, check: "config", want: "PASS ~/.config/caged/config.yaml (links.allow: 0; workdirs.allow: 2)"},
		{name: "an empty config file", c: doctorCase{status: &plan.ConfigStatus{Source: "~/.config/caged/config.yaml", Found: true}}, check: "config", want: "PASS ~/.config/caged/config.yaml (links.allow: 0)"},
		{name: "a leftover image.json", c: doctorCase{status: &plan.ConfigStatus{Source: "~/.config/caged/config.yaml", Legacy: "~/.config/caged/image.json"}}, check: "config", want: "WARN no config file (~/.config/caged/config.yaml); ~/.config/caged/image.json is no longer read: move its packages to image.apt in ~/.config/caged/config.yaml"},
		{name: "a leftover image.json next to a config file", c: doctorCase{status: &plan.ConfigStatus{Source: "~/.config/caged/config.yaml", Found: true, Legacy: "~/.config/caged/image.json"}}, check: "config", want: "WARN ~/.config/caged/config.yaml (links.allow: 0); ~/.config/caged/image.json is no longer read: move its packages to image.apt in ~/.config/caged/config.yaml"},

		// links
		{name: "followed links are counted", c: doctorCase{configs: &fakeConfigs{report: plan.ConfigReport{Mounts: []plan.ConfigMount{{Name: ".claude"}, {Name: ".claude", Link: "/h/.claude/a"}, {Name: ".claude", Link: "/h/.claude/b"}}}}}, check: "links", want: "PASS 2 symlinked config mounts allowed"},
		{name: "a refusal with a hint warns", c: doctorCase{configs: &fakeConfigs{report: plan.ConfigReport{Refused: []plan.Refusal{
			{Path: "/h/.claude/skip", Reason: "dangling or unresolvable symlink"},
			{Path: "/h/.claude/CLAUDE.md", Reason: "symlink target ~/dot/CLAUDE.md is not under links.allow", Hint: `add "- path: ~/dot" under links.allow in ~/.config/caged/config.yaml`},
		}}}}, check: "links", want: `WARN /h/.claude/CLAUDE.md (symlink target ~/dot/CLAUDE.md is not under links.allow); add "- path: ~/dot" under links.allow in ~/.config/caged/config.yaml`},
		{name: "a refusal without a hint is not a warning", c: doctorCase{configs: &fakeConfigs{report: plan.ConfigReport{Refused: []plan.Refusal{{Path: "/h/.claude/skip", Reason: "dangling or unresolvable symlink"}}}}}, check: "links", want: "PASS no symlinked config"},
		{name: "the link policy cannot be loaded", c: doctorCase{configs: &fakeConfigs{err: errors.New("~/.config/caged/config.yaml: links.allow entry \"~\" is too broad: it contains the home directory")}}, check: "links", want: `FAIL ~/.config/caged/config.yaml: links.allow entry "~" is too broad: it contains the home directory`},
		{name: "links need a home directory", c: doctorCase{env: &fakeEnv{homeErr: errors.New("unset"), wd: "/w"}}, check: "links", want: "FAIL find home directory: unset"},

		// ext-image
		{name: "extension not built yet", c: doctorCase{exts: fakeExts{ext: ext}}, check: "ext-image", want: "WARN caged-runtime:ext is not built yet; the next run builds it"},
		{name: "extension image current", c: doctorCase{exts: fakeExts{ext: ext}, images: both(currentExtensionImage(*ext, "sha256:current", version))}, check: "ext-image", want: "PASS caged-runtime:ext is current (apt: neovim, tmux)"},
		{name: "extension image stale", c: doctorCase{exts: fakeExts{ext: ext}, images: both(staleExt)}, check: "ext-image", want: "WARN caged-runtime:ext is out of date; the next run rebuilds it"},
		{name: "extension image built on another base", c: doctorCase{exts: fakeExts{ext: ext}, images: both(currentExtensionImage(*ext, "sha256:other", version))}, check: "ext-image", want: "WARN caged-runtime:ext is out of date; the next run rebuilds it"},
		{name: "extension image left over with no file", c: doctorCase{images: both(staleExt)}, check: "ext-image", want: "WARN caged-runtime:ext is no longer used (config.yaml lists no image.apt packages); caged clean removes it"},
		{name: "extension needs the base image first", c: doctorCase{exts: fakeExts{ext: ext}, images: map[string]engine.ImageInfo{}}, check: "ext-image", want: "WARN caged-runtime:ext is not built yet; the next run builds it"},

		// arch
		{name: "image arch differs", c: doctorCase{images: baseImage(engine.ImageInfo{ID: "x", OS: "linux", Arch: "arm64", Labels: currentImage(version).Labels})}, check: "arch", want: "FAIL image linux/arm64 does not match Docker (amd64); it runs under emulation; run caged build --no-cache"},
		{name: "no image to check the arch of", c: doctorCase{images: map[string]engine.ImageInfo{}}, check: "arch", want: "SKIP no image built yet"},
		{name: "arch uses the extension image when it exists", c: doctorCase{exts: fakeExts{ext: ext}, images: both(engine.ImageInfo{ID: "x", Arch: "arm64", Labels: currentExtensionImage(*ext, "sha256:current", version).Labels})}, check: "arch", want: "FAIL image linux/arm64 does not match Docker (amd64); it runs under emulation; run caged build --no-cache"},

		// project
		{name: "project dir shared (macOS)", c: doctorCase{host: Host{OS: "darwin", Arch: "amd64"}, workDir: "/Users/me/code/app"}, check: "project", want: "PASS /Users/me/code/app is shared with Docker Desktop"},
		{name: "probe fails", c: doctorCase{probeErr: errors.New("Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: exit status 125")}, check: "project", want: "FAIL Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: exit status 125"},
		{name: "project is the home directory", c: doctorCase{workDir: "/home/me"}, check: "project", want: "WARN /home/me is the home directory or /; caged refuses to mount it without --allow-broad-mount (not probed)"},
		{name: "project is /", c: doctorCase{workDir: "/"}, check: "project", want: "WARN / is the home directory or /; caged refuses to mount it without --allow-broad-mount (not probed)"},
		{name: "project dir is refused by the resolver", c: doctorCase{project: &fakeProject{err: errors.New(`inspect working directory "/nonexistent": stat /nonexistent: no such file or directory`)}, workDir: "/nonexistent"}, check: "project", want: `FAIL inspect working directory "/nonexistent": stat /nonexistent: no such file or directory`},
		{name: "no cwd", c: doctorCase{env: &fakeEnv{home: "/home/me", wdErr: errors.New("gone")}}, check: "project", want: "FAIL get current working directory: gone"},
		{name: "no home", c: doctorCase{env: &fakeEnv{homeErr: errors.New("unset"), wd: "/w"}}, check: "project", want: "FAIL find home directory: unset"},
		{name: "no image to probe with", c: doctorCase{images: map[string]engine.ImageInfo{}}, check: "project", want: "SKIP no image built yet to probe with"},

		// versions
		{name: "versions file missing", c: doctorCase{readErr: notFoundErr}, check: "versions", want: "WARN no versions file in caged-runtime:latest (the image was built before this release)"},
		{name: "versions file empty", c: doctorCase{versions: "\n"}, check: "versions", want: "WARN the versions file in caged-runtime:latest is empty"},
		{name: "unknown version is shown", c: doctorCase{versions: "pi=unknown\nclaude=1.0\n"}, check: "versions", want: "PASS claude 1.0, pi unknown"},
		{name: "no image to read from", c: doctorCase{images: map[string]engine.ImageInfo{}}, check: "versions", want: "SKIP no image built yet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			report, _ := tt.c.diagnose(t)
			for _, result := range report.Results {
				if result.Name == tt.check {
					if got := result.Status.String() + " " + result.Detail; got != tt.want {
						t.Fatalf("%s = %q, want %q", tt.check, got, tt.want)
					}
					return
				}
			}
			t.Fatalf("no %q row in %v", tt.check, lines(report))
		})
	}
}

func TestDoctorSkipsWhatDependsOnAFailure(t *testing.T) {
	t.Parallel()

	t.Run("daemon down skips everything that needs it", func(t *testing.T) {
		t.Parallel()
		down := &engine.Error{Kind: engine.KindDaemonDown, Message: "daemon unavailable"}
		report, calls := doctorCase{daemon: fakeDaemon{err: down}}.diagnose(t)
		want := []string{
			"docker PASS installed",
			"daemon FAIL daemon unavailable",
			"image SKIP needs daemon",
			"config PASS no config file (~/.config/caged/config.yaml)",
			"links PASS no symlinked config",
			"ext-image SKIP needs daemon",
			"arch SKIP needs daemon",
			"project SKIP needs daemon",
			"versions SKIP needs daemon",
		}
		if got := lines(report); !reflect.DeepEqual(got, want) {
			t.Fatalf("Diagnose() =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
		if report.Failed() != 1 {
			t.Fatalf("Failed() = %d, want 1: skipped checks do not count", report.Failed())
		}
		for _, call := range calls {
			if strings.HasPrefix(call, "inspect") || strings.HasPrefix(call, "probe") || strings.HasPrefix(call, "read") {
				t.Fatalf("calls = %q: nothing but the daemon check may run", calls)
			}
		}
	})
	t.Run("no docker skips the daemon check too", func(t *testing.T) {
		t.Parallel()
		report, _ := doctorCase{daemon: fakeDaemon{err: &engine.Error{Kind: engine.KindNotInstalled, Message: "Docker is not installed or not on PATH"}}}.diagnose(t)
		got := lines(report)
		if got[0] != "docker FAIL Docker is not installed or not on PATH" || got[1] != "daemon SKIP needs docker" || got[2] != "image SKIP needs daemon" {
			t.Fatalf("Diagnose() = %v", got)
		}
	})
	t.Run("an invalid config file skips what needs it", func(t *testing.T) {
		t.Parallel()
		report, _ := doctorCase{statusErr: errors.New("bad file")}.diagnose(t)
		got := lines(report)
		want := map[int]string{
			3: "config FAIL bad file",
			4: "links SKIP needs config",
			5: "ext-image SKIP needs config",
			7: "project SKIP needs config",
		}
		for index, line := range want {
			if got[index] != line {
				t.Fatalf("row %d = %q, want %q; Diagnose() = %v", index, got[index], line, got)
			}
		}
		if !strings.HasPrefix(got[2], "image PASS") || !strings.HasPrefix(got[6], "arch PASS") || !strings.HasPrefix(got[8], "versions PASS") {
			t.Fatalf("Diagnose() = %v", got)
		}
	})
}

func TestDoctorNeverBuildsOrRemoves(t *testing.T) {
	t.Parallel()

	// The doctor's dependencies do not even have a build or remove method: the
	// store is an ImageInspector. This pins that, and that a missing image only
	// leads to warnings.
	var _ ImageInspector = (*fakeStore)(nil)
	report, calls := doctorCase{images: map[string]engine.ImageInfo{}, exts: fakeExts{ext: testExtension()}}.diagnose(t)
	for _, call := range calls {
		if strings.HasPrefix(call, "build") || strings.HasPrefix(call, "remove") {
			t.Fatalf("calls = %q: doctor must not build or remove anything", calls)
		}
	}
	if report.Failed() != 0 {
		t.Fatalf("missing images must only warn: %v", lines(report))
	}
}

func TestDoctorUsesTheWorkDirOption(t *testing.T) {
	t.Parallel()

	project := &fakeProject{}
	report, calls := doctorCase{workDir: "/srv/other", project: project}.diagnose(t)
	if project.gotWorkDir != "/srv/other" || project.gotHome != "/home/me" || !project.gotAllow {
		t.Fatalf("resolver got %+v", project)
	}
	if !reflect.DeepEqual(calls[len(calls)-2:], []string{"probe caged-runtime:latest /srv/other", "read caged-runtime:latest /opt/caged/versions"}) {
		t.Fatalf("calls = %q", calls)
	}
	if report.Results[7].Detail != "/srv/other can be mounted" {
		t.Fatalf("project row = %+v", report.Results[7])
	}
}
