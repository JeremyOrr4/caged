package app

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/JeremyOrr4/caged/internal/textdiff"
)

// ConfigInitializer creates the starter config file on first use.
// *userconfig.Store implements it. An error with a MissingDir method that
// returns true means the config folder could not be created, so nothing is
// mounted that the container could write a config file into.
type ConfigInitializer interface {
	Ensure() (created bool, err error)
}

// ConfigContent returns the config file bytes caged acts on, and where they
// came from. *userconfig.Store implements it.
type ConfigContent interface {
	Content() (data []byte, found bool, err error)
	Source() (string, error)
}

// ConfigApprovals remembers the config content the user last approved.
// *userconfig.Approvals implements it.
type ConfigApprovals interface {
	Approved() (data []byte, ok bool, err error)
	Approve(data []byte) error
}

// guardOptions say how a command that acts on the config uses the file.
type guardOptions struct {
	// Create writes the starter file when there is none. Commands that skip
	// host config (--no-config) leave it false.
	Create bool
	// Trust approves a changed file without asking (--trust-config).
	Trust bool
}

// GuardDeps are the collaborators of a ConfigGuard.
type GuardDeps struct {
	Init      ConfigInitializer
	Content   ConfigContent
	Approvals ConfigApprovals
	In        io.Reader // answers the approval question
	Out       io.Writer // warnings, the change report and the question
	// Interactive is true when In is a terminal, so someone can answer.
	Interactive bool
}

// ConfigGuard prepares the config file for a command: it creates the starter
// file on first use, and makes sure the user has seen and approved every
// change since the last run. The container can edit config.yaml, so a change
// may not have been made by the user.
type ConfigGuard struct{ deps GuardDeps }

// NewConfigGuard returns a ConfigGuard.
func NewConfigGuard(deps GuardDeps) *ConfigGuard { return &ConfigGuard{deps: deps} }

// prepare readies the config file for a command that acts on it: it creates
// the file when opts.Create is set, then compares it with the last approved
// copy. The first copy seen is recorded without a question. A changed file
// stops the command unless the user approves it, or opts.Trust is set. A
// config folder that cannot be created is only a warning: with no folder
// there is nothing to review and nothing is mounted, and an empty approved
// copy makes any file that appears later a change (expectNothing). Any other
// failure to create the file stops the command, because its folder would be
// mounted with nothing approved (D-55).
func (g *ConfigGuard) prepare(opts guardOptions) error {
	if opts.Create {
		if err := g.createForReview(); err != nil {
			return err
		}
	}
	change, err := g.compare()
	if err != nil || change == nil {
		return err
	}
	if change.first {
		return g.deps.Approvals.Approve(change.current)
	}
	g.report(change, false)
	switch {
	case opts.Trust:
		g.say("Approved with --trust-config.")
	case !g.deps.Interactive:
		return fmt.Errorf("%s changed since you last approved it and there is no terminal to ask; review the diff above, then run caged from a terminal or pass --trust-config", change.source)
	case !g.confirm():
		return fmt.Errorf("%s changed and was not approved; nothing was run. Undo the change, or run caged again and answer y", change.source)
	}
	return g.deps.Approvals.Approve(change.current)
}

// notify is prepare for commands that only report. It creates the file and
// shows a change, but never asks, never approves anything but a first copy,
// and never stops the command: a failed creation or a failed record of the
// first copy is a warning, and an invalid file is left for the command to
// report.
func (g *ConfigGuard) notify() {
	if err := g.create(); err != nil {
		g.say("caged: warning: %v", err)
	}
	change, err := g.compare()
	if err != nil || change == nil {
		return // doctor and list report an invalid file themselves
	}
	if change.first {
		if err := g.deps.Approvals.Approve(change.current); err != nil {
			g.say("caged: warning: %v", err)
		}
		return
	}
	g.report(change, true)
}

// create writes the starter file when there is none, and says so.
func (g *ConfigGuard) create() error {
	created, err := g.deps.Init.Ensure()
	if err != nil || !created {
		return err
	}
	// best effort: the notice needs the path; without one the file still exists
	if source, err := g.deps.Content.Source(); err == nil {
		g.say("caged: created starter config %s (every example is commented out)", source)
	}
	return nil
}

