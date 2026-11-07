package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func line(c *canvas, y int) string {
	return strings.TrimRight(ansi.Strip(strings.Split(c.String(), "\n")[y]), " ")
}

func TestPutStopsAtTheEdge(t *testing.T) {
	c := newCanvas(5, 1)
	if x := c.put(0, 3, "abcdef", style{}); x != 5 {
		t.Errorf("x is %d", x)
	}
	if got := line(c, 0); got != "   ab" {
		t.Errorf("got %q", got)
	}
}

func TestWideCharacterDoesNotHangOverTheEdge(t *testing.T) {
	c := newCanvas(5, 1)
	c.put(0, 3, "ab日本", style{})
	if got := line(c, 0); got != "   ab" {
		t.Errorf("got %q", got)
	}
	c = newCanvas(4, 1)
	c.put(0, 3, "日", style{})
	if got := line(c, 0); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestWritingOverHalfAWideCharacter(t *testing.T) {
	for _, tt := range []struct {
		x    int
		s    string
		want string
	}{
		{0, "x", "x 本"},  // over the first half
		{1, "x", " x本"},  // over the second half
		{2, "x", "日x"},   // over the first half of the next
		{0, "xa", "xa本"}, // over both halves
	} {
		c := newCanvas(6, 1)
		c.put(0, 0, "日本", style{})
		c.put(0, tt.x, tt.s, style{})
		if got := line(c, 0); got != tt.want {
			t.Errorf("%q at %d: got %q, want %q", tt.s, tt.x, got, tt.want)
		}
	}
}

func TestZeroWidthCharactersStayOnTheirCell(t *testing.T) {
	c := newCanvas(10, 1)
	x := c.put(0, 0, "\u2764\ufe0fa", style{})
	if x != 3 {
		t.Errorf("x is %d", x)
	}
	if got := line(c, 0); got != "\u2764\ufe0fa" {
		t.Errorf("got %q", got)
	}
}

func TestPutCut(t *testing.T) {
	for _, tt := range []struct {
		segs []seg
		w    int
		want string
	}{
		{[]seg{{text: "abc"}}, 5, "abc"},
		{[]seg{{text: "abc"}}, 3, "abc"},
		{[]seg{{text: "abcd"}}, 3, "ab…"},
		{[]seg{{text: "ab"}, {text: "cd"}}, 3, "ab…"},
		{[]seg{{text: "a"}, {text: "bcd"}}, 3, "ab…"},
		{[]seg{{text: "日本語"}}, 4, "日…"},
		{[]seg{{text: "日本語"}}, 5, "日本…"},
		{[]seg{{text: "abc"}}, 1, "…"},
		{[]seg{{text: "abc"}}, 0, ""},
	} {
		c := newCanvas(10, 1)
		c.putCut(0, 0, tt.segs, tt.w, style{})
		if got := line(c, 0); got != tt.want {
			t.Errorf("%v in %d: got %q, want %q", tt.segs, tt.w, got, tt.want)
		}
	}
}

func TestBoxOverSomething(t *testing.T) {
	for _, x := range []int{0, 1, 2, 3, 4} {
		c := newCanvas(8, 3)
		for y := range 3 {
			c.put(y, 0, "日本語ab", style{})
		}
		c.box(x, 0, 4, 3, nil, nil)
		for y, l := range strings.Split(c.String(), "\n") {
			if w := ansi.StringWidth(l); w != 8 {
				t.Errorf("a box at %d: row %d is %d wide: %q", x, y, w, l)
			}
		}
	}
}
