package app

import (
	"bytes"
	"strings"
	"testing"
)

type fakeInit struct {
	err     error
	created bool
	calls   int
}

func (f *fakeInit) Ensure() (bool, error) { f.calls++; return f.created, f.err }

type fakeContent struct {
	data      string
	found     bool
	err       error
	sourceErr error
}

func (f *fakeContent) Content() ([]byte, bool, error) {
	if f.err != nil {
		return nil, false, f.err
	}
	return []byte(f.data), f.found, nil
}
func (f *fakeContent) Source() (string, error) {
	if f.sourceErr != nil {
		return "", f.sourceErr
	}
	return "~/.config/caged/config.yaml", nil
}

type fakeApprovals struct {
	data       string
	known      bool
	readErr    error
	writeErr   error
	approvedTo []string
	attempts   int // Approve calls, failed ones included
}

func (f *fakeApprovals) Approved() ([]byte, bool, error) {
	return []byte(f.data), f.known, f.readErr
}

func (f *fakeApprovals) Approve(data []byte) error {
	f.attempts++
	if f.writeErr != nil {
		return f.writeErr
	}
	f.approvedTo = append(f.approvedTo, string(data))
	f.data, f.known = string(data), true
	return nil
}

type guardRig struct {
	init      *fakeInit
	content   *fakeContent
	approvals *fakeApprovals
	out       bytes.Buffer
	answer    string
	tty       bool
}

func (r *guardRig) guard() *ConfigGuard {
	return NewConfigGuard(GuardDeps{
		Init: r.init, Content: r.content, Approvals: r.approvals,
		In: strings.NewReader(r.answer), Out: &r.out, Interactive: r.tty,
	})
}

func newRig(current, approved string, known bool) *guardRig {
	return &guardRig{
		init:      &fakeInit{},
		content:   &fakeContent{data: current, found: true},
		approvals: &fakeApprovals{data: approved, known: known},
		tty:       true,
	}
}

const (
	configV1 = "# note\nimage:\n  apt: [tmux]\n"
	configV2 = "# note\nimage:\n  apt: [tmux, nmap]\nlinks:\n  allow:\n    - path: ~/.kube-copy\n"
)

func TestPrepareDoesNothingWhenTheFileIsUnchanged(t *testing.T) {
	t.Parallel()

	r := newRig(configV1, configV1, true)
	if err := r.guard().prepare(guardOptions{Create: true}); err != nil {
		t.Fatal(err)
	}
	if r.out.Len() != 0 || len(r.approvals.approvedTo) != 0 {
		t.Errorf("output %q, approvals %v; want silence for an unchanged file", r.out.String(), r.approvals.approvedTo)
	}
}

func TestPrepareRecordsTheFirstCopyWithoutAsking(t *testing.T) {
	t.Parallel()

	r := newRig(configV1, "", false)
	r.tty = false
	if err := r.guard().prepare(guardOptions{Create: true}); err != nil {
		t.Fatal(err)
	}
	if len(r.approvals.approvedTo) != 1 || r.approvals.approvedTo[0] != configV1 {
		t.Errorf("approved %v; want the first copy recorded", r.approvals.approvedTo)
	}
	if r.out.Len() != 0 {
		t.Errorf("output %q; want none", r.out.String())
	}
}

