package app

import (
	"fmt"
	"io"
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

// Notice writes one line, always. Progress output is best effort, so write
// errors are ignored here and nowhere else.
func (r *Reporter) Notice(format string, args ...any) {
	_, _ = fmt.Fprintf(r.w, format+"\n", args...)
}

// Detail writes one line, only with --verbose.
func (r *Reporter) Detail(format string, args ...any) {
	if r.verbose {
		r.Notice(format, args...)
	}
}
