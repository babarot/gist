package tui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/highlight"
	"github.com/babarot/gist/v2/internal/markdown"
	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/text"
	"github.com/babarot/gist/v2/internal/tui/keymap"
)

const (
	// infoWidth is the width of the info pane.
	infoWidth = 30
	// commentsShare is how much of the width the comments pane takes, in percent.
	commentsShare = 46
	// maxLines is the most lines of a file shown.
	maxLines = 1000
	// lineNumbers is the width of the line numbers and the gap after them.
	lineNumbers = 6
	// minContent is the narrowest the content goes before the pane is left out.
	minContent = 20
)

// paneX is where the line between the content and the pane is, or the width of
// the screen when there is no room for a pane.
func (m *Model) paneX() int {
	w := infoWidth
	if m.side == sideComments {
		w = max(infoWidth, (m.width-4)*commentsShare/100)
	}
	if dx := m.width - 1 - w - 2; dx >= minContent {
		return dx
	}
	return m.width
}

func isMarkdown(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".md" || ext == ".markdown"
}

// onMarkdown says the file shown of g is one drawn as Markdown.
func (m *Model) onMarkdown(g model.Gist) bool {
	return len(g.Files) > 0 && isMarkdown(g.Files[m.selectedFile(g)].Name)
}

// shownFile is the file shown of g, read whole when it has been.
func (m *Model) shownFile(g model.Gist) (model.File, bool) {
	if len(g.Files) == 0 {
		return model.File{}, false
	}
	i := m.selectedFile(g)
	name := g.Files[i].Name
	if c, ok := m.content(g); ok {
		if j := slices.IndexFunc(c.Files, func(f model.File) bool { return f.Name == name }); j >= 0 {
			return c.Files[j], true
		}
	}
	return g.Files[i], false
}

// contentRows is how many lines the file takes as it is shown.
func (m *Model) contentRows(g model.Gist, f model.File) int {
	if m.showsRendered(f) {
		return len(m.rendered(g, f, m.contentWidth()))
	}
	return min(len(m.highlighted(g, f)), maxLines)
}

func (m *Model) showsRendered(f model.File) bool { return isMarkdown(f.Name) && !m.plain }

// contentWidth is the width the content is drawn in.
func (m *Model) contentWidth() int { return min(m.paneX(), m.width-1) - 4 }

// contentTime is when the content read of g was made, which can be later than
// the list says, so that what is drawn from it goes when it does.
func (m *Model) contentTime(g model.Gist) time.Time {
	if c, ok := m.content(g); ok {
		return c.Updated
	}
	return g.Updated
}

func (m *Model) rendered(g model.Gist, f model.File, width int) []markdown.Line {
	k := drawnKey{fileKey{g.ID, f.Name}, m.contentTime(g), width}
	lines, ok := m.preview.md[k]
	if !ok {
		if len(m.preview.md) >= maxDrawn {
			clear(m.preview.md)
		}
		lines = markdown.Render(firstLines(f.Content, maxLines), width)
		m.preview.md[k] = lines
	}
	return lines
}

// language is the language of a file, which telling can take reading all of it.
func (m *Model) language(g model.Gist, f model.File) string {
	k := fileKey{g.ID, f.Name}
	at := m.contentTime(g)
	l, ok := m.preview.langs[k]
	if !ok || !l.updated.Equal(at) {
		l = fileLang{at, highlight.Language(f.Name, firstLines(f.Content, maxLines))}
		m.preview.langs[k] = l
	}
	return l.name
}

func (m *Model) highlighted(g model.Gist, f model.File) [][]highlight.Token {
	k := fileKey{g.ID, f.Name}
	at := m.contentTime(g)
	c, ok := m.preview.code[k]
	if !ok || !c.updated.Equal(at) {
		if len(m.preview.code) >= maxDrawn {
			clear(m.preview.code)
		}
		c = codeLines{at, highlight.Lines(f.Name, firstLines(f.Content, maxLines))}
		m.preview.code[k] = c
	}
	return c.lines
}

// firstLines is the first n lines of s: more are not shown, and a large file
// would take long to highlight.
func firstLines(s string, n int) string {
	at := 0
	for range n {
		i := strings.IndexByte(s[at:], '\n')
		if i < 0 {
			return s
		}
		at += i + 1
	}
	return s[:at]
}

