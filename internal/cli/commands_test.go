package cli

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
)

func TestListCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		statuses []app.AgentStatus
		want     string
	}{
		{
			name: "aligned columns, with not found and dashes",
			statuses: []app.AgentStatus{
				{Name: "claude", Config: []string{"~/.claude"}, EnvSet: []string{"ANTHROPIC_API_KEY"}},
				{Name: "codex", Config: []string{"~/.codex"}},
				{Name: "copilot", EnvSet: []string{"GH_TOKEN", "GITHUB_TOKEN"}},
				{Name: "gemini", Config: []string{"~/.gemini (2 files)"}},
				{Name: "pi", Config: []string{"~/.config/pi", "~/.pi"}},
			},
			want: "AGENT    CONFIG               AUTH ENV SET\n" +
				"claude   ~/.claude            ANTHROPIC_API_KEY\n" +
				"codex    ~/.codex             -\n" +
				"copilot  not found            GH_TOKEN, GITHUB_TOKEN\n" +
				"gemini   ~/.gemini (2 files)  -\n" +
				"pi       ~/.config/pi, ~/.pi  -\n",
		},
		{name: "no agents still prints the header", want: "AGENT  CONFIG  AUTH ENV SET\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stdout, err := execute(t, &fakeService{statuses: tt.statuses}, "list")
			if err != nil || stdout != tt.want {
				t.Fatalf("list = %q, %v; want %q", stdout, err, tt.want)
			}
		})
	}
	t.Run("an error is returned", func(t *testing.T) {
		t.Parallel()
		stdout, err := execute(t, &fakeService{err: errors.New("find home directory: unset")}, "list")
		if err == nil || err.Error() != "find home directory: unset" || stdout != "" {
			t.Fatalf("list = %q, %v", stdout, err)
		}
	})
	t.Run("takes no arguments", func(t *testing.T) {
		t.Parallel()
		if _, err := execute(t, &fakeService{}, "list", "extra"); err == nil {
			t.Fatal("list accepted an argument")
		}
	})
}

func TestDoctorCommand(t *testing.T) {
	t.Parallel()

	report := app.DoctorReport{Header: "caged 0.1.0 (darwin/arm64)", Results: []app.Result{
		{Name: "docker", Status: app.StatusPass, Detail: "/usr/local/bin/docker"},
		{Name: "daemon", Status: app.StatusWarn, Detail: "old engine"},
		{Name: "extension", Status: app.StatusPass, Detail: "no extension file"},
		{Name: "project", Status: app.StatusSkip, Detail: "needs daemon"},
	}}
	t.Run("prints the header and one row per check, and exits 0 without failures", func(t *testing.T) {
		t.Parallel()
		service := &fakeService{report: report}
		stdout, err := execute(t, service, "doctor", "-w", "/work")
		want := "caged 0.1.0 (darwin/arm64)\n" +
			"PASS  docker     /usr/local/bin/docker\n" +
			"WARN  daemon     old engine\n" +
			"PASS  extension  no extension file\n" +
			"SKIP  project    needs daemon\n"
		if err != nil || stdout != want {
			t.Fatalf("doctor = %q, %v; want %q", stdout, err, want)
		}
		if !reflect.DeepEqual(service.doctorOpts, []app.DoctorOptions{{WorkDir: "/work"}}) {
			t.Fatalf("options = %+v", service.doctorOpts)
		}
	})
	t.Run("a failing check returns the summary error", func(t *testing.T) {
		t.Parallel()
		failing := report
		failing.Results = append([]app.Result{{Name: "docker", Status: app.StatusFail, Detail: "missing"}, {Name: "arch", Status: app.StatusFail, Detail: "x"}}, report.Results...)
		stdout, err := execute(t, &fakeService{report: failing}, "doctor")
		var failed *app.ChecksFailedError
		if !errors.As(err, &failed) || failed.Failed != 2 || err.Error() != "doctor: 2 checks failed" {
			t.Fatalf("doctor error = %v, want 2 failed checks", err)
		}
		if stdout == "" {
			t.Fatal("the rows must still be printed before the error")
		}
	})
	t.Run("takes no arguments", func(t *testing.T) {
		t.Parallel()
		if _, err := execute(t, &fakeService{}, "doctor", "extra"); err == nil {
			t.Fatal("doctor accepted an argument")
		}
	})
}

func TestCleanCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		results []app.CleanResult
		err     error
		want    string
		wantErr string
		dryRun  bool
	}{
		{
			name: "removed, untagged, and kept images",
			args: []string{"clean"},
			results: []app.CleanResult{
				{Ref: "caged-runtime:ext", Size: "412MB", Outcome: app.Removed},
				{Ref: "<untagged> sha256:1a2b3c4d5e6f", Size: "3.0GB", Outcome: app.Removed},
				{Ref: "caged-runtime:latest", Size: "3.1GB", Outcome: app.Kept, Reason: "in use by a container (remove it with docker rm first)"},
			},
			want: "removed caged-runtime:ext (412MB)\nremoved <untagged> sha256:1a2b3c4d5e6f (3.0GB)\nkept caged-runtime:latest: in use by a container (remove it with docker rm first)\n",
		},
		{
			name:    "dry run",
			args:    []string{"clean", "--dry-run"},
			dryRun:  true,
			results: []app.CleanResult{{Ref: "caged-runtime:latest", Size: "3.1GB", Outcome: app.WouldRemove}, {Ref: "caged-runtime:old", Outcome: app.WouldRemove}},
			want:    "would remove caged-runtime:latest (3.1GB)\nwould remove caged-runtime:old\n",
		},
		{name: "nothing to clean", args: []string{"clean"}, want: "nothing to clean\n"},
		{name: "an error after some removals prints what was done and fails", args: []string{"clean"}, results: []app.CleanResult{{Ref: "caged-runtime:ext", Size: "1MB", Outcome: app.Removed}}, err: errors.New("remove image x: boom"), want: "removed caged-runtime:ext (1MB)\n", wantErr: "remove image x: boom"},
		{name: "an error with nothing done prints nothing", args: []string{"clean"}, err: errors.New("daemon unavailable"), wantErr: "daemon unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			service := &fakeService{cleaned: tt.results, err: tt.err}
			stdout, err := execute(t, service, tt.args...)
			if (err == nil) != (tt.wantErr == "") || (err != nil && err.Error() != tt.wantErr) {
				t.Fatalf("clean error = %v, want %q", err, tt.wantErr)
			}
			if stdout != tt.want {
				t.Fatalf("clean output = %q, want %q", stdout, tt.want)
			}
			if !reflect.DeepEqual(service.cleanOpts, []app.CleanOptions{{DryRun: tt.dryRun}}) {
				t.Fatalf("options = %+v, want dry-run %v", service.cleanOpts, tt.dryRun)
			}
		})
	}
	t.Run("takes no arguments", func(t *testing.T) {
		t.Parallel()
		if _, err := execute(t, &fakeService{}, "clean", "extra"); err == nil {
			t.Fatal("clean accepted an argument")
		}
	})
}

func TestCompletionSuggestsAgentsWithDescriptions(t *testing.T) {
	t.Parallel()

	stdout, err := execute(t, &fakeService{}, "__complete", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"claude\tClaude Code\n", "gh\tGitHub CLI\n", "pi\tPi coding agent\n"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("completion output is missing %q:\n%s", want, stdout)
		}
	}
}

// TestDoctorAndCleanHelp pins the two help texts no golden file covers (D-14).
// Each want is the baseline binary's output (refactor-baseline-help.txt).
func TestDoctorAndCleanHelp(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"doctor": "Check that Docker is installed and running, that the runtime image is current, that the\n" +
			"config file is valid (config), that symlinked agent config is allowed (links), that the image\n" +
			"matches Docker's architecture, that the project directory can be mounted, and which CLI\n" +
			"versions the image contains. Exits 1 when a check fails. It never builds, pulls, or removes\n" +
			"anything, and never prints an environment value.\n" +
			"\n" +
			"Usage:\n" +
			"  caged doctor [flags]\n" +
			"\n" +
			"Flags:\n" +
			"  -h, --help             help for doctor\n" +
			"  -w, --workdir string   check that this host directory can be mounted (default: the current directory)\n",
		"clean": "Remove the runtime image, the extension image, and any older or replaced caged images, so they do not\n" +
			"outlive caged. Run it before uninstalling. An image that a container still uses is kept and\n" +
			"reported. It never touches ~/.config/caged, agent configs, containers, volumes, or the Docker build\n" +
			"cache (docker builder prune clears that).\n" +
			"\n" +
			"Usage:\n" +
			"  caged clean [flags]\n" +
			"\n" +
			"Flags:\n" +
			"      --dry-run   print what would be removed, and remove nothing\n" +
			"  -h, --help      help for clean\n",
	}
	for command, want := range tests {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			stdout, err := execute(t, &fakeService{}, command, "--help")
			if err != nil || stdout != want {
				t.Fatalf("%s --help = %q, %v; want %q", command, stdout, err, want)
			}
		})
	}
}

