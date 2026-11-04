package textdiff

import (
	"fmt"
	"strings"
	"testing"
)

func TestIdenticalTextsHaveNoDiff(t *testing.T) {
	t.Parallel()

	r := Compare([]byte("a\nb\n"), []byte("a\nb\n"))
	if !r.empty() || r.Unified("old", "new") != "" {
		t.Errorf("Compare(same) = %+v; want empty", r)
	}
}

func TestUnifiedShowsChangedLinesWithContext(t *testing.T) {
	t.Parallel()

	old := "1\n2\n3\n4\n5\n6\n7\n8\n9\n"
	updated := "1\n2\n3\n4\nFIVE\n6\n7\n8\n9\n"
	r := Compare([]byte(old), []byte(updated))
	if r.Added != 1 || r.Removed != 1 {
		t.Errorf("Added/Removed = %d/%d; want 1/1", r.Added, r.Removed)
	}
	want := "--- approved\n+++ current\n@@ -2,7 +2,7 @@\n 2\n 3\n 4\n-5\n+FIVE\n 6\n 7\n 8\n"
	if got := r.Unified("approved", "current"); got != want {
		t.Errorf("Unified() =\n%s\nwant\n%s", got, want)
	}
}

func TestUnifiedAdditionsAndRemovalsAtTheEnds(t *testing.T) {
	t.Parallel()

	r := Compare([]byte("keep\n"), []byte("keep\nnew1\nnew2\n"))
	got := r.Unified("a", "b")
	if r.Added != 2 || r.Removed != 0 || !strings.Contains(got, "+new1\n+new2\n") {
		t.Errorf("Unified() = %q, %+v; want two added lines", got, r)
	}
	r = Compare([]byte("a\nb\n"), nil)
	if r.Removed != 2 || r.Added != 0 || !strings.Contains(r.Unified("a", "b"), "@@ -1,2 +0,0 @@\n-a\n-b\n") {
		t.Errorf("deleting everything = %q; want both lines removed", r.Unified("a", "b"))
	}
	r = Compare(nil, []byte("x\n"))
	if !strings.Contains(r.Unified("a", "b"), "@@ -0,0 +1 @@\n+x\n") {
		t.Errorf("creating = %q; want x added", r.Unified("a", "b"))
	}
}

func TestDistantChangesMakeSeparateHunks(t *testing.T) {
	t.Parallel()

	var old, updated []string
	for i := 1; i <= 30; i++ {
		old = append(old, "line")
		updated = append(updated, "line")
	}
	updated[1], updated[27] = "first", "second"
	got := Compare([]byte(strings.Join(old, "\n")+"\n"), []byte(strings.Join(updated, "\n")+"\n")).Unified("a", "b")
	if n := strings.Count(got, "@@ -"); n != 2 {
		t.Errorf("got %d hunks; want 2\n%s", n, got)
	}
}

// TestHunkMergeBoundary pins where hunks merge: with three lines of context
// on each side, changes six unchanged lines apart touch and share a hunk,
// seven apart do not.
func TestHunkMergeBoundary(t *testing.T) {
	t.Parallel()

	var old []string
	for i := range 20 {
		old = append(old, fmt.Sprintf("l%02d", i))
	}
	for _, tc := range []struct{ gap, hunks int }{{6, 1}, {7, 2}} {
		t.Run(fmt.Sprintf("gap %d", tc.gap), func(t *testing.T) {
			t.Parallel()
			updated := append([]string(nil), old...)
			updated[3], updated[3+tc.gap+1] = "changed", "changed too"
			got := Compare([]byte(strings.Join(old, "\n")+"\n"), []byte(strings.Join(updated, "\n")+"\n")).Unified("a", "b")
			if n := strings.Count(got, "@@ -"); n != tc.hunks {
				t.Errorf("got %d hunks; want %d\n%s", n, tc.hunks, got)
			}
		})
	}
}

func TestOnlyTheFinalNewlineDiffering(t *testing.T) {
	t.Parallel()

	r := Compare([]byte("a\n"), []byte("a"))
	if r.empty() || !r.finalNewline || !strings.Contains(r.Unified("a", "b"), "newline at the end") {
		t.Errorf("Compare() = %+v; want a final-newline note", r)
	}
}

// TestHugeInputsKeepTheirCommonEnds is bug B-TD-1: past maxCells, the lines
// both texts start and end with are kept, so a small edit to a big file is
// still a small diff, and identical big texts are no diff at all.
func TestHugeInputsKeepTheirCommonEnds(t *testing.T) {
	t.Parallel()

	var old []string
	for i := range 2500 {
		old = append(old, fmt.Sprintf("line %d", i))
	}
	text := strings.Join(old, "\n") + "\n"
	edited := strings.Replace(text, "line 1250\n", "line 1250 edited\n", 1)

	t.Run("one edited line", func(t *testing.T) {
		t.Parallel()
		r := Compare([]byte(text), []byte(edited))
		if got := r.Unified("a", "b"); r.Added != 1 || r.Removed != 1 || strings.Count(got, "@@ -") != 1 || !strings.Contains(got, "@@ -1248,7 +1248,7 @@\n") {
			t.Errorf("Added/Removed = %d/%d\n%s", r.Added, r.Removed, got)
		}
	})
	t.Run("identical", func(t *testing.T) {
		t.Parallel()
		if r := Compare([]byte(text), []byte(text)); !r.empty() || r.Unified("a", "b") != "" {
			t.Errorf("identical big texts: Added/Removed = %d/%d", r.Added, r.Removed)
		}
	})
	t.Run("only the final newline", func(t *testing.T) {
		t.Parallel()
		if r := Compare([]byte(text), []byte(strings.TrimSuffix(text, "\n"))); !r.finalNewline {
			t.Errorf("Added/Removed = %d/%d, finalNewline = false", r.Added, r.Removed)
		}
	})
	t.Run("middle still too big", func(t *testing.T) {
		t.Parallel()
		ends := strings.Repeat("same\n", 10)
		r := Compare([]byte(ends+strings.Repeat("x\n", 2500)+ends), []byte(ends+strings.Repeat("y\n", 2500)+ends))
		if got := r.Unified("a", "b"); r.Added != 2500 || r.Removed != 2500 || !strings.HasPrefix(got, "--- a\n+++ b\n@@ -8,2506 +8,2506 @@\n") {
			t.Errorf("Added/Removed = %d/%d\n%.200s", r.Added, r.Removed, got)
		}
	})
}

func TestHugeInputsFallBackToWholeReplacement(t *testing.T) {
	t.Parallel()

	big := strings.Repeat("x\n", 2500)
	r := Compare([]byte(big), []byte(strings.Repeat("y\n", 2500)))
	if r.Removed != 2500 || r.Added != 2500 {
		t.Errorf("Added/Removed = %d/%d; want 2500/2500", r.Added, r.Removed)
	}
}
