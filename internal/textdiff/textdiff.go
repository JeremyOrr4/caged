// Package textdiff renders a unified line diff of two texts. It does no IO.
package textdiff

import (
	"fmt"
	"slices"
	"strings"
)

const (
	contextLines = 3
	// maxCells bounds the line-by-line comparison table (old lines times
	// updated lines). Past it, only the lines between the texts' common start
	// and end are compared, and replaced whole when they are still too many.
	maxCells = 4_000_000
)

type op struct {
	kind byte // ' ' kept, '-' removed, '+' added
	text string
}

// Result is a computed diff.
type Result struct {
	ops          []op
	Added        int  // lines only in the updated text
	Removed      int  // lines only in the old text
	finalNewline bool // the texts differ only in their final newline
}

// Compare diffs old against updated.
func Compare(old, updated []byte) Result {
	a, b := lines(old), lines(updated)
	r := Result{ops: diff(a, b)}
	for _, o := range r.ops {
		switch o.kind {
		case '+':
			r.Added++
		case '-':
			r.Removed++
		}
	}
	r.finalNewline = r.Added == 0 && r.Removed == 0 && string(old) != string(updated)
	return r
}

// empty reports whether the texts are identical.
func (r Result) empty() bool { return r.Added == 0 && r.Removed == 0 && !r.finalNewline }

// Unified renders the diff in unified format, labelled with the two names. An
// empty Result renders as "".
func (r Result) Unified(oldName, updatedName string) string {
	if r.empty() {
		return ""
	}
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n+++ %s\n", oldName, updatedName)
	if r.finalNewline {
		out.WriteString("(only the newline at the end of the file differs)\n")
		return out.String()
	}
	oldBefore, newBefore, counted := 0, 0, 0 // lines of each text before ops[counted]
	for _, h := range r.hunks() {
		gapOld, gapNew := lineCounts(r.ops[counted:h.from])
		oldBefore, newBefore = oldBefore+gapOld, newBefore+gapNew
		oldCount, newCount := lineCounts(r.ops[h.from:h.to])
		fmt.Fprintf(&out, "@@ -%s +%s @@\n", span(oldBefore+1, oldCount), span(newBefore+1, newCount))
		oldBefore, newBefore, counted = oldBefore+oldCount, newBefore+newCount, h.to
		for _, o := range r.ops[h.from:h.to] {
			out.WriteByte(o.kind)
			out.WriteString(o.text)
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func span(start, count int) string {
	if count == 0 { // an empty range is named by the line before it
		return fmt.Sprintf("%d,0", start-1)
	}
	if count == 1 {
		return fmt.Sprint(start)
	}
	return fmt.Sprintf("%d,%d", start, count)
}

// hunk is a range of ops: changes with their context lines.
type hunk struct{ from, to int }

// hunks puts contextLines of unchanged lines around every change, and joins
// changes whose context touches or overlaps into one hunk.
func (r Result) hunks() []hunk {
	var out []hunk
	for i, o := range r.ops {
		if o.kind == ' ' {
			continue
		}
		h := hunk{from: max(i-contextLines, 0), to: min(i+contextLines+1, len(r.ops))}
		if last := len(out) - 1; last >= 0 && h.from <= out[last].to {
			out[last].to = h.to
		} else {
			out = append(out, h)
		}
	}
	return out
}

// lineCounts returns how many old and updated lines ops cover.
func lineCounts(ops []op) (oldLines, updatedLines int) {
	for _, o := range ops {
		if o.kind != '+' {
			oldLines++
		}
		if o.kind != '-' {
			updatedLines++
		}
	}
	return oldLines, updatedLines
}

func lines(data []byte) []string {
	text := string(data)
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// diff returns the edit script from a to b, from a longest common subsequence.
// Texts too big for the comparison table keep the lines they both start and
// end with, and only the lines between them are compared, or replaced whole
// when those are still too many.
func diff(a, b []string) []op {
	if len(a) == 0 || len(b) == 0 {
		return replaceAll(a, b) // no table needed: one side is all removed or all added
	}
	if len(a)*len(b) <= maxCells {
		return lcsDiff(a, b)
	}
	head := commonPrefix(a, b)
	tail := commonSuffix(a[head:], b[head:])
	midA, midB := a[head:len(a)-tail], b[head:len(b)-tail]
	var middle []op
	if len(midA) > 0 && len(midB) > 0 && len(midA)*len(midB) <= maxCells {
		middle = lcsDiff(midA, midB)
	} else {
		middle = replaceAll(midA, midB)
	}
	return slices.Concat(opsFor(' ', a[:head]), middle, opsFor(' ', a[len(a)-tail:]))
}

// lcsDiff returns the edit script from a to b, from a longest common
// subsequence of the whole texts.
func lcsDiff(a, b []string) []op {
	lcs := lcsTable(a, b)
	var ops []op
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			ops = append(ops, op{' ', a[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, op{'-', a[i]})
			i++
		default:
			ops = append(ops, op{'+', b[j]})
			j++
		}
	}
	return append(ops, replaceAll(a[i:], b[j:])...)
}

// commonPrefix returns how many lines a and b start with in common.
func commonPrefix(a, b []string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// commonSuffix returns how many lines a and b end with in common.
func commonSuffix(a, b []string) int {
	n := 0
	for n < len(a) && n < len(b) && a[len(a)-1-n] == b[len(b)-1-n] {
		n++
	}
	return n
}

// opsFor is the edit script that applies kind (keep, remove, add) to every line.
func opsFor(kind byte, lines []string) []op {
	ops := make([]op, 0, len(lines))
	for _, line := range lines {
		ops = append(ops, op{kind, line})
	}
	return ops
}

// replaceAll is the edit script that removes every line of a, then adds every
// line of b.
func replaceAll(a, b []string) []op {
	return append(opsFor('-', a), opsFor('+', b)...)
}

// lcsTable returns lcs, where lcs[i][j] is the length of the longest common
// subsequence of a[i:] and b[j:].
func lcsTable(a, b []string) [][]int {
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i, v := range slices.Backward(a) {
		for j := len(b) - 1; j >= 0; j-- {
			if v == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	return lcs
}
