package pathpolicy

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// FuzzWithin checks Within against a rule written independently of it: on
// absolute paths it is the plain prefix rule (the rule hostconfig's
// withinSlash uses for container paths), and a relative path and an absolute
// one are never inside each other.
func FuzzWithin(f *testing.F) {
	for _, seed := range [][2]string{
		{"/h/a", "/h"}, {"/h", "/h"}, {"/h2", "/h"}, {"/", "/h"}, {"/h/a/../b", "/h"}, {"/h/../x", "/h"},
		{"/", "/"}, {"/h/a", "/h/"}, {"/hh", "/h"}, {"/h/..hidden", "/h"}, {"rel/a", "/h"}, {"/h/a", "rel"},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, p, r string) {
		if filepath.IsAbs(p) != filepath.IsAbs(r) && Within(p, r) {
			t.Fatalf("Within(%q, %q) = true across a relative and an absolute path", p, r)
		}
		path, root := filepath.Clean("/"+p), filepath.Clean("/"+r)
		want := path == root || root == "/" || strings.HasPrefix(path, root+"/")
		if got := Within("/"+p, "/"+r); got != want {
			t.Fatalf("Within(%q, %q) = %v, want %v (prefix rule)", "/"+p, "/"+r, got, want)
		}
	})
}

// FuzzPolicyCheck checks Check, with and without a lexical name, against the
// rule it implements, on the policy of TestCheck (allow /h/dot; built-in deny
// /h/.ssh, which is also reachable as /h/dot/ssh/.ssh):
// Denied when any candidate is inside a deny form or contains one, else
// Allowed when the resolved path is inside the allow root, else NotAllowed.
func FuzzPolicyCheck(f *testing.F) {
	policy := newCheckPolicy(f)
	withoutDeny := policy.Without("/h/.ssh")
	denyForms := []string{"/h/.ssh", "/h/dot/ssh/.ssh"}
	for _, seed := range [][2]string{
		{"/h/dot/pi", ""}, {"/h/other", ""}, {"/h/.ssh/id", ""}, {"/h/dot", ""}, {"/h/dot/pi", "/h/.ssh/x"},
		{"/h/dot/ssh/.ssh/k", ""}, {"/", ""}, {"rel", "/h/dot"}, {"/h/dot/../.ssh", ""}, {"/h/dot/pi", "/h"},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, path, lexical string) {
		want := func(candidates ...string) Verdict {
			for _, candidate := range candidates {
				for _, form := range denyForms {
					if Within(candidate, form) || Within(form, candidate) {
						return Denied
					}
				}
			}
			if Within(path, "/h/dot") {
				return Allowed
			}
			return NotAllowed
		}
		if got := policy.Check(path).Verdict; got != want(path) {
			t.Fatalf("Check(%q) = %v, want %v", path, got, want(path))
		}
		if got := policy.Check(path, lexical).Verdict; got != want(path, lexical) {
			t.Fatalf("Check(%q, %q) = %v, want %v", path, lexical, got, want(path, lexical))
		}
		if got, plain := withoutDeny.Check(path).Verdict, Within(path, "/h/dot"); got != map[bool]Verdict{true: Allowed, false: NotAllowed}[plain] {
			t.Fatalf("Without(/h/.ssh).Check(%q) = %v; the only deny root is set aside", path, got)
		}
	})
}

// FuzzNew checks that New accepts an allow root exactly when each of its forms
// is absolute, does not contain the home directory, and is not inside the deny
// root; that a refusal names the file and the field; and that the accepted
// forms start with the cleaned path and hold no duplicate.
func FuzzNew(f *testing.F) {
	for _, seed := range [][2]string{
		{"/h/dot", "/h/dot"}, {"/srv/dotfiles", "/h/dot/"}, {"~/dot", "/h/dot"}, {"dot", "/srv/x"}, {"", ""},
		{"/", "/h/dot"}, {"/h", "/h/dot"}, {"/h/.ssh/x", "/srv/x"}, {"/h/../h/dot", "/srv/a"}, {"/srv/a", "/h"},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, allow, form string) {
		forms := []string{filepath.Clean(allow), filepath.Clean(form)}
		valid := true
		for _, candidate := range forms {
			valid = valid && filepath.IsAbs(candidate) && !Within(testHome, candidate) && !Within(candidate, "/h/.ssh")
		}
		policy, err := New(testConfig([]Root{{Path: allow, Forms: []string{form}}}, []Root{{Path: "/h/.ssh", Builtin: true}}))
		if (err == nil) != valid {
			t.Fatalf("New(allow %q, form %q) error = %v, want valid = %v", allow, form, err, valid)
		}
		if err != nil {
			if want := testSource + ": links.allow entry "; !strings.HasPrefix(err.Error(), want) {
				t.Fatalf("New(%q) error %q does not start with %q", allow, err, want)
			}
			return
		}
		got := policy.AllowPaths()
		if got[0] != forms[0] || len(slices.Compact(slices.Clone(got))) != len(got) || len(got) > 2 {
			t.Fatalf("New(allow %q, form %q).AllowPaths() = %q", allow, form, got)
		}
	})
}

// FuzzTilde checks that Tilde shows the home directory as "~", a path under it
// as "~/<rest>" that joins back to the path, and any other path unchanged but
// cleaned. A home of "/" abbreviates only itself.
func FuzzTilde(f *testing.F) {
	for _, seed := range [][2]string{{"/h", "/h"}, {"/h", "/h/a/b"}, {"/h", "/h2/a"}, {"/h/", "/h/a"}, {"/", "/a"}, {"/h", "/h/../h/a"}} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, h, p string) {
		home, path := filepath.Clean("/"+h), filepath.Clean("/"+p)
		under := home != "/" && strings.HasPrefix(path, home+"/")
		got := Tilde("/"+h, "/"+p)
		rest, ok := strings.CutPrefix(got, "~/")
		switch {
		case path == home:
			if got != "~" {
				t.Fatalf("Tilde(%q, the home itself) = %q", home, got)
			}
		case under:
			if !ok || filepath.Join(home, rest) != path {
				t.Fatalf("Tilde(%q, %q) = %q, want ~/ and the rest", home, path, got)
			}
		case got != path:
			t.Fatalf("Tilde(%q, %q) = %q, want the cleaned path", home, path, got)
		}
	})
}
