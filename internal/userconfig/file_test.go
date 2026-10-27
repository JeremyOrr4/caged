package userconfig

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const src = "~/.config/caged/config.yaml"

type parseCase struct {
	name    string
	input   string
	want    config
	wantErr string
}

// parseCases are the TestParse table; FuzzParse seeds from their inputs.
func parseCases() []parseCase {
	return []parseCase{
		{name: "packages are sorted", input: "image:\n  apt: [tmux, neovim, shellcheck]\n", want: config{apt: []string{"neovim", "shellcheck", "tmux"}}},
		{name: "duplicates are removed", input: "image:\n  apt: [tmux, neovim, tmux, tmux]\n", want: config{apt: []string{"neovim", "tmux"}}},
		{name: "all the allowed characters", input: "image:\n  apt: [g++, libstdc++-13-dev, python3.12, 0ad, a-b.c+d]\n", want: config{apt: []string{"0ad", "a-b.c+d", "g++", "libstdc++-13-dev", "python3.12"}}},
		{name: "empty file", input: ``, want: config{}},
		{name: "comments only", input: "# nothing here\n", want: config{}},
		{name: "empty list", input: "image:\n  apt: []\n", want: config{}},
		{name: "version 1", input: "version: 1\n", want: config{}},
		{
			name:  "links and workdirs",
			input: "links:\n  allow:\n    - path: ~/dotfiles\n      read_only: true\n    - path: /srv/dot\n  deny: [~/secrets]\nworkdirs:\n  allow:\n    - path: ~/git\n",
			want: config{
				linkAllow:   []allowRoot{{path: "~/dotfiles", readOnly: true}, {path: "/srv/dot"}},
				linkDeny:    []string{"~/secrets"},
				workdirsSet: true,
				workdirs:    []string{"~/git"},
			},
		},
		{name: "workdirs without roots is still set", input: "workdirs:\n  allow: []\n", want: config{workdirsSet: true}},
		{name: "workdirs with a null allow list is still set", input: "workdirs:\n  allow:\n", want: config{workdirsSet: true}},
		{name: "a bare workdirs key is still set", input: "workdirs:\n", want: config{workdirsSet: true}},
		{name: "a null workdirs key is still set", input: "workdirs: ~\n", want: config{workdirsSet: true}},
		{name: "a workdirs key spelled null is still set", input: "workdirs: null\n", want: config{workdirsSet: true}},
		{name: "a workdirs key tagged null is still set", input: "workdirs: !!null \"\"\n", want: config{workdirsSet: true}},
		{name: "a workdirs key that is an alias of null is still set", input: "version: &n ~\nworkdirs: *n\n", want: config{workdirsSet: true}},

		{name: "unknown field", input: "imgae:\n  apt: [jq]\n", wantErr: src + `: line 1: unknown field "imgae"`},
		{name: "unknown nested field", input: "image:\n  npm: [x]\n", wantErr: src + `: line 2: unknown field "npm"`},
		{name: "wrong type", input: "image: [x]\n", wantErr: src + ": line 1: wrong type (found seq)"},
		{name: "wrong type with a local tag", input: "image: !foo x\n", wantErr: src + ": line 1: wrong type"},
		{name: "wrong type with a local tag in a list", input: "links: {allow: [!foo x]}\n", wantErr: src + ": line 1: wrong type"},
		{name: "wrong type with a verbatim tag", input: "image: !<tag:example.com,2000:x> x\n", wantErr: src + ": line 1: wrong type"},
		{name: "wrong type with a local tag on a number", input: "version: !foo 1\n", wantErr: src + ": line 1: wrong type"},
		{name: "wrong type with control characters in a tag", input: "image: !!foo%1B[31m x\n", wantErr: src + ": line 1: wrong type"},
		{name: "wrong type with an unknown core tag", input: "version: !!foo 1\n", wantErr: src + ": line 1: wrong type (found foo)"},
		{name: "wrong type with the non-specific tag", input: "image: ! x\n", wantErr: src + ": line 1: wrong type (found str)"},
		{name: "invalid YAML", input: "image: [\n", wantErr: src + ": invalid YAML: line 1: did not find expected node content"},
		// yaml shows the value raw when an explicit core tag does not fit it; caged quotes it
		{name: "a core tag that does not fit its value", input: "version: !!bool maybe\n", wantErr: src + `: invalid YAML: cannot decode str "maybe" as bool`},
		{name: "a null tag on a number", input: "!!null 0", wantErr: src + `: invalid YAML: cannot decode int "0" as null`},
		{name: "a core tag on a value with control characters", input: "version: !!timestamp \"\\e]0;x\\a\"\n", wantErr: src + `: invalid YAML: cannot decode str "\x1b]0;x\a" as timestamp`},
		{name: "a core tag on a value that looks like the message", input: "version: !!int \"x` as a !!float\"\n", wantErr: src + `: invalid YAML: cannot decode str "x` + "`" + ` as a !!float" as int`},
		{name: "duplicate key", input: "image:\n  apt: [jq]\nimage:\n  apt: [tmux]\n", wantErr: src + `: line 3: mapping key "image" already defined at line 1`},
		// an alias used as a key repeats a field from another node; yaml's own message names caged's Go type
		{name: "duplicate key through an alias", input: "? &k image\n: {apt: [jq]}\n*k : {apt: [tmux]}\n", wantErr: src + `: line 3: duplicate field "image"`},
		// yaml compares alias keys by their anchor name before it decodes them
		{name: "the same alias used twice as a key", input: "? &k image\n: {apt: [jq]}\n*k : {apt: [tmux]}\n*k : {apt: [x]}\n", wantErr: src + `: line 4: mapping key "k" already defined at line 3`},
		{name: "duplicate nested key through an alias", input: "links:\n  ? &d deny\n  : [~/a]\n  *d : [~/b]\n", wantErr: src + `: line 4: duplicate field "deny"`},
		{name: "two documents", input: "version: 1\n---\nversion: 1\n", wantErr: src + ": only one YAML document is allowed"},
		{name: "unsupported version", input: "version: 2\n", wantErr: src + ": unsupported version 2; this caged reads version 1"},

		{name: "shell syntax", input: "image:\n  apt: [\"vim; curl x\"]\n", wantErr: src + `: invalid apt package name "vim; curl x" in image.apt; names must match ^[a-z0-9][a-z0-9+.-]+$`},
		{name: "version pin", input: "image:\n  apt: [\"vim=2:9.1\"]\n", wantErr: src + `: invalid apt package name "vim=2:9.1" in image.apt; names must match ^[a-z0-9][a-z0-9+.-]+$`},
		{name: "uppercase", input: "image:\n  apt: [Vim]\n", wantErr: src + `: invalid apt package name "Vim" in image.apt; names must match ^[a-z0-9][a-z0-9+.-]+$`},
		{name: "the first bad name is reported", input: "image:\n  apt: [vim, BAD, \"also bad\"]\n", wantErr: src + `: invalid apt package name "BAD" in image.apt; names must match ^[a-z0-9][a-z0-9+.-]+$`},

		{name: "unquoted ~ is YAML null", input: "links:\n  allow:\n    - path: ~\n", wantErr: src + `: links.allow entry has no path (a bare ~ is YAML null; quote it as "~")`},
		{name: "links.allow entry without a path", input: "links:\n  allow:\n    - read_only: true\n", wantErr: src + `: links.allow entry has no path (a bare ~ is YAML null; quote it as "~")`},
		{name: "unquoted ~ in links.deny", input: "links:\n  deny: [~]\n", wantErr: src + `: links.deny entry has no path (a bare ~ is YAML null; quote it as "~")`},
		{name: "a null item in image.apt", input: "image:\n  apt: [tmux, ~]\n", wantErr: src + `: invalid apt package name "" in image.apt; names must match ^[a-z0-9][a-z0-9+.-]+$`},
		{name: "an empty item in links.deny", input: "links:\n  deny:\n    -\n", wantErr: src + `: links.deny entry has no path (a bare ~ is YAML null; quote it as "~")`},
		{name: "a null item in links.allow", input: "links:\n  allow:\n    - ~\n", wantErr: src + `: links.allow entry has no path (write each entry as "- path: ~/dir")`},
		{name: "an empty item in workdirs.allow", input: "workdirs:\n  allow:\n    - path: ~/git\n    -\n", wantErr: src + `: workdirs.allow entry has no path (write each entry as "- path: ~/dir")`},
		{name: "unquoted ~ in workdirs.allow", input: "workdirs:\n  allow:\n    - path: ~\n", wantErr: src + `: workdirs.allow entry has no path (a bare ~ is YAML null; quote it as "~")`},
		{name: "relative links.allow", input: "links:\n  allow:\n    - path: relative/x\n", wantErr: src + `: links.allow entry "relative/x" must be an absolute path or start with ~/`},
		{name: "variable in links.allow", input: "links:\n  allow:\n    - path: $HOME/x\n", wantErr: src + `: links.allow entry "$HOME/x" must be an absolute path or start with ~/`},
		{name: "relative links.deny", input: "links:\n  deny: [secrets]\n", wantErr: src + `: links.deny entry "secrets" must be an absolute path or start with ~/`},
		{name: "relative workdirs.allow", input: "workdirs:\n  allow:\n    - path: git\n", wantErr: src + `: workdirs.allow entry "git" must be an absolute path or start with ~/`},
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	for _, tt := range parseCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parse([]byte(tt.input), src)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("parse() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parse() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestInvalidYAML calls invalidYAML directly: the last four rows are shapes
// yaml does not produce today.
func TestInvalidYAML(t *testing.T) {
	t.Parallel()
	tests := []struct{ message, want string }{
		{"yaml: line 1: did not find expected node content", "line 1: did not find expected node content"},
		{"yaml: cannot decode !!str `x` as a !!int", `cannot decode str "x" as int`},
		{"yaml: cannot decode !!str x as a !!int", "cannot decode a tagged value"},
		{"yaml: cannot decode !!str `x`", "cannot decode a tagged value"},
		{"yaml: cannot decode !!str `x` as a !!Int", "cannot decode a tagged value"},
		{"yaml: cannot decode !!x\x1b `x` as a !!int", "cannot decode a tagged value"},
	}
	for _, tt := range tests {
		if got := invalidYAML(tt.message); got != tt.want {
			t.Errorf("invalidYAML(%q) = %q, want %q", tt.message, got, tt.want)
		}
	}
}

func TestExpand(t *testing.T) {
	t.Parallel()
	tests := []struct{ path, want string }{
		{"~", "/home/me"},
		{"~/dot", "/home/me/dot"},
		{"/srv/dot/", "/srv/dot"},
	}
	for _, tt := range tests {
		if got := expand("/home/me", tt.path); got != tt.want {
			t.Errorf("expand(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// TestParseNullAndTildeInEveryList puts a null item (a bare ~ or an empty "-",
// as the whole item or as a root's path) or a quoted "~" at every position of
// a valid list of each list field: a null item is refused naming its field,
// never dropped; a quoted "~" is kept as "~" in a path field and refused as an
// apt name.
func TestParseNullAndTildeInEveryList(t *testing.T) {
	t.Parallel()

	const noPath = `entry has no path (a bare ~ is YAML null; quote it as "~")`
	const nullRoot = `entry has no path (write each entry as "- path: ~/dir")`
	const badApt = `SRC: invalid apt package name %q in image.apt; names must match ^[a-z0-9][a-z0-9+.-]+$`
	type nullForm struct{ line, refusal string } // the refusal follows "SRC: <field> "
	nullItems := []nullForm{{"    - ~\n", noPath}, {"    -\n", noPath}}
	nullRoots := []nullForm{{"    - ~\n", nullRoot}, {"    -\n", nullRoot}, {"    - path: ~\n", noPath}, {"    - path:\n", noPath}}
	lists := []struct {
		field, head, item string
		nulls             []nullForm
		quoted            string
		paths             func(config) []string
	}{
		{"image.apt", "image:\n  apt:\n", "    - p%d\n", nullItems, "    - \"~\"\n", nil},
		{"links.deny", "links:\n  deny:\n", "    - ~/d%d\n", nullItems, "    - \"~\"\n", func(c config) []string { return c.linkDeny }},
		{"links.allow", "links:\n  allow:\n", "    - path: ~/a%d\n", nullRoots, "    - path: \"~\"\n", func(c config) []string {
			var paths []string
			for _, root := range c.linkAllow {
				paths = append(paths, root.path)
			}
			return paths
		}},
		{"workdirs.allow", "workdirs:\n  allow:\n", "    - path: ~/w%d\n", nullRoots, "    - path: \"~\"\n", func(c config) []string { return c.workdirs }},
	}
	for _, list := range lists {
		// doc is the list with n valid items and bad at position pos.
		doc := func(bad string, n, pos int) string {
			var doc strings.Builder
			doc.WriteString(list.head)
			for i := range n + 1 {
				if i == pos {
					doc.WriteString(bad)
				}
				if i < n {
					fmt.Fprintf(&doc, list.item, i)
				}
			}
			return doc.String()
		}
		for n := range 5 {
			for pos := range n + 1 {
				for _, null := range list.nulls {
					in := doc(null.line, n, pos)
					want := "SRC: " + list.field + " " + null.refusal
					if list.paths == nil {
						want = fmt.Sprintf(badApt, "")
					}
					if cfg, err := parse([]byte(in), "SRC"); err == nil || err.Error() != want {
						t.Errorf("parse(%q) = %+v, %v; want error %q", in, cfg, err, want)
					}
				}
				in := doc(list.quoted, n, pos)
				cfg, err := parse([]byte(in), "SRC")
				if list.paths == nil {
					if want := fmt.Sprintf(badApt, "~"); err == nil || err.Error() != want {
						t.Errorf("parse(%q) = %+v, %v; want error %q", in, cfg, err, want)
					}
					continue
				}
				if got := list.paths(cfg); err != nil || len(got) != n+1 || got[pos] != "~" {
					t.Errorf("parse(%q) paths = %q, %v; want %d entries with \"~\" at %d", in, got, err, n+1, pos)
				}
			}
		}
	}
}

// TestParseUnknownFieldLine checks that an unknown key is named with its line
// after any number of lines of each kind: comments, blank lines, CRLF lines
// and known keys.
func TestParseUnknownFieldLine(t *testing.T) {
	t.Parallel()

	prefixes := []string{"# comment\n", "\n", "# crlf\r\n", "version: 1\n"}
	for _, prefix := range prefixes {
		for k := range 64 {
			doc := strings.Repeat(prefix, k)
			if prefix == "version: 1\n" && k > 1 {
				doc = "version: 1\n" + strings.Repeat("# comment\n", k-1) // a key may appear once
			}
			doc += "x: 1\n"
			want := fmt.Sprintf(`SRC: line %d: unknown field "x"`, k+1)
			if _, err := parse([]byte(doc), "SRC"); err == nil || err.Error() != want {
				t.Errorf("parse(%q) error = %v; want %q", doc, err, want)
			}
		}
	}
	nested := "image:\n  apt:\n    - jq\n    - tmux\nlinks:\n  deny: [~/x]\nx: 1\n"
	if _, err := parse([]byte(nested), "SRC"); err == nil || err.Error() != `SRC: line 7: unknown field "x"` {
		t.Errorf("parse(%q) error = %v; want line 7", nested, err)
	}
}
