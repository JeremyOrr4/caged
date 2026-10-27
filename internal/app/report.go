package app

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Reporter writes progress messages for one command. Notice lines always
// appear; Detail lines appear only with --verbose.
type Reporter struct {
	w       io.Writer
	verbose bool
}

// NewReporter returns a Reporter that writes to w and, when verbose is set,
// also writes Detail lines.
func NewReporter(w io.Writer, verbose bool) *Reporter {
	return &Reporter{w: w, verbose: verbose}
}

// Notice writes one line, always, made printable. Progress output is best
// effort, so write errors are ignored.
func (r *Reporter) Notice(format string, args ...any) {
	_, _ = io.WriteString(r.w, printable(fmt.Sprintf(format, args...))+"\n")
}

// Detail writes one line, only with --verbose.
func (r *Reporter) detail(format string, args ...any) {
	if r.verbose {
		r.Notice(format, args...)
	}
}

// PrintableLine is printable for text shown as one line or one field of a
// table: newlines and tabs are escaped too, so a path cannot start a row of
// its own or shift a column.
func PrintableLine(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(printable(s), "\n", `\n`), "\t", `\t`)
}

// printable escapes, Go style (\x1b, \r, \u202e), every rune a terminal could
// act on: control characters, invisible format characters such as a
// right-to-left override, and bytes that are not UTF-8. Newlines and tabs are
// kept. Paths, errors and config lines can come from the container, so they
// must not move the cursor or rewrite what the user reads.
func printable(s string) string {
	var b strings.Builder
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, `\x%02x`, s[0])
		case r == '\n', r == '\t', unicode.IsPrint(r):
			b.WriteString(s[:size])
		default:
			quoted := strconv.QuoteRune(r)
			b.WriteString(quoted[1 : len(quoted)-1])
		}
		s = s[size:]
	}
	return b.String()
}
