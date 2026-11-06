// Package text makes text from GitHub fit to draw. Anyone can write a starred
// gist or a comment, and an escape sequence reaching the terminal could move
// the cursor or change the title.
//
// This is for what is drawn only, which is why just the TUI and the Markdown
// renderer may import it. Anywhere on the way to editing or printing it would
// change a gist's content without showing in a diff.
package text

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
)

// Clean removes escape sequences and control characters from s, and writes a
// tab as four spaces. Newlines stay.
func Clean(s string) string {
	s = strings.ReplaceAll(ansi.Strip(s), "\t", "    ")
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			return -1
		}
		return r
	}, s)
}

// Line is Clean for what is drawn on one line.
func Line(s string) string { return strings.ReplaceAll(Clean(s), "\n", " ") }

// Breaks are the byte offsets the rows of s start at when it is wrapped to width
// cells, the first being 0. A row ends after its last space when it has one, and
// anywhere otherwise, as Japanese has none.
func Breaks(s string, width int) []int {
	width = max(1, width)
	starts := []int{0}
	start, cells, space, at := 0, 0, -1, 0
	state := -1
	for rest := s; rest != ""; {
		var cluster string
		cluster, rest, _, state = uniseg.FirstGraphemeClusterInString(rest, state)
		w := Width(cluster)
		for cells+w > width && at > start {
			start = at
			if space > starts[len(starts)-1] {
				start = space
			}
			starts = append(starts, start)
			cells, space = Width(s[start:at]), -1
		}
		cells += w
		at += len(cluster)
		if cluster == " " {
			space = at
		}
	}
	return starts
}

// Width is how many cells s takes. A variation selector or a zero-width joiner
// takes none.
func Width(s string) int { return ansi.StringWidth(s) }
