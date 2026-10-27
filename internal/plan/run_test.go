package plan

import "testing"

func TestRefusalString(t *testing.T) {
	t.Parallel()

	if got := (Refusal{Path: "~/.claude/key", Reason: "symlink target ~/.ssh/id is denied"}).String(); got != "~/.claude/key (symlink target ~/.ssh/id is denied)" {
		t.Errorf("without a hint: %q", got)
	}
	if got := (Refusal{Path: "p", Reason: "r", Hint: "h"}).String(); got != "p (r); h" {
		t.Errorf("with a hint: %q", got)
	}
}
