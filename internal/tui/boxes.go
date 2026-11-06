package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/diff"
	"github.com/babarot/gist/v2/internal/text"
	"github.com/babarot/gist/v2/internal/tui/keymap"
)

// boxAt is where a box of width w and height h goes.
func (m *Model) boxAt(w, h, y int) (int, int) {
	return max(0, (m.width-w)/2), min(y, max(0, m.height-h))
}

// keyLine writes keys and what they do, from x.
func (m *Model) keyLine(c *canvas, y, x int, hints []hint) {
	p := m.pal
	for i, h := range hints {
		if i > 0 {
			x = c.put(y, x, " · ", style{fg: p.overlay})
		}
		x = c.put(y, x, h.key, style{fg: p.blue, bold: true})
		x = c.put(y, x, " "+h.what, style{fg: p.overlay})
	}
}

func title(c *canvas, x, y int, s string, st style) {
	c.put(y, x+1, " "+s+" ", st)
}

func (m *Model) drawDiff(c *canvas) {
	p := m.pal
	e := m.editing
	x, y, w, h := 4, 2, c.w-8, c.h-5
	if w < 10 || h < 5 {
		return
	}
	c.box(x, y, w, h, p.blue, nil)
	title(c, x, y, "push "+e.file+" to "+shortID(e.gist.ID), style{fg: p.subtext1, bold: true})
	added, removed := fmt.Sprintf("+%d", e.diff.Added), fmt.Sprintf("−%d", e.diff.Removed)
	sx := x + w - 3 - text.Width(added) - text.Width(removed) - 1
	sx = c.put(y, sx, " ", style{fg: p.blue})
	sx = c.put(y, sx, added, style{fg: p.green})
	sx = c.put(y, sx, " ", style{fg: p.blue})
	sx = c.put(y, sx, removed, style{fg: p.red})
	c.put(y, sx, " ", style{fg: p.blue})
	rows := m.diffRows()
	for i := range rows {
		n := e.scroll + i
		if n >= len(e.diff.Lines) {
			break
		}
		l := e.diff.Lines[n]
		st, mark := style{fg: p.subtext0}, "  "
		switch l.Kind {
		case diff.Hunk:
			st, mark = style{fg: p.teal}, ""
		case diff.Added:
			st, mark = style{fg: p.green}, "+ "
		case diff.Removed:
			st, mark = style{fg: p.red}, "- "
		}
		c.putCut(y+1+i, x+2, []seg{{mark + l.Text, st}}, w-4, style{fg: p.overlay})
	}
	fy := y + h - 2
	fx := c.put(fy, x+2, "push this change to GitHub? ", style{fg: p.text})
	m.keyLine(c, fy, fx, []hint{{m.keys.Hint(keymap.Confirm), "push"}, {m.keys.Hint(keymap.EditAgain), "edit again"}, {keymap.Back, "keep as draft"}})
}

