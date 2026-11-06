package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/rivo/uniseg"

	"github.com/babarot/gist/v2/internal/text"
)

// style is how a cell looks. A nil color is the terminal's own.
type style struct {
	fg, bg color.Color
	bold   bool
}

// cell is a cell of the screen. A wide character takes two: its text is in the
// first, and the second is empty with width 0.
type cell struct {
	text  string
	width int
	style style
}

// canvas is the screen as a grid of cells. Everything is drawn at a place on
// it, so a line cannot come out wider than the terminal, whatever is written,
// what is drawn over the screen needs no other way of drawing, and a click is
// looked up in the same places things were drawn at.
//
// Everything put on it is cleaned, so text from GitHub cannot reach the
// terminal as an escape sequence.
type canvas struct {
	w, h int
	rows [][]cell
}

func newCanvas(w, h int) *canvas {
	c := &canvas{w: w, h: h, rows: make([][]cell, h)}
	for y := range c.rows {
		c.rows[y] = make([]cell, w)
		c.clear(y, 0, w, nil)
	}
	return c
}

// put writes s from x on row y, up to the right edge, and returns the x after
// it. A style with no background leaves the one that is there.
func (c *canvas) put(y, x int, s string, st style) int {
	if y < 0 || y >= c.h {
		return x
	}
	s = text.Line(s)
	state := -1
	for s != "" {
		var cluster string
		cluster, s, _, state = uniseg.FirstGraphemeClusterInString(s, state)
		w := text.Width(cluster)
		if w == 0 {
			continue
		}
		if x+w > c.w {
			break
		}
		if x >= 0 {
			c.set(y, x, cluster, w, st)
		}
		x += w
	}
	return x
}

func (c *canvas) set(y, x int, s string, w int, st style) {
	row := c.rows[y]
	c.breakWide(y, x)
	if w == 2 {
		c.breakWide(y, x+1)
	}
	if st.bg == nil {
		st.bg = row[x].style.bg
	}
	row[x] = cell{text: s, width: w, style: st}
	if w == 2 {
		row[x+1] = cell{style: st}
	}
}

// breakWide blanks the wide character whose half is at x, about to be written
// over.
func (c *canvas) breakWide(y, x int) {
	row := c.rows[y]
	switch {
	case row[x].width == 0 && x > 0:
		row[x-1] = cell{text: " ", width: 1, style: row[x-1].style}
	case row[x].width == 2 && x+1 < c.w:
		row[x+1] = cell{text: " ", width: 1, style: row[x+1].style}
	}
}

// clear blanks the cells from x0 up to x1 on row y, in background bg.
func (c *canvas) clear(y, x0, x1 int, bg color.Color) {
	x0, x1 = max(0, x0), min(c.w, x1)
	for x := x0; x < x1; x++ {
		if c.rows[y][x].width != 0 || c.rows[y][x].text != "" {
			c.breakWide(y, x)
		}
		c.rows[y][x] = cell{text: " ", width: 1, style: style{bg: bg}}
	}
}

// fill sets the background of the cells from x0 up to x1 on row y.
func (c *canvas) fill(y, x0, x1 int, bg color.Color) {
	if y < 0 || y >= c.h {
		return
	}
	for x := max(0, x0); x < min(c.w, x1); x++ {
		c.rows[y][x].style.bg = bg
	}
}

// seg is a run of text in one style.
type seg struct {
	text string
	st   style
}

// putCut writes segs from x in at most w cells, ending in more when they do not
// fit, and returns the x after them.
func (c *canvas) putCut(y, x int, segs []seg, w int, more style) int {
	total := 0
	for _, s := range segs {
		total += text.Width(text.Line(s.text))
	}
	if total <= w {
		for _, s := range segs {
			x = c.put(y, x, s.text, s.st)
		}
		return x
	}
	if w <= 0 {
		return x
	}
	end := x + w - 1
	for _, s := range segs {
		var fit strings.Builder
		rest, state, cx := text.Line(s.text), -1, x
		for rest != "" {
			var cluster string
			cluster, rest, _, state = uniseg.FirstGraphemeClusterInString(rest, state)
			n := text.Width(cluster)
			if cx+n > end {
				break
			}
			fit.WriteString(cluster)
			cx += n
		}
		x = c.put(y, x, fit.String(), s.st)
		if x >= end || cx < x {
			break
		}
	}
	return c.put(y, min(x, end), "…", more)
}

// box draws a rounded frame.
func (c *canvas) box(x, y, w, h int, border, bg color.Color) {
	line := style{fg: border, bg: bg}
	for yy := y; yy < y+h && yy < c.h; yy++ {
		c.clear(yy, x, x+w, bg)
	}
	inner := max(0, w-2)
	c.put(y, x, "╭"+strings.Repeat("─", inner)+"╮", line)
	for yy := y + 1; yy < y+h-1; yy++ {
		c.put(yy, x, "│", line)
		c.put(yy, x+w-1, "│", line)
	}
	c.put(y+h-1, x, "╰"+strings.Repeat("─", inner)+"╯", line)
}

// String is the screen, as the terminal is to draw it.
func (c *canvas) String() string {
	var b strings.Builder
	for y, row := range c.rows {
		if y > 0 {
			b.WriteByte('\n')
		}
		end := len(row)
		for end > 0 && row[end-1].text == " " && row[end-1].style == (style{}) {
			end--
		}
		var run strings.Builder
		var runStyle style
		flush := func() {
			if run.Len() > 0 {
				b.WriteString(render(runStyle, run.String()))
				run.Reset()
			}
		}
		for _, cl := range row[:end] {
			if cl.style != runStyle {
				flush()
				runStyle = cl.style
			}
			run.WriteString(cl.text)
		}
		flush()
	}
	return b.String()
}

func render(st style, s string) string {
	if st == (style{}) {
		return s
	}
	l := lipgloss.NewStyle().Bold(st.bold)
	if st.fg != nil {
		l = l.Foreground(st.fg)
	}
	if st.bg != nil {
		l = l.Background(st.bg)
	}
	return l.Render(s)
}