func TestPrepareRecordsTheStarterFileItCreated(t *testing.T) {
	t.Parallel()

	r := newRig("# starter\n", "", false)
	r.init.created = true
	if err := r.guard().prepare(guardOptions{Create: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.out.String(), "created starter config ~/.config/caged/config.yaml") {
		t.Errorf("output %q; want a notice that the file was created", r.out.String())
	}
	if len(r.approvals.approvedTo) != 1 {
		t.Errorf("approved %v; want the starter recorded", r.approvals.approvedTo)
	}
}

func TestPrepareWithAMissingFileAndNoHistoryDoesNothing(t *testing.T) {
	t.Parallel()

	r := newRig("", "", false)
	r.content.found = false
	if err := r.guard().prepare(guardOptions{}); err != nil {
		t.Fatal(err)
	}
	if r.init.calls != 0 || len(r.approvals.approvedTo) != 0 {
		t.Errorf("Ensure calls %d, approvals %v; want neither without Create", r.init.calls, r.approvals.approvedTo)
	}
}

func TestPrepareSkipsCreationWhenNotAsked(t *testing.T) {
	t.Parallel()

	r := newRig(configV1, configV1, true)
	if err := r.guard().prepare(guardOptions{Create: false}); err != nil {
		t.Fatal(err)
	}
	if r.init.calls != 0 {
		t.Errorf("Ensure called %d times; want 0 under --no-config", r.init.calls)
	}
}

func TestPrepareShowsTheExactChangeAndAsksBeforeGoingOn(t *testing.T) {
	t.Parallel()

	r := newRig(configV2, configV1, true)
	r.answer = "y\n"
	if err := r.guard().prepare(guardOptions{}); err != nil {
		t.Fatalf("prepare() error = %v; want approval to continue", err)
	}
	out := r.out.String()
	for _, want := range []string{
		"CAGED CONFIG CHANGED: ~/.config/caged/config.yaml",
		"may have edited it",
		"4 lines added, 1 line removed",
		"-  apt: [tmux]\n",
		"+  apt: [tmux, nmap]\n",
		"+    - path: ~/.kube-copy\n",
		"Use this config? [y/N]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if len(r.approvals.approvedTo) != 1 || r.approvals.approvedTo[0] != configV2 {
		t.Errorf("approved %v; want the new content recorded after yes", r.approvals.approvedTo)
	}
}

func TestPrepareRefusesWhenTheUserDeclines(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{"n\n", "\n", "", "maybe\n", "no\n"} {
		r := newRig(configV2, configV1, true)
		r.answer = answer
		err := r.guard().prepare(guardOptions{})
		if err == nil || !strings.Contains(err.Error(), "not approved") {
			t.Errorf("answer %q: prepare() error = %v; want a refusal", answer, err)
		}
		if len(r.approvals.approvedTo) != 0 {
			t.Errorf("answer %q: approvals %v; want the old approval kept", answer, r.approvals.approvedTo)
		}
	}
}

func TestPrepareAcceptsYesInAnyCase(t *testing.T) {
	t.Parallel()

	for _, answer := range []string{"y", "Y\n", "yes\n", " YES \n"} {
		r := newRig(configV2, configV1, true)
		r.answer = answer
		if err := r.guard().prepare(guardOptions{}); err != nil {
			t.Errorf("answer %q: prepare() error = %v; want approval", answer, err)
		}
	}
}

func TestPrepareFailsClosedWithoutATerminal(t *testing.T) {
	t.Parallel()

	r := newRig(configV2, configV1, true)
	r.tty = false
	r.answer = "y\n" // piped input must not count as consent
	err := r.guard().prepare(guardOptions{})
	if err == nil || !strings.Contains(err.Error(), "--trust-config") {
		t.Fatalf("prepare() error = %v; want a refusal that names --trust-config", err)
	}
	if !strings.Contains(r.out.String(), "+  apt: [tmux, nmap]") {
		t.Errorf("output lacks the diff:\n%s", r.out.String())
	}
	if len(r.approvals.approvedTo) != 0 {
		t.Errorf("approvals %v; want none", r.approvals.approvedTo)
	}
}

func TestPrepareTrustApprovesWithoutAsking(t *testing.T) {
	t.Parallel()

	r := newRig(configV2, configV1, true)
	r.tty = false
	if err := r.guard().prepare(guardOptions{Trust: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.out.String(), "+    - path: ~/.kube-copy") || !strings.Contains(r.out.String(), "--trust-config") {
		t.Errorf("output lacks the diff and the trust notice:\n%s", r.out.String())
	}
	if len(r.approvals.approvedTo) != 1 {
		t.Errorf("approvals %v; want the change recorded", r.approvals.approvedTo)
	}
}

func TestPrepareNotifyShowsTheChangeButNeverBlocksOrRecords(t *testing.T) {
	t.Parallel()

	r := newRig(configV2, configV1, true)
	r.tty = false
	r.guard().notify() // has no error to return: it never blocks
	if !strings.Contains(r.out.String(), "Not approved yet") || !strings.Contains(r.out.String(), "+  apt: [tmux, nmap]") {
		t.Errorf("output lacks the pending notice and diff:\n%s", r.out.String())
	}
	if len(r.approvals.approvedTo) != 0 {
		t.Errorf("approvals %v; want none", r.approvals.approvedTo)
	}
}

func TestNotifyRecordsOnlyAFirstCopy(t *testing.T) {
	t.Parallel()

	r := newRig(configV1, "", false)
	r.tty = false
	r.guard().notify()
	if len(r.approvals.approvedTo) != 1 || r.approvals.approvedTo[0] != configV1 || r.out.Len() != 0 {
		t.Errorf("approved %v, output %q; want the first copy recorded silently", r.approvals.approvedTo, r.out.String())
	}
	if r.init.calls != 1 {
		t.Errorf("Ensure calls %d; want 1", r.init.calls)
	}
}

func TestPrepareLabelsCommentOnlyChanges(t *testing.T) {
	t.Parallel()

	r := newRig(configV1+"# another comment\n", configV1, true)
	r.answer = "y\n"
	if err := r.guard().prepare(guardOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.out.String(), "comments and blank lines only; the settings are the same") {
		t.Errorf("output does not say the change is comments only:\n%s", r.out.String())
	}
}

func TestPrepareDoesNotCallATrailingCommentHarmless(t *testing.T) {
	t.Parallel()

	r := newRig("image:\n  apt: [tmux] # x\n", "image:\n  apt: [tmux]\n", true)
	r.answer = "y\n"
	if err := r.guard().prepare(guardOptions{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.out.String(), "comments and blank lines only") {
		t.Errorf("a changed setting line was labelled comment-only:\n%s", r.out.String())
	}
}

func TestPrepareShowsADeletedFileAsRemovedLines(t *testing.T) {
	t.Parallel()

	r := newRig("", configV1, true)
	r.content.found = false
	r.answer = "y\n"
	if err := r.guard().prepare(guardOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.out.String(), "-image:") {
		t.Errorf("output lacks the removed lines:\n%s", r.out.String())
	}
}

// TestPrepareEscapesControlCharactersInTheDiff pins WB-T5: an agent that puts
// terminal escapes or a carriage return into config.yaml cannot hide or
// rewrite the lines of the change it asks the user to approve.
func TestPrepareEscapesControlCharactersInTheDiff(t *testing.T) {
	t.Parallel()
	r := newRig(configV1+"links:\n  allow: [~/.ssh-copy]\x1b[1A\x1b[2K\r# nothing here\n", configV1, true)
	r.tty = false
	if err := r.guard().prepare(guardOptions{}); err == nil {
		t.Fatal("prepare() succeeded without a terminal")
	}
	got := r.out.String()
	if strings.ContainsAny(got, "\x1b\r") || !strings.Contains(got, `+  allow: [~/.ssh-copy]\x1b[1A\x1b[2K\r# nothing here`) {
		t.Fatalf("output holds raw control characters or lacks the escaped line:\n%q", got)
	}
}