// createForReview is create for prepare: a config folder that could not be
// created is a warning, and expectNothing records that nothing was approved.
func (g *ConfigGuard) createForReview() error {
	err := g.create()
	if err == nil || !missingDir(err) {
		return err
	}
	g.say("caged: warning: %v", err)
	return g.expectNothing()
}

// expectNothing records an empty approved copy when nothing is approved yet,
// for a config folder that could not be created: a container may still be
// able to create it (one that runs as another user, as under rootless
// Docker), and the file it puts there must be shown and asked about, not
// taken as the first copy. A failure to record stops the command.
func (g *ConfigGuard) expectNothing() error {
	_, known, err := g.deps.Approvals.Approved()
	if err != nil || known {
		return err
	}
	return g.deps.Approvals.Approve(nil)
}

// missingDir reports whether err says the config folder could not be
// created (ConfigInitializer).
func missingDir(err error) bool {
	var missing interface{ MissingDir() bool }
	return errors.As(err, &missing) && missing.MissingDir()
}

// configChange is config.yaml as last approved and as it is now. first means
// nothing was approved yet: the caller records current without a question.
type configChange struct {
	source            string
	approved, current []byte
	first             bool
}

// compare returns how config.yaml differs from the last approved copy, or nil
// when it does not. It records nothing.
func (g *ConfigGuard) compare() (*configChange, error) {
	current, found, err := g.deps.Content.Content()
	if err != nil {
		return nil, err
	}
	approved, known, err := g.deps.Approvals.Approved()
	if err != nil {
		return nil, err
	}
	switch {
	case !known && found:
		return &configChange{current: current, first: true}, nil
	case !known, bytes.Equal(approved, current):
		return nil, nil
	}
	source, err := g.deps.Content.Source()
	if err != nil {
		return nil, err
	}
	return &configChange{source: source, approved: approved, current: current}, nil
}

// say writes one line to Out, made printable as Reporter's lines are.
func (g *ConfigGuard) say(format string, args ...any) {
	_, _ = io.WriteString(g.deps.Out, printable(fmt.Sprintf(format, args...))+"\n") // best effort: progress output, as in Reporter
}

// report prints a banner, what changed, and the diff. pending says the change
// is not approved yet, for commands that do not ask.
func (g *ConfigGuard) report(change *configChange, pending bool) {
	const rule = "================================================================"
	result := textdiff.Compare(change.approved, change.current)
	g.say("\n%s\n CAGED CONFIG CHANGED: %s\n%s", rule, change.source, rule)
	g.say("This file decides what caged installs and which host paths it mounts. It is")
	g.say("mounted read/write into containers, so an agent may have edited it.")
	summary := fmt.Sprintf("%s added, %s removed", plural(result.Added, "line"), plural(result.Removed, "line"))
	if slices.Equal(significantLines(change.approved), significantLines(change.current)) {
		summary += " (comments and blank lines only; the settings are the same)"
	}
	g.say("Since you last approved it: %s.\n", summary)
	g.say("%s", strings.TrimRight(result.Unified("approved", "current"), "\n"))
	if pending {
		g.say("\nNot approved yet. caged will ask before it runs with this change.")
	}
	g.say("%s", rule)
}

// confirm asks whether to use the changed file. Anything but y or yes, or no
// input at all, means no.
func (g *ConfigGuard) confirm() bool {
	_, _ = fmt.Fprint(g.deps.Out, "Use this config? [y/N] ") // best effort: an unseen question still needs a typed yes
	// A read error ends the answer: what was typed before it counts ("y" then
	// Ctrl-D is a yes), and no input at all is a no.
	line, _ := bufio.NewReader(g.deps.In).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// significantLines returns the lines of data that are neither blank nor
// comment-only, without trailing space. A trailing comment on a setting is
// kept, so changing it counts as a change.
func significantLines(data []byte) []string {
	var out []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			out = append(out, strings.TrimRight(line, " \t\r"))
		}
	}
	return out
}
