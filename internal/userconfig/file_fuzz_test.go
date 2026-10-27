package userconfig

import (
	"regexp"
	"strings"
	"testing"
)

// The rules below are written independently of the parser: what config.yaml
// may contain, and how caged's messages must look.
var (
	aptNameRule = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]+$`)
	// quoted is any double- or single-quoted run, the user's own text in a message.
	quoted = regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'[^']*'`)
)

// leak matches what a raw yaml type message shows and caged's must not, once
// the user's quoted text is removed: a Go package or one of the file* types,
// yaml's "into T" and "in type T", a "!!" tag, a backtick-quoted value, or
// yaml's own prefix.
var leak = regexp.MustCompile("userconfig\\.|\\bfile[A-Z]\\w*|\\binto \\S|in type |!!|`|yaml: ")

func pathRule(path string) bool {
	return (path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "/")) && !strings.Contains(path, "$")
}

// FuzzParse checks that parse never panics; that every error names the file
// first, shows no control character and none of yaml's Go-facing text; and
// that every accepted config holds only valid apt names (sorted, without
// duplicates) and valid paths.
func FuzzParse(f *testing.F) {
	for _, tt := range parseCases() {
		f.Add([]byte(tt.input))
	}
	for _, seed := range []string{
		starterConfig,
		"\uFEFFimage:\n  apt: [jq]\n", "image:\r\n  apt: [jq]\r\n", "image:\n\tapt: [jq]\n", "image:\n  apt: [\"a\x00b\"]\n", "image: \xff\n",
		"a: &a [x]\nb: *a\n", "<<: {image: {apt: [jq]}}\n", "image: !foo x\n", "version: !!foo 1\n", "image: !!foo%1B[31m x\n",
		"links:\n  deny: [/srv/$x]\n", "image:\n  apt: [a]\n", // a variable in an absolute path; a one-letter name
		strings.Repeat("[", 10001), "workdirs:\n", "workdirs: ~\n", "? &k image\n: {apt: [jq]}\n*k : {apt: [tmux]}\n",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<14 {
			data = data[:1<<14]
		}
		cfg, err := parse(data, "SRC")
		if err != nil {
			msg := err.Error()
			if !strings.HasPrefix(msg, "SRC: ") {
				t.Fatalf("parse(%q) error %q does not start with the file", data, msg)
			}
			if i := strings.IndexFunc(msg, func(r rune) bool { return r < 0x20 || r == 0x7f }); i >= 0 {
				t.Fatalf("parse(%q) error %q holds a control character at %d", data, msg, i)
			}
			if found := leak.FindString(quoted.ReplaceAllString(msg, `""`)); found != "" {
				t.Fatalf("parse(%q) error %q shows %q", data, msg, found)
			}
			return
		}
		for i, name := range cfg.apt {
			if !aptNameRule.MatchString(name) {
				t.Fatalf("parse(%q) accepted apt name %q", data, name)
			}
			if i > 0 && cfg.apt[i-1] >= name {
				t.Fatalf("parse(%q) apt = %q; want sorted without duplicates", data, cfg.apt)
			}
		}
		paths := append(append([]string(nil), cfg.linkDeny...), cfg.workdirs...)
		for _, root := range cfg.linkAllow {
			paths = append(paths, root.path)
		}
		for _, path := range paths {
			if !pathRule(path) {
				t.Fatalf("parse(%q) accepted path %q", data, path)
			}
		}
	})
}
