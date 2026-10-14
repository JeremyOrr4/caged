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
			rep.Detail("detail %d", 2)
			if out.String() != tt.want {
				t.Fatalf("output = %q, want %q", out.String(), tt.want)
			}
		})
	}
}
