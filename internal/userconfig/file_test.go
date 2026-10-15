package userconfig

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()

	const src = "~/.config/caged/config.yaml"
	tests := []struct {
		name    string
		input   string
		want    config
		wantErr string
	}{
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

		{name: "unknown field", input: "imgae:\n  apt: [jq]\n", wantErr: src + `: line 1: unknown field "imgae"`},
		{name: "unknown nested field", input: "image:\n  npm: [x]\n", wantErr: src + `: line 2: unknown field "npm"`},
		{name: "wrong type", input: "image: [x]\n", wantErr: src + ": line 1: wrong type (found seq)"},
		{name: "invalid YAML", input: "image: [\n", wantErr: src + ": invalid YAML: line 1: did not find expected node content"},
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
		{name: "unquoted ~ in workdirs.allow", input: "workdirs:\n  allow:\n    - path: ~\n", wantErr: src + `: workdirs.allow entry has no path (a bare ~ is YAML null; quote it as "~")`},
		{name: "relative links.allow", input: "links:\n  allow:\n    - path: relative/x\n", wantErr: src + `: links.allow entry "relative/x" must be an absolute path or start with ~/`},
		{name: "variable in links.allow", input: "links:\n  allow:\n    - path: $HOME/x\n", wantErr: src + `: links.allow entry "$HOME/x" must be an absolute path or start with ~/`},
		{name: "relative links.deny", input: "links:\n  deny: [secrets]\n", wantErr: src + `: links.deny entry "secrets" must be an absolute path or start with ~/`},
		{name: "relative workdirs.allow", input: "workdirs:\n  allow:\n    - path: git\n", wantErr: src + `: workdirs.allow entry "git" must be an absolute path or start with ~/`},
	}
	for _, tt := range tests {
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