// lineCount is how many lines s has.
func lineCount(s string) int {
	n := strings.Count(s, "\n")
	if s != "" && !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

func (m *Model) innerRows() int { return max(0, m.layout().boxH-2) }

// scroll moves what J and K move: the comments when they are open, else the
// content.
func (m *Model) scroll(by int) {
	if m.side == sideComments {
		m.scrollComments(by)
		return
	}
	m.scrollContent(by)
}

func (m *Model) scrollContent(by int) {
	g, ok := m.selected()
	if !ok {
		return
	}
	f, read := m.shownFile(g)
	if !read || f.Binary {
		return
	}
	k := fileKey{g.ID, f.Name}
	limit := max(0, m.contentRows(g, f)-m.innerRows())
	m.preview.scroll[k] = max(0, min(m.preview.scroll[k]+by, limit))
}

func (m *Model) scrollComments(by int) {
	g, ok := m.selected()
	if !ok {
		return
	}
	limit := max(0, len(m.commentLines(g))-m.innerRows())
	m.preview.commentScroll[g.ID] = max(0, min(m.preview.commentScroll[g.ID]+by, limit))
}

func (m *Model) drawPreview(c *canvas, l layout) {
	p := m.pal
	if l.boxH < 2 {
		return
	}
	c.box(0, l.boxY, c.w, l.boxH, p.surface1, nil)
	g, ok := m.selected()
	if !ok {
		c.put(l.boxY, 2, " preview ", style{fg: p.subtext1, bold: true})
		return
	}
	dx := m.paneX()
	m.drawFileTabs(c, l, g, dx)
	f, read := m.shownFile(g)
	m.drawContent(c, l, g, f, read, min(dx, c.w-1))
	if dx < c.w {
		line := style{fg: p.surface1}
		for y := l.boxY + 1; y < l.boxY+l.boxH-1; y++ {
			c.put(y, dx, "│", line)
		}
		c.put(l.boxY, dx, "┬", line)
		c.put(l.boxY+l.boxH-1, dx, "┴", line)
		m.drawSideTabs(c, l, g, dx)
		if m.side == sideComments {
			m.drawComments(c, l, g, dx+2)
		} else {
			m.drawInfo(c, l, g, f, read, dx+2)
		}
	}
	// Last, as they can run longer than the pane they end under
	m.drawHints(c, l, g, f, read)
}

// drawHints says along the bottom of the frame what the keys do here.
func (m *Model) drawHints(c *canvas, l layout, g model.Gist, f model.File, read bool) {
	p := m.pal
	y := l.boxY + l.boxH - 1
	hints := []string{g.ID[:min(7, len(g.ID))]}
	if len(g.Files) > 1 {
		hints = append(hints, m.keys.Hint(keymap.PrevFile, keymap.NextFile)+" file")
	}
	var left string
	if read && !f.Binary {
		if m.side == sideInfo && m.contentRows(g, f) > m.innerRows() {
			hints = append(hints, m.keys.Hint(keymap.ScrollDown, keymap.ScrollUp))
		}
		if isMarkdown(f.Name) {
			what := "plain"
			if m.plain {
				what = "rendered"
			}
			hints = append(hints, m.keys.Hint(keymap.ToggleMD)+" "+what)
		}
		switch n := lineCount(f.Content); {
		case f.Truncated:
			left = " truncated "
		case n > maxLines:
			left = fmt.Sprintf(" %d of %d lines ", maxLines, n)
		}
	}
	if m.side == sideComments {
		hints = append(hints, m.keys.Hint(keymap.ToggleSide)+" info", m.keys.Hint(keymap.ScrollDown, keymap.ScrollUp)+" comments")
	} else {
		hints = append(hints, m.keys.Hint(keymap.ToggleSide)+" comments")
	}
	s := " " + strings.Join(hints, " · ") + " "
	c.put(y, c.w-2-text.Width(s), s, style{fg: p.overlay})
	if left != "" {
		c.put(y, 2, left, style{fg: p.yellow})
	}
}

// drawFileTabs lays the files along the top of the frame.
func (m *Model) drawFileTabs(c *canvas, l layout, g model.Gist, dx int) {
	p := m.pal
	line := style{fg: p.surface1}
	x := c.put(l.boxY, 1, " ", line)
	cur := m.selectedFile(g)
	for i, f := range g.Files {
		s := " " + f.Name + " "
		if x+text.Width(s) > dx-2 {
			c.put(l.boxY, x, "…", style{fg: p.overlay})
			break
		}
		x0 := x
		if i == cur {
			x = c.put(l.boxY, x, s, style{fg: p.base, bg: p.blue, bold: true})
		} else {
			x = c.put(l.boxY, x, s, style{fg: p.overlay})
		}
		m.addHit(l.boxY, x0, x, func() tea.Cmd {
			m.fileTab[g.ID] = i
			return nil
		})
		x = c.put(l.boxY, x, " ", line)
	}
}

func (m *Model) drawSideTabs(c *canvas, l layout, g model.Gist, dx int) {
	p := m.pal
	line := style{fg: p.surface1}
	x := c.put(l.boxY, dx+1, " ", line)
	for i, label := range []string{"info", fmt.Sprintf("comments (%d)", g.Comments)} {
		x0 := x
		s := " " + label + " "
		if m.side == i {
			x = c.put(l.boxY, x, s, style{fg: p.base, bg: p.blue, bold: true})
		} else {
			x = c.put(l.boxY, x, s, style{fg: p.overlay})
		}
		m.addHit(l.boxY, x0, x, func() tea.Cmd {
			m.side = i
			return m.readCommentsNow()
		})
		x = c.put(l.boxY, x, " ", line)
	}
}

func (m *Model) drawContent(c *canvas, l layout, g model.Gist, f model.File, read bool, dx int) {
	p := m.pal
	const x = 2
	inner := l.boxH - 2
	top := l.boxY + 1
	switch {
	case m.preview.failed[g.ID] && !read:
		c.put(top, x, "this gist could not be read · R tries again", style{fg: p.red})
		return
	case !read:
		c.put(top, x, m.spinChar(), style{fg: p.blue})
		c.put(top, x+2, "loading "+f.Name+"…", style{fg: p.subtext0})
		return
	case f.Binary:
		c.put(top+1, x+2, fmt.Sprintf("binary file · %s · %s", f.Type, size(f.Size)), style{fg: p.subtext0})
		c.put(top+2, x+2, m.keys.Hint(keymap.Browse)+" opens it in the browser", style{fg: p.dim})
		return
	}
	width := dx - 1 - x
	k := fileKey{g.ID, f.Name}
	limit := max(0, m.contentRows(g, f)-inner)
	off := min(m.preview.scroll[k], limit)
	if m.showsRendered(f) {
		lines := m.rendered(g, f, m.contentWidth())
		for i := range inner {
			if off+i >= len(lines) {
				break
			}
			m.putMarkdown(c, top+i, x+1, lines[off+i], width-1)
		}
		return
	}
	lines := m.highlighted(g, f)
	for i := range inner {
		n := off + i
		if n >= min(len(lines), maxLines) {
			break
		}
		c.put(top+i, x, fmt.Sprintf("%4d", n+1), style{fg: p.dim})
		segs := make([]seg, len(lines[n]))
		for j, t := range lines[n] {
			segs[j] = seg{t.Text, m.tokenStyle(t.Kind)}
		}
		c.putCut(top+i, x+lineNumbers, segs, width-lineNumbers, style{fg: p.overlay})
	}
}

func (m *Model) tokenStyle(k highlight.Kind) style {
	p := m.pal
	switch k {
	case highlight.Keyword:
		return style{fg: p.mauve}
	case highlight.String:
		return style{fg: p.green}
	case highlight.Number:
		return style{fg: p.peach}
	case highlight.Comment:
		return style{fg: p.overlay}
	case highlight.Function, highlight.Tag:
		return style{fg: p.blue}
	}
	return style{fg: p.text}
}

func (m *Model) putMarkdown(c *canvas, y, x int, l markdown.Line, w int) {
	segs := make([]seg, len(l))
	for i, s := range l {
		segs[i] = seg{s.Text, m.roleStyle(s)}
	}
	c.putCut(y, x, segs, w, style{fg: m.pal.overlay})
}

func (m *Model) roleStyle(s markdown.Span) style {
	p := m.pal
	st := style{fg: p.text, bold: s.Bold}
	switch s.Role {
	case markdown.Heading:
		st.fg = p.blue
	case markdown.Rule, markdown.QuoteMark, markdown.TableLine:
		st.fg = p.surface1
	case markdown.Quote, markdown.Todo, markdown.Finished:
		st.fg = p.overlay
	case markdown.Mark:
		st.fg = p.peach
	case markdown.Done:
		st.fg = p.green
	case markdown.Code:
		st.fg, st.bg = p.peach, p.surface0
	case markdown.CodeBlock:
		st.fg, st.bg = p.subtext1, p.surface0
	case markdown.CodeLang:
		st.fg, st.bg = p.overlay, p.surface0
	case markdown.Indented:
		st.fg = p.green
	case markdown.Link:
		st.fg = p.blue
	case markdown.Image:
		st.fg = p.teal
	case markdown.Cell:
		st.fg = p.subtext1
	}
	return st
}

func (m *Model) drawInfo(c *canvas, l layout, g model.Gist, f model.File, read bool, x int) {
	p := m.pal
	now := m.now()
	var total int64
	for _, gf := range g.Files {
		total += gf.Size
	}
	vis, visSt := "secret", style{fg: p.peach}
	if g.Public {
		vis, visSt = "public", style{fg: p.text}
	}
	plain := style{fg: p.text}
	type row struct {
		k, v string
		st   style
	}
	var rows []row
	if m.cur == tabStarred {
		rows = append(rows, row{"owner", g.Owner, plain})
	}
	revisions := "…"
	if c2, ok := m.content(g); ok {
		revisions = strconv.Itoa(len(c2.History))
	}
	rows = append(rows,
		row{"gist", g.ID[:min(7, len(g.ID))], plain},
		row{"visibility", vis, visSt},
		row{"files", fmt.Sprintf("%d · %s", len(g.Files), size(total)), plain},
		row{"revisions", revisions, plain},
		row{"comments", strconv.Itoa(g.Comments), plain},
		row{"created", ago(now.Sub(g.Created)) + " ago", plain},
		row{"updated", ago(now.Sub(g.Updated)) + " ago", plain},
	)
	if len(g.Files) > 0 {
		rows = append(rows, row{}, row{"file", f.Name, plain})
		sizeOf := size(f.Size)
		kind := f.Language
		switch {
		case read && f.Binary:
			kind = f.Type
		case read:
			kind = m.language(g, f)
			sizeOf += fmt.Sprintf(" · %d lines", lineCount(f.Content))
		}
		if kind != "" {
			rows = append(rows, row{"type", kind, plain})
		}
		rows = append(rows, row{"size", sizeOf, plain})
	}
	for i, r := range rows {
		y := l.boxY + 1 + i
		if y >= l.boxY+l.boxH-1 {
			break
		}
		if r.k == "" {
			continue
		}
		c.put(y, x, r.k, style{fg: p.overlay})
		c.putCut(y, x+11, []seg{{r.v, r.st}}, infoWidth-12, style{fg: p.overlay})
	}
}

// commentLines are the comments as lines of the pane, oldest first.
func (m *Model) commentLines(g model.Gist) [][]seg {
	p := m.pal
	cs := m.preview.comments[g.ID]
	w := max(infoWidth, (m.width-4)*commentsShare/100) - 3
	now := m.now()
	var out [][]seg
	for _, cm := range cs {
		if len(out) > 0 {
			out = append(out, nil)
		}
		who := style{fg: p.mauve, bold: true}
		if cm.Author == m.login {
			who.fg = p.peach
		}
		out = append(out, []seg{{cm.Author, who}, {" · " + when(now.Sub(cm.Created)), style{fg: p.overlay}}})
		k := drawnKey{fileKey{g.ID, strconv.FormatInt(cm.ID, 10)}, cm.Updated, w}
		lines, ok := m.preview.notes[k]
		if !ok {
			if len(m.preview.notes) >= maxDrawn {
				clear(m.preview.notes)
			}
			lines = markdown.Render(cm.Body, w)
			m.preview.notes[k] = lines
		}
		for _, l := range lines {
			row := []seg{{"  ", style{}}}
			for _, s := range l {
				row = append(row, seg{s.Text, m.roleStyle(s)})
			}
			out = append(out, row)
		}
	}
	return out
}

// when says how long ago d was, as a sentence would.
func when(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	return ago(d) + " ago"
}

func (m *Model) drawComments(c *canvas, l layout, g model.Gist, x int) {
	p := m.pal
	top, rows := l.boxY+1, l.boxH-2
	w := c.w - 2 - x
	if g.Comments == 0 {
		c.put(top+1, x, "no comments", style{fg: p.dim})
		return
	}
	if _, ok := m.preview.comments[g.ID]; !ok {
		c.put(top+1, x, m.spinChar(), style{fg: p.blue})
		c.put(top+1, x+2, "loading comments…", style{fg: p.subtext0})
		return
	}
	lines := m.commentLines(g)
	limit := max(0, len(lines)-rows)
	off := min(m.preview.commentScroll[g.ID], limit)
	for i := range rows {
		if off+i >= len(lines) {
			break
		}
		c.putCut(top+i, x, lines[off+i], w, style{fg: p.overlay})
	}
	if off < limit {
		c.put(top+rows-1, x+w-1, "↓", style{fg: p.overlay})
	}
}
