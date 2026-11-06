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

// Width is how many cells s takes. A variation selector or a zero-width joiner
// takes none.
func Width(s string) int { return ansi.StringWidth(s) }
