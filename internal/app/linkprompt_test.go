package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

// steppingConfigs returns its reports in turn, the last one again once they
// run out: the report before and after an allow entry was added.
type steppingConfigs struct {
	reports []plan.ConfigReport
	calls   int
}

func (s *steppingConfigs) Resolve(string) (plan.ConfigReport, error) {
	report := s.reports[min(s.calls, len(s.reports)-1)]
	s.calls++
	return report, nil
}

type allowCall struct {
	path     string
	readOnly bool
}

type fakeAllower struct {
	calls []allowCall
	err   error
}

func (f *fakeAllower) AllowLink(path string, readOnly bool) ([]byte, error) {
	f.calls = append(f.calls, allowCall{path, readOnly})
	if f.err != nil {
		return nil, f.err
	}
	return []byte("links:\n  allow:\n    - path: \"" + path + "\"\n"), nil
}

// piRootRefused is the owner's layout: ~/.pi is a Stow link into dotfiles
// that links.allow does not cover.
var piRootRefused = plan.Refusal{
	Path: "/home/me/.pi", Reason: "symlink target ~/dotfiles/pi/.pi is not under links.allow",
	Hint: `add "- path: ~/dotfiles/pi/.pi" under links.allow in ~/.config/caged/config.yaml`,
	Warn: true, Agent: "pi", Allow: "/home/me/dotfiles/pi/.pi",
}

var piMounted = plan.ConfigMount{Agent: "pi", Name: ".pi", HostPath: "/home/me/dotfiles/pi/.pi", DestPath: "/home/caged/.pi", Root: "/home/me/.pi", Writable: true, Link: "/home/me/.pi"}

type linkRig struct {
	configs   *steppingConfigs
	allower   *fakeAllower
	approvals *fakeApprovals
	stderr    strings.Builder
}

func (r *linkRig) plan(t *testing.T, request Request, answers string, interactive bool) plan.RunSpec {
	t.Helper()
	env := &testkit.Env{Home: "/home/me"}
	prompter := NewPrompter(strings.NewReader(answers), &r.stderr, interactive)
	planner := NewPlanner(env, &fakeProject{}, r.configs).WithLinkPrompt(LinkPrompt{Prompter: prompter, Allower: r.allower, Approvals: r.approvals})
	request.NoMount = true
	spec, err := planner.Plan(request, "img", NewReporter(&r.stderr, false))
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func newLinkRig(reports ...plan.ConfigReport) *linkRig {
	return &linkRig{configs: &steppingConfigs{reports: reports}, allower: &fakeAllower{}, approvals: &fakeApprovals{}}
}

func TestYesAddsTheAllowEntryApprovesItAndMountsTheConfig(t *testing.T) {
	t.Parallel()
	rig := newLinkRig(plan.ConfigReport{Refused: []plan.Refusal{piRootRefused}}, plan.ConfigReport{Mounts: []plan.ConfigMount{piMounted}})

	spec := rig.plan(t, Request{Agent: lookup(t, "pi")}, "y\n", true)

	if len(rig.allower.calls) != 1 || rig.allower.calls[0] != (allowCall{"/home/me/dotfiles/pi/.pi", false}) {
		t.Fatalf("AllowLink calls = %+v", rig.allower.calls)
	}
	if len(rig.approvals.approvedTo) != 1 || !strings.Contains(rig.approvals.approvedTo[0], "/home/me/dotfiles/pi/.pi") {
		t.Fatalf("approved = %q; want the edited file", rig.approvals.approvedTo)
	}
	if rig.configs.calls != 2 || len(spec.ConfigMounts) != 1 || spec.ConfigMounts[0].DestPath != "/home/caged/.pi" {
		t.Fatalf("resolves = %d, mounts = %+v; want ~/.pi mounted after a second resolve", rig.configs.calls, spec.ConfigMounts)
	}
	out := rig.stderr.String()
	for _, want := range []string{
		"caged: pi config ~/.pi is NOT mounted (symlink target ~/dotfiles/pi/.pi is not under links.allow), so pi runs without it",
		"Add ~/dotfiles/pi/.pi to links.allow (read/write) so it is mounted? [y/N] ",
		"caged: added ~/dotfiles/pi/.pi to links.allow.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stderr lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "WARNING: pi config") {
		t.Errorf("a fixed root is still reported:\n%s", out)
	}
}

func TestNoOrNobodyToAskLeavesTheConfigAloneAndWarns(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		request     Request
		answers     string
		interactive bool
		asked       bool
	}{
		{name: "no", answers: "n\n", interactive: true, asked: true},
		{name: "no input", answers: "", interactive: true, asked: true},
		{name: "not a terminal", answers: "y\n", interactive: false},
		{name: "trust-config does not widen the policy", request: Request{TrustConfig: true}, answers: "y\n", interactive: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rig := newLinkRig(plan.ConfigReport{Refused: []plan.Refusal{piRootRefused}})
			request := tt.request
			request.Agent = lookup(t, "pi")

			rig.plan(t, request, tt.answers, tt.interactive)

			if len(rig.allower.calls) != 0 || len(rig.approvals.approvedTo) != 0 || rig.configs.calls != 1 {
				t.Fatalf("allow calls = %+v, approved = %q, resolves = %d; want nothing changed", rig.allower.calls, rig.approvals.approvedTo, rig.configs.calls)
			}
			out := rig.stderr.String()
			if asked := strings.Contains(out, "[y/N]"); asked != tt.asked {
				t.Fatalf("asked = %v, want %v:\n%s", asked, tt.asked, out)
			}
			const warning = `WARNING: pi config ~/.pi is NOT mounted (symlink target ~/dotfiles/pi/.pi is not under links.allow), so pi runs without it and anything pi changes there is lost when it exits. Fix: add "- path: ~/dotfiles/pi/.pi" under links.allow in ~/.config/caged/config.yaml, or answer y when caged asks`
			if !strings.Contains(out, warning) {
				t.Fatalf("stderr lacks the warning:\n%s", out)
			}
		})
	}
}

