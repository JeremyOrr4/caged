package app

import "testing"

// These tests pin what the config review prints, byte for byte: the banner, the
// two explanation lines, the summary, the diff, and the question.

const (
	reportRule = "================================================================\n"
	reportHead = "\n" + reportRule +
		" CAGED CONFIG CHANGED: ~/.config/caged/config.yaml\n" +
		reportRule +
		"This file decides what caged installs and which host paths it mounts. It is\n" +
		"mounted read/write into containers, so an agent may have edited it.\n"
	reportV1toV2 = "Since you last approved it: 4 lines added, 1 line removed.\n" +
		"\n" +
		"--- approved\n" +
		"+++ current\n" +
		"@@ -1,3 +1,6 @@\n" +
		" # note\n" +
		" image:\n" +
		"-  apt: [tmux]\n" +
		"+  apt: [tmux, nmap]\n" +
		"+links:\n" +
		"+  allow:\n" +
		"+    - path: ~/.kube-copy\n"
)

func TestPrepareTranscript(t *testing.T) {
	t.Parallel()

	r := newRig(configV2, configV1, true)
	r.answer = "y\n"
	if err := r.guard().prepare(guardOptions{}); err != nil {
		t.Fatal(err)
	}
	want := reportHead + reportV1toV2 + reportRule + "Use this config? [y/N] "
	if got := r.out.String(); got != want {
		t.Errorf("output =\n%q\nwant\n%q", got, want)
	}
}

func TestNotifyTranscript(t *testing.T) {
	t.Parallel()

	r := newRig(configV2, configV1, true)
	r.tty = false
	r.guard().notify()
	want := reportHead + reportV1toV2 +
		"\n" +
		"Not approved yet. caged will ask before it runs with this change.\n" +
		reportRule
	if got := r.out.String(); got != want {
		t.Errorf("output =\n%q\nwant\n%q", got, want)
	}
}

func TestTrustTranscriptForACommentOnlyChange(t *testing.T) {
	t.Parallel()

	r := newRig(configV1+"# another comment\n", configV1, true)
	r.tty = false
	if err := r.guard().prepare(guardOptions{Trust: true}); err != nil {
		t.Fatal(err)
	}
	want := reportHead +
		"Since you last approved it: 1 line added, 0 lines removed (comments and blank lines only; the settings are the same).\n" +
		"\n" +
		"--- approved\n" +
		"+++ current\n" +
		"@@ -1,3 +1,4 @@\n" +
		" # note\n" +
		" image:\n" +
		"   apt: [tmux]\n" +
		"+# another comment\n" +
		reportRule +
		"Approved with --trust-config.\n"
	if got := r.out.String(); got != want {
		t.Errorf("output =\n%q\nwant\n%q", got, want)
	}
}
