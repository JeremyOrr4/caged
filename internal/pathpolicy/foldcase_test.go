package pathpolicy

import "testing"

// mustFold is policy.FoldCase, failing the test on an error.
func mustFold(t *testing.T, policy *Policy) *Policy {
	t.Helper()
	folded, err := policy.FoldCase()
	if err != nil {
		t.Fatal(err)
	}
	return folded
}

// TestFoldCaseMatchesDenyRootsInAnyCase pins W7-T8 (bugs.md Q-WB-7): on a
// file system that ignores case, ~/.SSH is ~/.ssh, so a folded policy denies
// it; an exact policy, for a file system that keeps case, does not.
func TestFoldCaseMatchesDenyRootsInAnyCase(t *testing.T) {
	t.Parallel()
	exact := newCheckPolicy(t)
	folded := mustFold(t, exact)
	for _, tc := range []struct {
		path          string
		exact, folded Verdict
	}{
		{"/h/.SSH/id", NotAllowed, Denied},
		{"/h/.Ssh", NotAllowed, Denied},
		{"/H", NotAllowed, Denied},                  // holds the deny root ~/.ssh in other letters
		{"/h/DOT/SSH/.SSH/key", NotAllowed, Denied}, // the deny root's other form
		{"/h/.ssh/id", Denied, Denied},              // exact names are denied either way
		{"/h/Dot/notes", NotAllowed, NotAllowed},    // allow roots still match exactly
		{"/h/dot/notes", Allowed, Allowed},
	} {
		if got := exact.Check(tc.path).Verdict; got != tc.exact {
			t.Errorf("exact Check(%q) = %v, want %v", tc.path, got, tc.exact)
		}
		if got := folded.Check(tc.path).Verdict; got != tc.folded {
			t.Errorf("folded Check(%q) = %v, want %v", tc.path, got, tc.folded)
		}
	}
	if d := folded.Check("/h/.SSH/id"); d.Root.Path != "/h/.ssh" || folded.Remedy(d, "/h/.SSH/id", false) != "~/.ssh is on caged's built-in deny list and cannot be allowed" {
		t.Errorf("folded decision = %+v, remedy %q; want ~/.ssh named", d, folded.Remedy(d, "/h/.SSH/id", false))
	}
}

// TestFoldCaseKeepsTheOriginalAndSurvivesWithout: FoldCase returns a copy,
// and Without keeps the copy's case rule for the deny roots that remain.
func TestFoldCaseKeepsTheOriginalAndSurvivesWithout(t *testing.T) {
	t.Parallel()
	policy, err := New(testConfig(nil, []Root{{Path: "/h/.ssh", Builtin: true}, {Path: "/h/.gnupg", Builtin: true}}))
	if err != nil {
		t.Fatal(err)
	}
	folded := mustFold(t, policy)
	if got := policy.Check("/h/.GNUPG").Verdict; got != NotAllowed {
		t.Errorf("original Check(/h/.GNUPG) = %v after FoldCase, want NotAllowed", got)
	}
	without := folded.Without("/h/.ssh")
	if got := without.Check("/h/.SSH").Verdict; got != NotAllowed {
		t.Errorf("Without(~/.ssh).Check(/h/.SSH) = %v, want NotAllowed", got)
	}
	if got := without.Check("/h/.GNUPG").Verdict; got != Denied {
		t.Errorf("Without(~/.ssh).Check(/h/.GNUPG) = %v, want Denied: the copy keeps folding", got)
	}
}

// TestOverlapComparesNamesAsThePolicyDoes: Overlap is containment either way
// round, in any case only for a folded policy.
func TestOverlapComparesNamesAsThePolicyDoes(t *testing.T) {
	t.Parallel()
	exact := newCheckPolicy(t)
	folded := mustFold(t, exact)
	for _, tc := range []struct {
		path, other   string
		exact, folded bool
	}{
		{"/x/state/caged", "/x/state/caged", true, true},
		{"/x/state", "/x/state/caged", true, true},
		{"/x/state/caged/a", "/x/state/caged", true, true},
		{"/x/STATE/caged", "/x/state/caged", false, true},
		{"/x/config", "/X/CONFIG/caged", false, true},
		{"/x/state2", "/x/state", false, false},
		{"/x/other", "/x/state", false, false},
	} {
		if got := exact.Overlap(tc.path, tc.other); got != tc.exact {
			t.Errorf("exact Overlap(%q, %q) = %v, want %v", tc.path, tc.other, got, tc.exact)
		}
		if got := folded.Overlap(tc.path, tc.other); got != tc.folded {
			t.Errorf("folded Overlap(%q, %q) = %v, want %v", tc.path, tc.other, got, tc.folded)
		}
	}
}

