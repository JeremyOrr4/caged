package userconfig

import (
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

const editPath = "/home/me/.config/caged/config.yaml"

// reviewedStore returns a store whose config.yaml holds content (none when
// content is nil), read once as the config guard reads it before a run.
func reviewedStore(t *testing.T, content *string) (*Store, *fakeFS) {
	t.Helper()
	fsys := &fakeFS{}
	if content != nil {
		fsys.put(editPath, []byte(*content))
	}
	store := NewStore(fsys, testkit.Env{Home: "/home/me"})
	if _, _, err := store.Content(); err != nil {
		t.Fatal(err)
	}
	return store, fsys
}

func ptr(s string) *string { return &s }

func TestAllowLinkInsertsTheEntryAndKeepsTheRestOfTheFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		content  *string
		path     string
		readOnly bool
		want     string
	}{
		{
			name:    "starter template",
			content: ptr(starterConfig),
			path:    "/home/me/git/dotfiles/pi/.pi",
			want:    starterConfig + "\n## Added by caged when you allowed this link target.\nlinks:\n  allow:\n    - path: \"~/git/dotfiles/pi/.pi\"\n",
		},
		{
			name: "no file",
			path: "/home/me/d",
			want: "## Added by caged when you allowed this link target.\nlinks:\n  allow:\n    - path: \"~/d\"\n",
		},
		{
			name:    "other keys, no final newline",
			content: ptr("image:\n  apt:\n    - jq"),
			path:    "/home/me/d",
			want:    "image:\n  apt:\n    - jq\n\n## Added by caged when you allowed this link target.\nlinks:\n  allow:\n    - path: \"~/d\"\n",
		},
		{
			name:    "links with only deny",
			content: ptr("links:\n  # keep me\n  deny:\n    - ~/secrets\n"),
			path:    "/home/me/d",
			want:    "links:\n  # keep me\n  allow:\n    - path: \"~/d\"\n  deny:\n    - ~/secrets\n",
		},
		{
			name:     "existing allow list",
			content:  ptr("links:\n    allow:\n        # mine\n        -   path: ~/a\n            read_only: true\n"),
			path:     "/home/me/d",
			readOnly: true,
			want:     "links:\n    allow:\n        # mine\n        - path: \"~/d\"\n          read_only: true\n        -   path: ~/a\n            read_only: true\n",
		},
		{
			name:    "bare links",
			content: ptr("version: 1\nlinks:\n# trailing\n"),
			path:    "/home/me/d",
			want:    "version: 1\nlinks:\n  allow:\n    - path: \"~/d\"\n# trailing\n",
		},
		{
			name:    "bare allow",
			content: ptr("links:\n  allow:\n  deny: [~/x]\n"),
			path:    "/home/me/d",
			want:    "links:\n  allow:\n    - path: \"~/d\"\n  deny: [~/x]\n",
		},
		{
			name:    "path outside home",
			content: ptr(""),
			path:    "/srv/dotfiles",
			want:    "## Added by caged when you allowed this link target.\nlinks:\n  allow:\n    - path: \"/srv/dotfiles\"\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store, fsys := reviewedStore(t, tt.content)
			got, err := store.AllowLink(tt.path, tt.readOnly)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Fatalf("content =\n%s\nwant\n%s", got, tt.want)
			}
			if fsys.files[editPath] != tt.want {
				t.Fatalf("file =\n%s\nwant the returned content", fsys.files[editPath])
			}
			if fsys.modes[editPath] != 0o600 {
				t.Errorf("mode = %v, want 0600", fsys.modes[editPath])
			}
			// The store answers from the written file, so the new policy
			// applies at once, and the guard sees the content it approves.
			if data, _, _ := store.Content(); string(data) != tt.want {
				t.Fatalf("Content after AllowLink = %q", data)
			}
			policy, err := store.LinkPolicy("/home/me")
			if err != nil {
				t.Fatal(err)
			}
			decision := policy.Check(tt.path + "/x")
			if decision.Verdict != pathpolicy.Allowed || decision.Root.ReadOnly != tt.readOnly {
				t.Fatalf("decision after AllowLink = %+v; want allowed, read-only %v", decision, tt.readOnly)
			}
		})
	}
}

func TestAllowLinkRefusesWhatItCannotDoSafely(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		path    string
		setup   func(*fakeFS)
		want    string
	}{
		{name: "flow list", content: "links:\n  allow: []\n", path: "/home/me/d", want: "links.allow is not a block-style list"},
		{name: "flow mapping", content: "links: {deny: [~/x]}\n", path: "/home/me/d", want: "links is not a block-style mapping"},
		{name: "too broad", content: "", path: "/home", want: "too broad"},
		{name: "inside a deny root", content: "", path: "/home/me/.ssh/keys", want: "inside the denied path"},
		{name: "relative", content: "", path: "dotfiles", want: "will not add"},
		{name: "holds a read-only root", content: "links:\n  allow:\n    - path: ~/d/pi\n      read_only: true\n", path: "/home/me/d", want: "read_only setting differs"},
		{
			name: "linked file", content: "", path: "/home/me/d", want: "is a symlink, which caged does not edit",
			setup: func(f *fakeFS) { f.lstatLinks = map[string]bool{editPath: true} },
		},
		{
			name: "changed after review", content: "# reviewed\n", path: "/home/me/d", want: "changed after caged reviewed it",
			setup: func(f *fakeFS) { f.put(editPath, []byte("links:\n  allow:\n    - path: ~/Documents\n")) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store, fsys := reviewedStore(t, &tt.content)
			if tt.setup != nil {
				tt.setup(fsys)
			}
			before := fsys.files[editPath]
			_, err := store.AllowLink(tt.path, false)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v; want it to mention %q", err, tt.want)
			}
			if tt.path != "dotfiles" && !strings.Contains(err.Error(), "yourself") {
				t.Errorf("err = %v; want the line to add by hand", err)
			}
			if fsys.files[editPath] != before || fsys.modes != nil {
				t.Fatalf("file was written: %q", fsys.files[editPath])
			}
		})
	}
}

func TestAllowLinkLeavesAnAllowedPathAlone(t *testing.T) {
	t.Parallel()
	content := "links:\n  allow:\n    - path: ~/d\n"
	store, fsys := reviewedStore(t, &content)
	got, err := store.AllowLink("/home/me/d", false)
	if err != nil || string(got) != content {
		t.Fatalf("AllowLink = %q, %v; want the content unchanged", got, err)
	}
	if fsys.modes != nil {
		t.Fatal("an already allowed path rewrote the file")
	}
}