func (m *Model) drawConflict(c *canvas) {
	p := m.pal
	e := m.editing
	w, h := min(76, c.w-8), 11
	x, y := m.boxAt(w, h, 5)
	c.box(x, y, w, h, p.yellow, nil)
	title(c, x, y, "GitHub side was updated", style{fg: p.yellow, bold: true})
	lx, iw := x+3, w-6
	cur := e.conflict.Current
	f := e.session.File(e.file)
	c.putCut(y+2, lx, []seg{{e.file + "  ", style{fg: p.text, bold: true}},
		{shortID(cur.ID) + " was updated " + when(m.now().Sub(cur.Updated)) + " after this edit started.", style{fg: p.subtext0}}}, iw, style{fg: p.overlay})
	c.putCut(y+3, lx, []seg{{"your edit is based on revision " + shortID(f.Revision) + "; GitHub is at revision " + shortID(cur.Revision()) + ".",
		style{fg: p.subtext0}}}, iw, style{fg: p.overlay})
	key := style{fg: p.blue, bold: true}
	body := style{fg: p.text}
	c.putCut(y+5, lx, []seg{{m.keys.Hint(keymap.Overwrite), key}, {" overwrites with your edit; the replaced one stays as a revision.", body}}, iw, style{fg: p.overlay})
	c.putCut(y+6, lx, []seg{{m.keys.Hint(keymap.Reload), key}, {" reads the new revision and shows your edit as a diff against it.", body}}, iw, style{fg: p.overlay})
	m.keyLine(c, y+h-2, lx, []hint{{m.keys.Hint(keymap.Overwrite), "overwrite"}, {m.keys.Hint(keymap.Reload), "reload"}, {keymap.Back, "keep as draft"}})
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func (m *Model) drawDelete(c *canvas) {
	p := m.pal
	d := m.deleting
	g := d.gist
	w, h := min(72, c.w-8), 11
	x, y := m.boxAt(w, h, 5)
	c.box(x, y, w, h, p.red, nil)
	title(c, x, y, "delete gist", style{fg: p.red, bold: true})
	lx, iw := x+3, w-6
	names := make([]string, len(g.Files))
	for i, f := range g.Files {
		names[i] = f.Name
	}
	c.putCut(y+2, lx, []seg{{shortID(g.ID) + " ", style{fg: p.overlay}}, {strings.Join(names, ", "), style{fg: p.text, bold: true}}}, iw, style{fg: p.overlay})
	if g.Description != "" {
		c.putCut(y+3, lx, []seg{{g.Description, style{fg: p.subtext0}}}, iw, style{fg: p.overlay})
	} else {
		c.put(y+3, lx, "(no description)", style{fg: p.dim})
	}
	revisions := "its revisions"
	if cg, ok := m.content(g); ok {
		n := len(cg.History)
		revisions = fmt.Sprintf("its %d %s", n, plural(n, "revision", "revisions"))
	}
	c.putCut(y+5, lx, []seg{{"This deletes the gist and " + revisions + " on GitHub.", style{fg: p.text}}}, iw, style{fg: p.overlay})
	c.put(y+6, lx, "It cannot be undone.", style{fg: p.red})
	fy := y + h - 2
	if len(g.Files) > 1 {
		rest := len(g.Files) - 1
		c.putCut(y+7, lx, []seg{{fmt.Sprintf("%s removes only %s. The other %d %s.",
			m.keys.Hint(keymap.OnlyFile), d.file, rest, plural(rest, "file stays", "files stay")), style{fg: p.subtext0}}}, iw, style{fg: p.overlay})
		c.put(y+8, lx, "Older revisions keep this file.", style{fg: p.subtext0})
		m.keyLine(c, fy, lx, []hint{{m.keys.Hint(keymap.Confirm), "delete the gist"}, {m.keys.Hint(keymap.OnlyFile), "only this file"}, {keymap.Back, "no"}})
		return
	}
	fx := c.put(fy, lx, "delete this gist? ", style{fg: p.text})
	m.keyLine(c, fy, fx, []hint{{m.keys.Hint(keymap.Confirm), "yes"}, {keymap.Back, "no"}})
}

func (m *Model) drawDiscard(c *canvas) {
	p := m.pal
	d := m.discarding
	w, h := min(64, c.w-8), 8
	x, y := m.boxAt(w, h, 6)
	c.box(x, y, w, h, p.red, nil)
	title(c, x, y, "discard the draft", style{fg: p.red, bold: true})
	lx := x + 3
	c.putCut(y+2, lx, []seg{{d.file + "  ", style{fg: p.text, bold: true}}, {"of " + shortID(d.gist), style{fg: p.overlay}}}, w-6, style{fg: p.overlay})
	c.put(y+3, lx, "The edit that was not pushed goes away.", style{fg: p.text})
	m.keyLine(c, y+h-2, lx, []hint{{m.keys.Hint(keymap.Confirm), "discard"}, {keymap.Back, "keep"}})
}

const (
	inputRow    = 2
	inputText   = 5 // where the text starts, in the box
	inputMargin = 8 // what the box takes of its width around the text
)

func (m *Model) inputBox() (x, y, w, h int) {
	w, h = min(80, m.width-8), 8
	x, y = m.boxAt(w, h, 6)
	return x, y, w, h
}

func (m *Model) drawInput(c *canvas) {
	p := m.pal
	in := m.input
	x, y, w, h := m.inputBox()
	c.box(x, y, w, h, p.blue, nil)
	name := "description of " + shortID(in.gist.ID)
	note := "checked against the description on GitHub before saving"
	if in.kind == inputFilename {
		name, note = "rename "+in.file, "a rename makes a new revision"
	}
	title(c, x, y, name, style{fg: p.subtext1, bold: true})
	lx := x + 3
	c.put(y+inputRow, lx, "┃", style{fg: p.blue})
	shown, _ := inputView(in.box.Value(), in.box.Position(), w-inputMargin)
	c.putCut(y+inputRow, x+inputText, []seg{{shown, style{fg: p.text}}}, w-inputMargin, style{fg: p.overlay})
	if in.err != "" {
		c.put(y+4, lx, "✗ "+in.err, style{fg: p.red})
	} else {
		c.put(y+4, lx, note, style{fg: p.dim})
	}
	m.keyLine(c, y+h-2, lx, []hint{{"enter", "save"}, {keymap.Back, "cancel"}})
}

// inputView is what shows of a text box in width cells, and the cell its cursor
// is on. The box counts in characters, and a Japanese character takes two
// cells, so neither can be taken from it. What is before the cursor goes when
// there is no room, so that the cursor stays in sight.
func inputView(value string, pos, width int) (string, int) {
	runes := []rune(value)
	pos = min(max(0, pos), len(runes))
	start := 0
	for start < pos && text.Width(text.Line(string(runes[start:pos]))) > width-1 {
		start++
	}
	return string(runes[start:]), text.Width(text.Line(string(runes[start:pos])))
}

// boxCursor is the terminal's cursor for a text box drawn at x, y in width cells.
func boxCursor(box textinput.Model, x, y, width int) *tea.Cursor {
	cur := box.Cursor()
	if cur == nil {
		return nil
	}
	_, at := inputView(box.Value(), box.Position(), width)
	cur.X, cur.Y = x+at, y
	return cur
}

// cursor is where the terminal's cursor goes: in the text box, when one is open.
func (m *Model) cursor() *tea.Cursor {
	switch m.uiState() {
	case uiInput:
		x, y, w, _ := m.inputBox()
		return boxCursor(m.input.box, x+inputText, y+inputRow, w-inputMargin)
	case uiFilter:
		return boxCursor(m.filtering.box, filterText, filterRow, m.width-filterText)
	case uiNew:
		return m.formCursor()
	}
	return nil
}

func (m *Model) drawInputConflict(c *canvas) {
	p := m.pal
	in := m.input
	w, h := min(76, c.w-8), 10
	x, y := m.boxAt(w, h, 5)
	c.box(x, y, w, h, p.yellow, nil)
	title(c, x, y, "GitHub side was updated", style{fg: p.yellow, bold: true})
	lx, iw := x+3, w-6
	cur := in.conflict.Current
	if in.kind == inputDescription {
		c.put(y+2, lx, "the description on GitHub is now:", style{fg: p.subtext0})
		c.putCut(y+3, lx+2, []seg{{cur.Description, style{fg: p.text}}}, iw-2, style{fg: p.overlay})
		c.put(y+5, lx, "a description has no history: overwriting loses this one.", style{fg: p.yellow})
	} else {
		c.putCut(y+2, lx, []seg{{in.file + "  ", style{fg: p.text, bold: true}},
			{shortID(cur.ID) + " was updated " + when(m.now().Sub(cur.Updated)) + ".", style{fg: p.subtext0}}}, iw, style{fg: p.overlay})
	}
	m.keyLine(c, y+h-2, lx, []hint{{m.keys.Hint(keymap.Overwrite), "overwrite"}, {m.keys.Hint(keymap.Reload), "reload"}, {keymap.Back, "cancel"}})
}
