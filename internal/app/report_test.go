package app

import (
	"strings"
	"testing"
)

func TestReporter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		verbose bool
		want    string
	}{
		{name: "quiet shows notices only", verbose: false, want: "notice 1\n"},
		{name: "verbose shows notices and details", verbose: true, want: "notice 1\ndetail 2\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out strings.Builder
			rep := NewReporter(&out, tt.verbose)
			rep.Notice("notice %d", 1)
			rep.detail("detail %d", 2)
			if out.String() != tt.want {
				t.Fatalf("output = %q, want %q", out.String(), tt.want)
			}
		})
	}
}

// TestReporterEscapesWhatATerminalWouldActOn pins WB-T5 (bugs.md Q-APP-5,
// D-56): text caged writes from paths and errors cannot move the cursor,
// recolour, rewrite or reorder what the user reads. Newlines (one failure per
// line, D-51) and tabs are kept; printable text in any script is kept.
// TestPrintableLineEscapesNewlinesAndTabsToo pins WB-B5 (bugs.md Q-WB-6):
// a table field must stay one field on one row.
func TestPrintableLineEscapesNewlinesAndTabsToo(t *testing.T) {
	t.Parallel()
	if got, want := PrintableLine("a\nb\tc\x1b[2J\r é"), `a\nb\tc\x1b[2J\r é`; got != want {
		t.Fatalf("PrintableLine() = %q, want %q", got, want)
	}
}

func TestReporterEscapesWhatATerminalWouldActOn(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	NewReporter(&out, false).Notice("caged: %v", "read /h/\x1b[2Jx: bad\r\n\tline two \u202egnp.exe é 日本 \x9b\x00\x7f")
	const want = "caged: read /h/\\x1b[2Jx: bad\\r\n\tline two \\u202egnp.exe é 日本 \\x9b\\x00\\x7f\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}
