package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/cli"
)

// checkedDocs are the tracked documents a reader follows to use or change
// caged. CHANGELOG.md is history and is not checked.
var checkedDocs = []string{
	"README.md",
	"CONTRIBUTING.md",
	"docs/architecture.md",
	"docs/spec.md",
	"docs/implementation.md",
	"docs/first-run-config-feature-outline.md",
}

// foreignFlags are flags the docs name that belong to other programs, so no
// caged command has them.
var foreignFlags = map[string]string{
	"--rm": "docker", "--user": "docker", "--tmpfs": "docker", "--label": "docker", "--iidfile": "docker",
	"--entrypoint": "docker", "--format": "docker", "--platform": "docker", "--detach-keys": "docker",
	"-it": "docker", "-v": "docker", "-e": "docker", "-f": "docker",
	"--model": "an agent", "--resume": "an agent", "--add-dir": "an agent", "--provider": "an agent",
	"--device-auth": "an agent", "--no-daemon": "an agent", "--message": "an agent", "-i": "an agent",
	"-ldflags": "go build", "-s": "go build", "-race": "go test", "-shuffle": "go test", "-count": "go test",
	"-run": "go test", "-fuzz": "go test", "-fuzztime": "go test", "-cover": "go test", "-diff": "go mod tidy",
	"--snapshot": "goreleaser", "--clean": "goreleaser",
}

// documentedAbsences are code spans the docs show to say caged does not have
// them.
var documentedAbsences = map[string]string{
	"--no-auth":    "removed: --no-config does the same (spec FR-15)",
	"caged update": "out of scope: caged build --no-cache is the update path (spec)",
}

// docFacts is what the docs may name.
type docFacts struct {
	root    *cobra.Command
	agents  []string
	targets []string
	// exists reports whether a slash path relative to the repository root exists.
	exists func(rel string) bool
}

var (
	codeSpan     = regexp.MustCompile("`([^`]+)`")
	markdownLink = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	repoPath     = regexp.MustCompile(`^(internal|cmd|dev|docs|scripts|\.github)/[A-Za-z0-9_./-]*?(:[0-9]+(-[0-9]+)?)?$`)
	flagSpan     = regexp.MustCompile(`^(-{1,2}[A-Za-z][A-Za-z0-9-]*)(?:, (--[a-z][a-z0-9-]*))?(?:[ =].*)?$`)
	makeCommand  = regexp.MustCompile(`(?:^|\s)make ([A-Za-z0-9_-]+)`)
)

// docProblems returns one line per false statement docs/<doc> makes about
// caged: a command, agent, flag, make target or file that does not exist.
// It reads only text and facts, so a wrong sentence can be tested directly.
func docProblems(doc, text string, facts docFacts) []string {
	var problems []string
	report := func(line int, format string, args ...any) {
		problems = append(problems, fmt.Sprintf("%s:%d: %s", doc, line, fmt.Sprintf(format, args...)))
	}
	fenced := false
	for i, line := range strings.Split(text, "\n") {
		n := i + 1
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		var snippets []string
		if fenced {
			snippets = []string{strings.TrimPrefix(strings.TrimSpace(line), "$ ")}
		} else {
			for _, m := range codeSpan.FindAllStringSubmatch(line, -1) {
				snippets = append(snippets, m[1])
			}
			for _, m := range markdownLink.FindAllStringSubmatch(line, -1) {
				if problem := linkProblem(doc, m[1], facts); problem != "" {
					report(n, "%s", problem)
				}
			}
		}
		for _, snippet := range snippets {
			for _, problem := range snippetProblems(snippet, fenced, facts) {
				report(n, "%s", problem)
			}
		}
	}
	return problems
}