// failingWriter accepts ok writes, then fails every later one.
type failingWriter struct {
	ok  int
	err error
}

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.ok == 0 {
		return 0, w.err
	}
	w.ok--
	return len(p), nil
}

// TestCommandsReturnWriteErrors pins that a failed stdout write is the
// command's error (exit 1), not a silent success. In list only the final
// Flush can fail: the tabwriter buffers every row with a tab until then.
func TestCommandsReturnWriteErrors(t *testing.T) {
	t.Parallel()

	broken := errors.New("broken pipe")
	failing := app.DoctorReport{Header: "caged test (linux/amd64)", Results: []app.Result{{Name: "docker", Status: app.StatusFail, Detail: "missing"}}}
	tests := []struct {
		name    string
		service *fakeService
		args    []string
		ok      int // writes that succeed first
	}{
		{name: "build", service: &fakeService{built: []app.BuiltImage{{Role: "runtime", Ref: "caged-runtime:latest"}}}, args: []string{"build"}},
		{name: "clean with results", service: &fakeService{cleaned: []app.CleanResult{{Ref: "caged-runtime:latest", Outcome: app.Removed}}}, args: []string{"clean"}},
		{name: "clean with nothing to clean", service: &fakeService{}, args: []string{"clean"}},
		{name: "doctor header", service: &fakeService{report: app.DoctorReport{Header: "caged test (linux/amd64)"}}, args: []string{"doctor"}},
		{name: "doctor row, before the failed-checks error", service: &fakeService{report: failing}, args: []string{"doctor"}, ok: 1},
		{name: "list", service: &fakeService{statuses: []app.AgentStatus{{Name: "claude"}}}, args: []string{"list"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewRootCmd("test", agent.Builtin(), tt.service.services())
			cmd.SetArgs(tt.args)
			cmd.SetOut(&failingWriter{ok: tt.ok, err: broken})
			cmd.SetErr(&bytes.Buffer{})
			if err := cmd.Execute(); !errors.Is(err, broken) {
				t.Fatalf("%v error = %v; want the write error", tt.args, err)
			}
		})
	}
}

// TestStdoutTablesEscapeWhatATerminalWouldActOn pins WB-B5 (bugs.md
// Q-WB-6): doctor, list, build and clean print paths, image names and
// docker's messages one per line, and a container can name a link in its
// config folder with control characters, so each line is printable: an
// escape sequence or carriage return cannot rewrite the screen, and a
// newline or tab cannot forge a row or shift a column.
func TestStdoutTablesEscapeWhatATerminalWouldActOn(t *testing.T) {
	t.Parallel()

	const nasty = "~/.claude/x\x1b[2J\ry\nPASS  fake\tz"
	const shown = `~/.claude/x\x1b[2J\ry\nPASS  fake\tz`
	for _, tc := range []struct {
		name    string
		service *fakeService
		args    []string
		want    string
	}{
		{"doctor", &fakeService{report: app.DoctorReport{Header: "caged\x1b]0;t\x07", Results: []app.Result{{Status: app.StatusWarn, Name: "links", Detail: nasty}}}}, []string{"doctor"},
			"caged\\x1b]0;t\\a\nWARN  links      " + shown + "\n"},
		{"list", &fakeService{statuses: []app.AgentStatus{{Name: "claude", Config: []string{nasty}, EnvSet: []string{"A\rB"}}}}, []string{"list"},
			"AGENT   CONFIG                                AUTH ENV SET\nclaude  " + shown + "  A\\rB\n"},
		{"build", &fakeService{built: []app.BuiltImage{{Role: "runtime", Ref: "img\x1b[1A", Summary: nasty}}}, []string{"build"},
			"built runtime image img\\x1b[1A (" + shown + ")\n"},
		{"clean", &fakeService{cleaned: []app.CleanResult{{Ref: "img\r", Outcome: app.Kept, Reason: nasty}}}, []string{"clean"},
			"kept img\\r: " + shown + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stdout, err := execute(t, tc.service, tc.args...)
			if err != nil || stdout != tc.want {
				t.Fatalf("stdout = %q, %v; want %q", stdout, err, tc.want)
			}
		})
	}
}
