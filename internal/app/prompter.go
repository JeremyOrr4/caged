package app

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Prompter asks the user yes/no questions on the terminal. One Prompter
// serves a whole command, so every question reads from the same buffered
// reader and an answer typed ahead for a later question is not lost.
type Prompter struct {
	in          *bufio.Reader
	out         io.Writer
	interactive bool
}

// NewPrompter returns a Prompter that reads answers from in and writes
// questions to out. interactive says in is a terminal, so someone can answer.
func NewPrompter(in io.Reader, out io.Writer, interactive bool) *Prompter {
	return &Prompter{in: bufio.NewReader(in), out: out, interactive: interactive}
}

// Interactive reports whether someone can answer.
func (p *Prompter) Interactive() bool { return p.interactive }

// Confirm asks question, followed by " [y/N] ". Anything but y or yes, or no
// input at all, means no.
func (p *Prompter) Confirm(question string) bool {
	_, _ = fmt.Fprint(p.out, printable(question)+" [y/N] ") // best effort: an unseen question still needs a typed yes
	// A read error ends the answer: what was typed before it counts ("y" then
	// Ctrl-D is a yes), and no input at all is a no.
	line, _ := p.in.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}
