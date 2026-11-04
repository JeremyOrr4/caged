package textdiff

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// splitLines splits text into its lines, a final newline ending the last one
// (written independently of lines, which it checks).
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// lcsLength is the length of a longest common subsequence of a and b, from a
// forward two-row table (lcsTable fills a full table backwards).
func lcsLength(a, b []string) int {
	prev, row := make([]int, len(b)+1), make([]int, len(b)+1)
	for i := range a {
		for j := range b {
			if a[i] == b[j] {
				row[j+1] = prev[j] + 1
			} else {
				row[j+1] = max(prev[j+1], row[j])
			}
		}
		prev, row = row, prev
	}
	return prev[len(b)]
}

// spanOf parses one side of a hunk header, "start" or "start,count".
func spanOf(t *testing.T, text string) (start, count int) {
	t.Helper()
	startText, countText, hasCount := strings.Cut(text[1:], ",")
	start, err := strconv.Atoi(startText)
	if err != nil {
		t.Fatalf("bad hunk header part %q", text)
	}
	count = 1
	if hasCount {
		if count, err = strconv.Atoi(countText); err != nil {
			t.Fatalf("bad hunk header part %q", text)
		}
	}
	return start, count
}

// applyUnified applies the hunks of a Unified diff to old and returns the
// result, failing when a hunk does not match old or its header miscounts.
func applyUnified(t *testing.T, diff string, old []string) []string {
	t.Helper()
	var out []string
	next := 0 // index of the next old line not yet copied
	body := strings.Split(strings.TrimSuffix(diff, "\n"), "\n")[2:]
	for i := 0; i < len(body); {
		fields := strings.Fields(body[i])
		if len(fields) != 4 || fields[0] != "@@" {
			t.Fatalf("expected a hunk header, got %q in\n%s", body[i], diff)
		}
		oldStart, oldCount := spanOf(t, fields[1])
		_, newCount := spanOf(t, fields[2])
		if oldCount > 0 {
			oldStart-- // a non-empty range names its first line, an empty one the line before it
		}
		out, next = append(out, old[next:oldStart]...), oldStart
		gotOld, gotNew := 0, 0
		for i++; i < len(body) && !strings.HasPrefix(body[i], "@@"); i++ {
			kind, text := body[i][0], body[i][1:]
			if kind != '+' {
				if next >= len(old) || old[next] != text {
					t.Fatalf("hunk line %q does not match old line %d in\n%s", body[i], next+1, diff)
				}
				next, gotOld = next+1, gotOld+1
			}
			if kind != '-' {
				out, gotNew = append(out, text), gotNew+1
			}
		}
		if gotOld != oldCount || gotNew != newCount {
			t.Fatalf("hunk %s counts %d,%d lines but has %d,%d in\n%s", fields[1]+" "+fields[2], oldCount, newCount, gotOld, gotNew, diff)
		}
	}
	return append(out, old[next:]...)
}

// FuzzCompare checks that a diff is empty exactly for identical texts, is a
// minimal edit script, and renders as a unified diff that turns the old text
// into the updated one.
func FuzzCompare(f *testing.F) {
	for _, seed := range [][2]string{
		{"", ""}, {"a\n", "a\n"}, {"a\nb\nc\n", "a\nB\nc\n"}, {"a", "a\n"}, {"a\nb\n", ""}, {"", "x\ny\n"},
		{"x\nx\nx\n", "x\ny\nx\n"}, {"a\r\nb\n", "a\nb\n"}, {"\n\n\n", "\n"},
		{"0", "\n\n0"}, // lines added before a kept one: a non-minimal script would drop the kept line
		{"1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n", "1\nX\n3\n4\n5\n6\n7\n8\n9\nY\n11\n12\n"},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, old, updated string) {
		// At most 1024 lines a side keeps the table under maxCells, so this
		// checks the main path; FuzzCompareHuge checks the one past it.
		old, updated = old[:min(len(old), 1024)], updated[:min(len(updated), 1024)]
		a, b := splitLines(old), splitLines(updated)
		r := Compare([]byte(old), []byte(updated))
		if r.empty() != (old == updated) {
			t.Fatalf("Compare(%q, %q).empty() = %v", old, updated, r.empty())
		}
		var fromOld, fromNew []string
		kept := 0
		for _, o := range r.ops {
			if o.kind != '+' {
				fromOld = append(fromOld, o.text)
			}
			if o.kind != '-' {
				fromNew = append(fromNew, o.text)
			}
			if o.kind == ' ' {
				kept++
			}
		}
		if !slices.Equal(fromOld, a) || !slices.Equal(fromNew, b) {
			t.Fatalf("Compare(%q, %q): the edit script does not rebuild both texts", old, updated)
		}
		if common := lcsLength(a, b); kept != common || r.Added != len(b)-common || r.Removed != len(a)-common {
			t.Fatalf("Compare(%q, %q): kept %d, Added %d, Removed %d; a minimal script keeps %d", old, updated, kept, r.Added, r.Removed, common)
		}
		if out := r.Unified("a", "b"); !r.empty() && !r.finalNewline {
			if got := applyUnified(t, out, a); !slices.Equal(got, b) {
				t.Fatalf("applying the diff of %q, %q gives %q:\n%s", old, updated, got, out)
			}
		}
	})
}

// FuzzCompareHuge wraps the inputs in 1,100 lines that both texts share at
// the start and at the end, so the table would pass maxCells and Compare takes
// its big-text path. The rendered diff must still turn the old text into the
// updated one, and the edit script must still be minimal: a longest common
// subsequence of P+X+S and P+Y+S is P, S and one of X and Y.
func FuzzCompareHuge(f *testing.F) {
	var ends []string
	for i := range 1100 {
		ends = append(ends, fmt.Sprintf("same %d", i))
	}
	end := strings.Join(ends, "\n") + "\n"
	for _, seed := range [][2]string{{"", ""}, {"a\n", "b\n"}, {"same 1\n", "x\n"}, {"x\ny\n", "y\nx\n"}, {"a", "a\n"}} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, old, updated string) {
		old, updated = old[:min(len(old), 512)], updated[:min(len(updated), 512)]
		// The middle must be whole lines to sit between the ends.
		if old != "" && !strings.HasSuffix(old, "\n") {
			old += "\n"
		}
		if updated != "" && !strings.HasSuffix(updated, "\n") {
			updated += "\n"
		}
		fullOld, fullNew := end+old+end, end+updated+end
		r := Compare([]byte(fullOld), []byte(fullNew))
		kept := 0
		for _, o := range r.ops {
			if o.kind == ' ' {
				kept++
			}
		}
		a, b := splitLines(fullOld), splitLines(fullNew)
		if want := 2*len(ends) + lcsLength(splitLines(old), splitLines(updated)); kept != want || r.Added != len(b)-want || r.Removed != len(a)-want {
			t.Fatalf("Compare(%q -> %q between the ends): kept %d, Added %d, Removed %d; a minimal script keeps %d", old, updated, kept, r.Added, r.Removed, want)
		}
		if r.empty() != (fullOld == fullNew) {
			t.Fatalf("empty() = %v for %q -> %q", r.empty(), old, updated)
		}
		if out := r.Unified("a", "b"); !r.empty() {
			if got := applyUnified(t, out, a); !slices.Equal(got, b) {
				t.Fatalf("applying the diff of %q -> %q does not give the updated text:\n%.400s", old, updated, out)
			}
		}
	})
}