// TestFoldCaseRemedySuggestsWhatNewWouldAccept: a refusal's suggestion never
// names the home directory, nor a folder inside a deny root, in any case when
// the policy folds case.
func TestFoldCaseRemedySuggestsWhatNewWouldAccept(t *testing.T) {
	t.Parallel()
	exact := newCheckPolicy(t)
	notAllowed := Decision{Verdict: NotAllowed}
	for _, tc := range []struct {
		target        string
		isDir         bool
		exact, folded string // the suggested allow root
	}{
		{"/H/notes.md", false, "/H", "/H/notes.md"},        // the folder is home in other letters
		{"/h/.SSH/keys", true, "~/.SSH/keys", "~/.SSH"},    // the folder is inside ~/.ssh in other letters
		{"/h/other/notes.md", false, "~/other", "~/other"}, // nothing to fold
	} {
		suggest := func(path string) string { return `add "- path: ` + path + `" under links.allow in ` + testSource }
		if got := exact.Remedy(notAllowed, tc.target, tc.isDir); got != suggest(tc.exact) {
			t.Errorf("exact Remedy(%q) = %q, want %q", tc.target, got, suggest(tc.exact))
		}
		if got := mustFold(t, exact).Remedy(notAllowed, tc.target, tc.isDir); got != suggest(tc.folded) {
			t.Errorf("folded Remedy(%q) = %q, want %q", tc.target, got, suggest(tc.folded))
		}
	}
}

// TestFoldNameUsesSimpleCaseFolding pins the W7-T8 review: lowercasing is
// not how a case-insensitive file system compares names. The long s (U+017F)
// folds to s, as do the other members of each unicode.SimpleFold orbit.
func TestFoldNameUsesSimpleCaseFolding(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ a, b string }{
		{"/h/.ssh", "/H/.SSH"},
		{"/h/.local/state/caged", "/h/.local/ſtate/caged"}, // long s
		{"/h/Library/Keychains", "/h/Library/Keychainſ"},
		{"/h/.kube", "/h/.Kube"}, // Kelvin sign
	} {
		if FoldName(tc.a) != FoldName(tc.b) {
			t.Errorf("FoldName(%q) = %q, FoldName(%q) = %q; want one name", tc.a, FoldName(tc.a), tc.b, FoldName(tc.b))
		}
	}
	if FoldName("/h/.ssh") == FoldName("/h/.ssx") || FoldName("/a/b") != "/A/B" {
		t.Errorf("FoldName joins different names or changes separators: %q", FoldName("/a/b"))
	}
}

// TestFoldCaseDeniesALongSSpelling: the reviewer's attack, a link into
// ~/.local/ſtate/caged under links.allow ~/.local, is denied once folded.
func TestFoldCaseDeniesALongSSpelling(t *testing.T) {
	t.Parallel()
	policy, err := New(testConfig([]Root{{Path: "/h/.local"}}, []Root{{Path: "/h/.local/state/caged", Builtin: true}}))
	if err != nil {
		t.Fatal(err)
	}
	folded, err := policy.FoldCase()
	if err != nil {
		t.Fatal(err)
	}
	if got := folded.Check("/h/.local/ſtate/caged/config-0.approved.yaml").Verdict; got != Denied {
		t.Errorf("folded Check(~/.local/ſtate/caged/...) = %v, want Denied", got)
	}
}

// TestFoldCaseValidatesAllowRootsInAnyCase pins the W7-T8 review: New judged
// allow roots by exact names, so on macOS the home directory in other letters
// passed as an allow root. FoldCase validates again, under the fold.
func TestFoldCaseValidatesAllowRootsInAnyCase(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		allow string
		want  string
	}{
		{"/H", `links.allow entry "/H" is too broad: it contains the home directory`},
		{"/h/.SSH/keys", `links.allow entry "~/.SSH/keys" is inside the denied path "~/.ssh"`},
	} {
		policy, err := New(testConfig([]Root{{Path: tc.allow}}, []Root{{Path: "/h/.ssh", Builtin: true}}))
		if err != nil {
			t.Fatalf("New(allow %s) = %v; want it accepted where names keep case", tc.allow, err)
		}
		if _, err := policy.FoldCase(); err == nil || err.Error() != testSource+": "+tc.want {
			t.Errorf("FoldCase() error = %v; want %q", err, testSource+": "+tc.want)
		}
	}
}
