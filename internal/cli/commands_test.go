package cli

import (
	"errors"
	"reflect"
	"strings"
	"testing"

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
