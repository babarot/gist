// Package diff says what changed between two versions of a file, with three
// lines of context around each change.
package diff

import (
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// Kind is what a line of a diff is.
type Kind int

// The kinds.
const (
	Same Kind = iota
	Added
	Removed
	Hunk
)

// Line is a line of a diff.
type Line struct {
	Kind Kind
	Text string
}

// Diff is what changed.
type Diff struct {
	Lines []Line
	// Added and Removed count the lines.
	Added, Removed int
}

const context = 3

// noNewline marks a last line without a newline, which the comparison needs to
// see and which could not show otherwise.
const noNewline = "\x00"

// lines splits s, each line with its newline. difflib.SplitLines adds an empty
// line after a final newline, and writes a line without one into the next.
func lines(s string) []string {
	out := strings.SplitAfter(s, "\n")
	last := len(out) - 1
	if out[last] == "" {
		return out[:last]
	}
	out[last] += noNewline + "\n"
	return out
}

// Compare says what changed from before to after.
func Compare(before, after string) Diff {
	s, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        lines(before),
		B:        lines(after),
		FromFile: "a",
		ToFile:   "b",
		Context:  context,
	})
	if err != nil {
		// It only fails writing to memory, which does not fail
		return Diff{}
	}
	var d Diff
	out := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	// The first two are the names of the files
	for _, l := range out[min(2, len(out)):] {
		if l == "" {
			continue
		}
		l = strings.ReplaceAll(l, noNewline, " (no newline at end of file)")
		switch l[0] {
		case '@':
			d.Lines = append(d.Lines, Line{Hunk, strings.TrimSpace(l)})
		case '+':
			d.Added++
			d.Lines = append(d.Lines, Line{Added, l[1:]})
		case '-':
			d.Removed++
			d.Lines = append(d.Lines, Line{Removed, l[1:]})
		default:
			d.Lines = append(d.Lines, Line{Same, l[1:]})
		}
	}
	return d
}