func TestOnlyTheSelectedAgentsRootsAreAskedAbout(t *testing.T) {
	t.Parallel()
	claudeRoot := plan.Refusal{Path: "/home/me/.claude", Reason: "symlink target ~/dotfiles/claude is not under links.allow", Hint: "h", Warn: true, Agent: "claude", Allow: "/home/me/dotfiles/claude"}
	shared := plan.Refusal{Path: "/home/me/.gitconfig", Reason: "symlink target ~/dotfiles/git/.gitconfig is not under links.allow", Hint: "h", Warn: true, Agent: plan.SharedOwner, Allow: "/home/me/dotfiles/git", AllowReadOnly: true}
	rig := newLinkRig(plan.ConfigReport{Refused: []plan.Refusal{claudeRoot, piRootRefused, shared, piRootRefused}})

	rig.plan(t, Request{Agent: lookup(t, "pi")}, "n\ny\n", true)

	want := []allowCall{{"/home/me/dotfiles/git", true}}
	if len(rig.allower.calls) != 1 || rig.allower.calls[0] != want[0] {
		t.Fatalf("AllowLink calls = %+v; want %+v (pi asked once and declined, shared asked read-only, claude not asked)", rig.allower.calls, want)
	}
	out := rig.stderr.String()
	if strings.Count(out, "[y/N]") != 2 || strings.Contains(out, "claude") {
		t.Fatalf("stderr:\n%s\nwant two questions and nothing about claude without --verbose", out)
	}
	if !strings.Contains(out, "Add ~/dotfiles/git to links.allow (read-only)") {
		t.Fatalf("stderr lacks the read-only question:\n%s", out)
	}
}

func TestAFailedEditIsReportedAndTheRunGoesOn(t *testing.T) {
	t.Parallel()
	rig := newLinkRig(plan.ConfigReport{Refused: []plan.Refusal{piRootRefused}})
	rig.allower.err = errors.New("~/.config/caged/config.yaml changed after caged reviewed it")

	rig.plan(t, Request{Agent: lookup(t, "pi")}, "y\n", true)

	out := rig.stderr.String()
	if !strings.Contains(out, "caged: links.allow not changed: ~/.config/caged/config.yaml changed after caged reviewed it") || !strings.Contains(out, "WARNING: pi config ~/.pi is NOT mounted") {
		t.Fatalf("stderr:\n%s", out)
	}
	if len(rig.approvals.approvedTo) != 0 || rig.configs.calls != 1 {
		t.Fatalf("approved = %q, resolves = %d; want nothing approved and no second resolve", rig.approvals.approvedTo, rig.configs.calls)
	}
}

func TestStagedConfigOfTheSelectedAgentIsNoted(t *testing.T) {
	t.Parallel()
	staged := plan.ConfigMount{Agent: "pi", Name: ".pi", HostPath: "/home/me/.pi", DestPath: "/home/caged/.pi", Root: "/home/me/.pi", Writable: true, Staged: true}
	nested := staged
	nested.HostPath, nested.DestPath, nested.Link = "/home/me/dotfiles/themes", "/home/caged/.pi/themes", "/home/me/.pi/themes"
	rig := newLinkRig(plan.ConfigReport{Mounts: []plan.ConfigMount{staged, nested}})

	rig.plan(t, Request{Agent: lookup(t, "pi")}, "", true)
	if out := rig.stderr.String(); strings.Count(out, "NOTE: pi config ~/.pi runs from a copy") != 1 {
		t.Fatalf("stderr:\n%s\nwant one staged note", out)
	}

	rig = newLinkRig(plan.ConfigReport{Mounts: []plan.ConfigMount{staged}})
	rig.plan(t, Request{Agent: lookup(t, "claude")}, "", true)
	if out := rig.stderr.String(); strings.Contains(out, "NOTE") {
		t.Fatalf("another agent's staged config was noted:\n%s", out)
	}
}

func TestPrompterSharesOneReaderAcrossQuestions(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	prompter := NewPrompter(strings.NewReader("y\nn\nyes\n"), &out, true)
	got := []bool{prompter.Confirm("one?"), prompter.Confirm("two?"), prompter.Confirm("three\x1b[2J?")}
	if got[0] != true || got[1] != false || got[2] != true {
		t.Fatalf("answers = %v; want typed-ahead answers kept", got)
	}
	if !strings.Contains(out.String(), `three\x1b[2J? [y/N] `) {
		t.Fatalf("question not made printable: %q", out.String())
	}
}