// snippetProblems checks one inline code span, or one line of a fenced block.
func snippetProblems(snippet string, fenced bool, facts docFacts) []string {
	if documentedAbsences[snippet] != "" {
		return nil
	}
	var problems []string
	if snippet == "caged" || strings.HasPrefix(snippet, "caged ") {
		problems = append(problems, commandProblems(snippet, facts)...)
	}
	for _, m := range makeCommand.FindAllStringSubmatch(snippet, -1) {
		if !slices.Contains(facts.targets, m[1]) {
			problems = append(problems, fmt.Sprintf("make %s: no such Makefile target", m[1]))
		}
	}
	if fenced {
		return problems
	}
	if m := flagSpan.FindStringSubmatch(snippet); m != nil {
		for _, flag := range m[1:] {
			if flag != "" && !anyCommandHasFlag(facts.root, flag) && foreignFlags[flag] == "" {
				problems = append(problems, fmt.Sprintf("%s: no caged command has this flag", flag))
			}
		}
	}
	if repoPath.MatchString(snippet) && !strings.ContainsAny(snippet, "*<") {
		rel, _, _ := strings.Cut(snippet, ":") // the line number is not part of the path
		if !facts.exists(strings.TrimSuffix(rel, "/")) {
			problems = append(problems, fmt.Sprintf("%s: no such file or directory", rel))
		}
	}
	return problems
}

// commandProblems checks a "caged ..." command line: each flag exists on the
// command it is given to, and the first word names a command or an agent.
// Placeholders (<agent>, [prompt], NAME, 'quoted prompts') are skipped, and
// everything after "--" belongs to the agent.
func commandProblems(line string, facts docFacts) []string {
	var problems []string
	cmd := facts.root
	positional := false
	words := strings.Fields(line)[1:]
	for i := 0; i < len(words); i++ {
		word := words[i]
		switch {
		case word == "--" || word == "#":
			return problems
		case strings.HasPrefix(word, "-"):
			name, _, hasValue := strings.Cut(word, "=")
			found, takesValue := lookupFlag(cmd, name)
			if !found {
				problems = append(problems, fmt.Sprintf("%s: no flag %s", cmd.CommandPath(), name))
			} else if takesValue && !hasValue {
				i++ // the next word is the flag's value
			}
		case positional || isPlaceholder(word):
			positional = true
		case cmd == facts.root:
			positional = true
			if sub := findCommand(cmd, word); sub != nil {
				cmd = sub
				positional = false
			} else if !slices.Contains(facts.agents, word) {
				problems = append(problems, fmt.Sprintf("caged %s: no such command or agent", word))
			}
		default:
			positional = true
		}
	}
	return problems
}

// isPlaceholder reports whether a word stands for a value the reader fills in.
func isPlaceholder(word string) bool {
	return strings.ContainsAny(word, "[]<>'\"$~/.=:|") || strings.ToUpper(word) == word
}

func findCommand(parent *cobra.Command, name string) *cobra.Command {
	for _, sub := range parent.Commands() {
		if sub.Name() == name || slices.Contains(sub.Aliases, name) {
			return sub
		}
	}
	return nil
}

// lookupFlag reports whether cmd has flag ("-x" or "--name"), counting
// inherited flags, and whether the flag takes a value. It never names
// *pflag.Flag: go.mod lists pflag as indirect, and a direct import would fail
// the tidy check.
func lookupFlag(cmd *cobra.Command, flag string) (found, takesValue bool) {
	if long, ok := strings.CutPrefix(flag, "--"); ok {
		f := cmd.Flags().Lookup(long)
		if f == nil {
			f = cmd.InheritedFlags().Lookup(long)
		}
		return f != nil, f != nil && f.NoOptDefVal == ""
	}
	short := strings.TrimPrefix(flag, "-")
	if len(short) != 1 { // ShorthandLookup panics on longer names
		return false, false
	}
	f := cmd.Flags().ShorthandLookup(short)
	if f == nil {
		f = cmd.InheritedFlags().ShorthandLookup(short)
	}
	return f != nil, f != nil && f.NoOptDefVal == ""
}

// anyCommandHasFlag reports whether root or any command under it has flag.
func anyCommandHasFlag(root *cobra.Command, flag string) bool {
	if found, _ := lookupFlag(root, flag); found {
		return true
	}
	for _, sub := range root.Commands() {
		if anyCommandHasFlag(sub, flag) {
			return true
		}
	}
	return false
}

// linkProblem checks a relative markdown link target; web links and anchors
// are not checked.
func linkProblem(doc, target string, facts docFacts) string {
	if strings.Contains(target, "://") || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") {
		return ""
	}
	file, _, _ := strings.Cut(target, "#")
	rel := path.Clean(path.Join(path.Dir(doc), file))
	if facts.exists(rel) {
		return ""
	}
	return fmt.Sprintf("link %s: no such file", target)
}

