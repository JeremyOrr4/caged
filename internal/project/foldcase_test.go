package project

import (
	"strings"
	"testing"
)

// TestFoldCaseRefusesHomeInOtherLetters pins the W7-T8 review: on macOS,
// `cd /users/dev && caged` keeps the letters PWD has, so the home directory
// did not equal /Users/dev and was mounted without --allow-broad-mount.
func TestFoldCaseRefusesHomeInOtherLetters(t *testing.T) {
	t.Parallel()
	fsys := fakeFS{
		entries:  map[string]bool{"/users/dev": true},
		resolved: map[string]string{"/users/dev": "/users/dev", "/Users/dev": "/Users/dev"}, // EvalSymlinks keeps the letters
	}
	_, err := NewResolver(fsys, nil, true).Resolve("/users/dev", "/Users/dev", false)
	if err == nil || !strings.Contains(err.Error(), "entire home directory") {
		t.Fatalf("folded Resolve(/users/dev) error = %v; want the home directory refused", err)
	}
	if _, err := NewResolver(fsys, nil, false).Resolve("/users/dev", "/Users/dev", false); err != nil {
		t.Fatalf("exact Resolve(/users/dev) error = %v; want another folder mounted", err)
	}
	if _, err := NewResolver(fsys, nil, true).Resolve("/users/dev", "/Users/dev", true); err != nil {
		t.Fatalf("folded Resolve(/users/dev) with --allow-broad-mount error = %v; want it mounted", err)
	}
}

// TestFoldCaseValidatesWorkdirsAllowInAnyCase: workdirs.allow naming the home
// directory in other letters is too broad on macOS, as the exact name is.
func TestFoldCaseValidatesWorkdirsAllowInAnyCase(t *testing.T) {
	t.Parallel()
	fsys := fakeFS{
		entries:  map[string]bool{"/USERS/DEV/app": true},
		resolved: map[string]string{"/USERS/DEV/app": "/USERS/DEV/app", "/Users/dev": "/Users/dev"},
	}
	policies := &fakePolicies{policy: workdirPolicy(t, "/Users/dev", "/USERS/DEV")}
	if _, err := NewResolver(fsys, policies, false).Resolve("/USERS/DEV/app", "/Users/dev", false); err != nil {
		t.Fatalf("exact Resolve error = %v; want /USERS/DEV allowed where names keep case", err)
	}
	_, err := NewResolver(fsys, policies, true).Resolve("/USERS/DEV/app", "/Users/dev", false)
	if err == nil || !strings.Contains(err.Error(), `workdirs.allow entry "/USERS/DEV" is too broad`) {
		t.Fatalf("folded Resolve error = %v; want workdirs.allow /USERS/DEV refused as too broad", err)
	}
}
