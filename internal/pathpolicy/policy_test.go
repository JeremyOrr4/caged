package pathpolicy

import (
	"strings"
	"testing"
)

const (
	testHome   = "/h"
	testSource = "~/.config/caged/config.yaml"
)

func testConfig(allow, deny []Root) Config {
	return Config{Home: testHome, Source: testSource, Field: "links.allow", Allow: allow, Deny: deny}
}

func TestNewErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		allow []Root
		deny  []Root
		want  string
	}{
		{name: "root", allow: []Root{{Path: "/"}}, want: "too broad"},
		{name: "home", allow: []Root{{Path: "/h"}}, want: "too broad"},
		{name: "inside a denied home", allow: []Root{{Path: "/h/x"}}, deny: []Root{{Path: "/h/x/.."}}, want: "inside the denied path"},
		{name: "inside a denied path", allow: []Root{{Path: "/h/.ssh/keys"}}, deny: []Root{{Path: "/h/.ssh", Builtin: true}}, want: `inside the denied path "~/.ssh"`},
		{name: "relative", allow: []Root{{Path: "rel/path"}}, want: "must be an absolute path"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := New(testConfig(tc.allow, tc.deny))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("New error = %v, want it to contain %q", err, tc.want)
			}
			if !strings.HasPrefix(err.Error(), testSource+": links.allow entry ") {
				t.Fatalf("New error = %q, want it to name the file and field", err)
			}
		})
	}
}

func TestNewAcceptsNarrowRoots(t *testing.T) {
	t.Parallel()
	_, err := New(testConfig([]Root{{Path: "/h/dot"}, {Path: "/srv/dotfiles"}}, []Root{{Path: "/h/.ssh", Builtin: true}}))
	if err != nil {
		t.Fatalf("New returned %v", err)
	}
}

func newCheckPolicy(t *testing.T) *Policy {
	t.Helper()
	policy, err := New(testConfig(
		[]Root{{Path: "/h/dot"}},
		[]Root{{Path: "/h/.ssh", Forms: []string{"/h/.ssh", "/h/dot/ssh/.ssh"}, Builtin: true}},
	))
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func TestCheck(t *testing.T) {
	t.Parallel()
	policy := newCheckPolicy(t)
	tests := []struct {
		name    string
		path    string
		lexical []string
		want    Verdict
	}{
		{name: "under an allow root", path: "/h/dot/pi", want: Allowed},
		{name: "under no allow root", path: "/h/other", want: NotAllowed},
		{name: "inside a deny root", path: "/h/.ssh/id", want: Denied},
		{name: "contains a deny form", path: "/h/dot", want: Denied},
		{name: "lexical name inside a deny root", path: "/h/dot/pi", lexical: []string{"/h/.ssh/x"}, want: Denied},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := policy.Check(tc.path, tc.lexical...).Verdict; got != tc.want {
				t.Fatalf("Check(%q, %q) = %d, want %d", tc.path, tc.lexical, got, tc.want)
			}
		})
	}
}

func TestCheckAllowForms(t *testing.T) {
	t.Parallel()
	policy, err := New(testConfig([]Root{{Path: "/h/link", Forms: []string{"/h/link", "/real/dot"}}}, nil))
	if err != nil {
		t.Fatal(err)
	}
	if got := policy.Check("/real/dot/a").Verdict; got != Allowed {
		t.Fatalf("Check through the resolved form = %d, want Allowed", got)
	}
	if got := policy.Check("/h/link/a").Verdict; got != Allowed {
		t.Fatalf("Check through the written form = %d, want Allowed", got)
	}
	want := []string{"/h/link", "/real/dot"}
	if got := policy.AllowPaths(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("AllowPaths = %q, want %q", got, want)
	}
}

func TestCheckReportsTheDecidingRoot(t *testing.T) {
	t.Parallel()
	policy, err := New(testConfig([]Root{{Path: "/h/dot", ReadOnly: true}}, nil))
	if err != nil {
		t.Fatal(err)
	}
	decision := policy.Check("/h/dot/a")
	if decision.Verdict != Allowed || decision.Root.Path != "/h/dot" || !decision.Root.ReadOnly {
		t.Fatalf("Check = %+v, want Allowed by the read-only root /h/dot", decision)
	}
}

func TestRemedy(t *testing.T) {
	t.Parallel()
	policy, err := New(testConfig(
		[]Root{{Path: "/h/dot"}},
		[]Root{{Path: "/h/.ssh", Builtin: true}, {Path: "/h/secrets"}},
	))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		decision Decision
		target   string
		isDir    bool
		want     string
	}{
		{
			name:     "file suggests its directory",
			decision: Decision{Verdict: NotAllowed},
			target:   "/h/dot2/a.json",
			want:     `add "- path: ~/dot2" under links.allow in ~/.config/caged/config.yaml`,
		},
		{
			name:     "file in home suggests the file",
			decision: Decision{Verdict: NotAllowed},
			target:   "/h/a.json",
			want:     `add "- path: ~/a.json" under links.allow in ~/.config/caged/config.yaml`,
		},
		{
			name:     "directory suggests itself",
			decision: Decision{Verdict: NotAllowed},
			target:   "/h/dot2/pi",
			isDir:    true,
			want:     `add "- path: ~/dot2/pi" under links.allow in ~/.config/caged/config.yaml`,
		},
		{
			name:     "built-in deny",
			decision: policy.Check("/h/.ssh/id"),
			target:   "/h/.ssh/id",
			want:     "~/.ssh is on caged's built-in deny list and cannot be allowed",
		},
		{
			name:     "configured deny",
			decision: policy.Check("/h/secrets/a"),
			target:   "/h/secrets/a",
			want:     "~/secrets is denied by links.deny in ~/.config/caged/config.yaml",
		},
		{
			name:     "allowed needs no remedy",
			decision: policy.Check("/h/dot/a"),
			target:   "/h/dot/a",
			want:     "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := policy.Remedy(tc.decision, tc.target, tc.isDir); got != tc.want {
				t.Fatalf("Remedy = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTilde(t *testing.T) {
	t.Parallel()
	tests := []struct{ home, path, want string }{
		{"/h", "/h", "~"},
		{"/h", "/h/a/b", "~/a/b"},
		{"/h", "/h2/a", "/h2/a"},
		{"/h", "/x", "/x"},
		{"/h/", "/h/a", "~/a"},
	}
	for _, tc := range tests {
		if got := Tilde(tc.home, tc.path); got != tc.want {
			t.Errorf("Tilde(%q, %q) = %q, want %q", tc.home, tc.path, got, tc.want)
		}
	}
}

func TestWithin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path, root string
		want       bool
	}{
		{"/h/a", "/h", true},
		{"/h", "/h", true},
		{"/h2", "/h", false},
		{"/", "/h", false},
		{"/h/a/../b", "/h", true},
		{"/h/../x", "/h", false},
		{"/", "/", true},
	}
	for _, tc := range tests {
		if got := Within(tc.path, tc.root); got != tc.want {
			t.Errorf("Within(%q, %q) = %v, want %v", tc.path, tc.root, got, tc.want)
		}
	}
}

func TestBuiltinDeny(t *testing.T) {
	t.Parallel()
	paths := BuiltinDeny("/h")
	for _, want := range []string{"/h/.ssh", "/h/.config/caged", "/h/.config/gcloud", "/etc", "/run", "/var/run"} {
		found := false
		for _, path := range paths {
			found = found || path == want
		}
		if !found {
			t.Errorf("BuiltinDeny(/h) = %q, missing %q", paths, want)
		}
	}
}