// realDocFacts are the facts of this repository: the real command tree with
// cobra's default help, completion, help and version flags added.
func realDocFacts(t *testing.T) docFacts {
	t.Helper()
	root := cli.NewRootCmd("test", agent.Builtin(), cli.Services{})
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	root.InitDefaultVersionFlag()
	addHelpFlags(root)
	makefile, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	var targets []string
	for _, m := range regexp.MustCompile(`(?m)^([A-Za-z0-9_-]+):`).FindAllStringSubmatch(string(makefile), -1) {
		targets = append(targets, m[1])
	}
	return docFacts{
		root:    root,
		agents:  agent.Builtin().Names(),
		targets: targets,
		exists: func(rel string) bool {
			_, err := os.Stat(filepath.Join("..", "..", filepath.FromSlash(rel)))
			return err == nil
		},
	}
}

// addHelpFlags gives every command the -h, --help flag cobra adds when the
// command runs.
func addHelpFlags(cmd *cobra.Command) {
	cmd.InitDefaultHelpFlag()
	for _, sub := range cmd.Commands() {
		addHelpFlags(sub)
	}
}

// TestDocsNameOnlyWhatExists keeps the README and docs honest: every caged
// command line, flag, make target, repository path and relative link they
// show must exist.
func TestDocsNameOnlyWhatExists(t *testing.T) {
	facts := realDocFacts(t)
	for _, doc := range checkedDocs {
		text, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(doc)))
		if err != nil {
			t.Fatal(err)
		}
		for _, problem := range docProblems(doc, string(text), facts) {
			t.Error(problem)
		}
	}
}

// TestDocProblemsFindsWrongSentences shows the checker reports each kind of
// false statement and accepts the true ones next to it.
func TestDocProblemsFindsWrongSentences(t *testing.T) {
	facts := realDocFacts(t)
	files := []string{"README.md", "docs/spec.md", "internal/cli/root.go", "cmd/caged"}
	facts.exists = func(rel string) bool { return slices.Contains(files, rel) }
	tests := []struct {
		name, text string
		want       []string
	}{
		{name: "true statements", text: "Run `caged codex 'fix tests'`, `caged --verbose -w ../app claude`, `caged doctor`, `caged help list`,\n" +
			"`caged -p 8080 --env NAME pi`, `caged <agent> [prompt] -- --resume`, `--no-mount`, `-w, --workdir DIR`, `--rm`,\n" +
			"`make check`, `internal/cli/root.go:11`, `cmd/caged/`, [spec](spec.md) in docs, [x](https://example.com).", want: nil},
		{name: "unknown flag on the root command", text: "`caged --no-auth claude`", want: []string{"d.md:1: caged: no flag --no-auth"}},
		{name: "flag given to the wrong command", text: "`caged doctor --no-cache`", want: []string{"d.md:1: caged doctor: no flag --no-cache"}},
		{name: "unknown command", text: "`caged upgrade`", want: []string{"d.md:1: caged upgrade: no such command or agent"}},
		{name: "unknown agent in a fenced block", text: "```sh\n$ caged aider 'hi'\n```", want: []string{"d.md:2: caged aider: no such command or agent"}},
		{name: "fenced blocks keep their prose out", text: "```\n`--nope`\n```", want: nil},
		{name: "bare flag that no command has", text: "use `--no-config` or `--trust-everything`", want: []string{"d.md:1: --trust-everything: no caged command has this flag"}},
		{name: "missing make target", text: "run `make install`", want: []string{"d.md:1: make install: no such Makefile target"}},
		{name: "missing repository path", text: "see `internal/cli/gone.go:3`", want: []string{"d.md:1: internal/cli/gone.go: no such file or directory"}},
		{name: "a documented absence", text: "There is no `--no-auth` and no `caged update`.", want: nil},
		{name: "missing relative link", text: "see [releasing](releasing.md#tags)", want: []string{"d.md:1: link releasing.md#tags: no such file"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := "d.md"
			if tt.name == "true statements" {
				doc = "docs/d.md" // so the relative link resolves to docs/spec.md
			}
			got := docProblems(doc, tt.text, facts)
			if !slices.Equal(got, tt.want) {
				t.Errorf("docProblems() = %q, want %q", got, tt.want)
			}
		})
	}
}
