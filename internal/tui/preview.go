package tui

import (
	"fmt"
	"image/color"
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
	// paneShare is how much of the width the pane takes, in percent, with the
	// comments under the info until its line is dragged; it is between infoWidth
	// and paneMaxWidth. Otherwise it starts infoWidth wide.
	paneShare    = 27
	paneMaxWidth = 60
	// stackMinWidth and stackMinRows are how large the screen and the preview
	// have to be for the comments to go under the info instead of in its place.
	stackMinWidth = 160
	stackMinRows  = 20
	// maxLines is the most lines of a file shown.
	maxLines = 1000
	// lineNumbers is the width of the line numbers and the gap after them.
	lineNumbers = 6
	// minContent is the narrowest the content goes before the pane is left out.
	minContent = 20
)

// paneX is where the line between the content and the pane is, or the width of
// the screen when there is no room for a pane. The info and the comments take
// the same width, which only dragging the line changes; a width dragged to on
// a larger screen gives way to the content here.
func (m *Model) paneX() int {
	most := m.width - 3 - minContent
	if most < infoWidth {
		return m.width
	}
	w := m.paneW
	switch {
	case w > 0:
	case m.stacked():
		w = min(paneMaxWidth, (m.width-4)*paneShare/100)
	default:
		w = infoWidth
	}
	return m.width - 3 - min(most, max(infoWidth, w))
}

// dragPaneTo sets the width of the pane from the column its line was dragged
// to, as far as the content and the info have room.
func (m *Model) dragPaneTo(x int) {
	most := m.width - 3 - minContent
	if most < infoWidth {
		return
	}
	m.paneW = min(most, max(infoWidth, m.width-3-x))
}

// keepPaneWidth keeps the width of the pane for the next launch.
func (m *Model) keepPaneWidth() {
	if m.opt.Layout == nil {
		return
	}
	if err := m.opt.Layout.SavePaneWidth(m.paneW); err != nil {
		m.opt.Log.Warn("the layout cannot be kept", "err", err.Error())
	}
}

// stacked says the screen is large enough for the comments to go under the
// info, so that c only says which of the content and the comments J and K move.
func (m *Model) stacked() bool {
	return m.width >= stackMinWidth && m.innerRows() >= stackMinRows
}

// paneWidth is the room for the text of the pane, from two cells after its line
// to the frame.
func (m *Model) paneWidth() int { return max(infoWidth, m.width-3-m.paneX()) }

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
	g, ok := m.current()
	if !ok {
		return
	}
	f, read := m.shownFile(g)
	if !read || f.Binary {
		return
	}
	k := fileKey{g.ID, f.Name}
	limit := max(0, m.contentRows(g, f)-m.contentRoom())
	m.preview.scroll[k] = max(0, min(m.preview.scroll[k]+by, limit))
}

func (m *Model) scrollComments(by int) {
	g, ok := m.current()
	if !ok {
		return
	}
	limit := max(0, len(m.commentLines(g))-m.commentRoom(g))
	m.preview.commentScroll[g.ID] = max(0, min(m.preview.commentScroll[g.ID]+by, limit))
}

func (m *Model) drawPreview(c *canvas, l layout) {
	p := m.pal
	if l.boxH < 2 {
		return
	}
	c.box(0, l.boxY, c.w, l.boxH, p.surface1, nil)
	g, ok := m.current()
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
		switch {
		case m.stacked():
			m.drawInfo(c, l, g, f, read, dx+2)
			top := m.commentsTop(l, g)
			m.drawCommentsRule(c, l, g, dx, top-1)
			m.drawComments(c, g, dx+2, top, l.boxY+l.boxH-1-top)
		case m.side == sideComments:
			m.drawComments(c, g, dx+2, l.boxY+1, l.boxH-2)
		default:
			m.drawInfo(c, l, g, f, read, dx+2)
		}
	}
	if read && !f.Binary {
		k := fileKey{g.ID, f.Name}
		inner, rows := m.contentRoom(), m.contentRows(g, f)
		m.drawThumb(c, min(dx, c.w-1), contentTop(l), inner, min(m.preview.scroll[k], max(0, rows-inner)), rows, p.overlay)
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
		if m.side == sideInfo && m.contentRows(g, f) > m.contentRoom() {
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
	switch {
	case m.side == sideComments && m.stacked():
		hints = append(hints, m.keys.Hint(keymap.ToggleSide)+" content", m.keys.Hint(keymap.ScrollDown, keymap.ScrollUp)+" comments")
	case m.side == sideComments:
		hints = append(hints, m.keys.Hint(keymap.ToggleSide)+" info", m.keys.Hint(keymap.ScrollDown, keymap.ScrollUp)+" comments")
	default:
		hints = append(hints, m.keys.Hint(keymap.ToggleSide)+" comments")
	}
	s := " " + strings.Join(hints, " · ") + " "
	c.put(y, c.w-2-text.Width(s), s, style{fg: p.overlay})
	if m.hasDraft(g, f.Name) {
		c.put(y, 2, " "+draftMark+" draft · "+m.keys.Hint(keymap.Edit)+" to continue ", style{fg: p.peach})
		return
	}
	if left != "" {
		c.put(y, 2, left, style{fg: p.yellow})
	}
}

// hasDraft says a file of g has a draft.
func (m *Model) hasDraft(g model.Gist, file string) bool {
	return m.drafts[g.ID] && m.opt.Editor != nil && m.opt.Editor.State.HasDraft(g.ID, file)
}

// More tabs on the left of the frame are told by tabsBefore, on the right by
// tabsAfter.
const (
	tabsBefore = "‹ "
	tabsAfter  = "…"
	tabsX      = 2 // where the first tab starts
)

// firstTab is the first of the tabs to draw, in room cells: the tabs before the
// one shown go until it is in sight.
func firstTab(widths []int, cur, room int) int {
	from := 0
	for from < cur {
		w := 0
		if from > 0 {
			w = text.Width(tabsBefore)
		}
		for _, x := range widths[from : cur+1] {
			w += x + 1
		}
		if w-1 <= room {
			break
		}
		from++
	}
	return from
}

// contentTop is the first line of the content: the files are on the line
// before it, under the top of the frame.
func contentTop(l layout) int { return l.boxY + 2 }

// contentRoom is how many lines of the content are in sight.
func (m *Model) contentRoom() int { return max(0, m.innerRows()-1) }

// drawFileTabs says along the top of the frame how many files there are and
// lays them on the line under it, so that however many there are the top
// stays a line. A file is a choice within the pane, so its tab is fainter than
// those of drawSideTabs, which switch the pane.
func (m *Model) drawFileTabs(c *canvas, l layout, g model.Gist, dx int) {
	p := m.pal
	line := style{fg: p.surface1}
	x := c.put(l.boxY, 1, " ", line)
	s := fmt.Sprintf(" files (%d) ", len(g.Files))
	x = c.putCut(l.boxY, x, []seg{{s, style{fg: p.subtext1, bold: true}}}, dx-2-x, line)
	c.put(l.boxY, x, " ", line)
	m.putFileTabs(c, l.boxY+1, tabsX, min(dx, c.w-1)-1, g)
}

// putFileTabs lays the tabs of the files of g on line y from x to end,
// keeping the one shown in sight however many there are.
func (m *Model) putFileTabs(c *canvas, y, x, end int, g model.Gist) {
	p := m.pal
	cur := m.selectedFile(g)
	widths := make([]int, len(g.Files))
	for i, f := range g.Files {
		widths[i] = text.Width(text.Line(" " + f.Name + " "))
	}
	from := firstTab(widths, cur, end-tabsX)
	if from > 0 {
		x = c.put(y, x, tabsBefore, style{fg: p.overlay})
	}
	for i := from; i < len(g.Files); i++ {
		s := " " + g.Files[i].Name + " "
		on := style{fg: p.blue, bg: p.surface0, bold: true}
		if x+widths[i] > end {
			// Only the tab shown is cut, when it is wider than the room
			if i == cur {
				c.putCut(y, x, []seg{{s, on}}, end-x, on)
			} else {
				c.put(y, x, tabsAfter, style{fg: p.overlay})
			}
			break
		}
		x0 := x
		if i == cur {
			x = c.put(y, x, s, on)
		} else {
			x = c.put(y, x, s, style{fg: p.overlay})
		}
		m.addHit(y, x0, x, func() tea.Cmd {
			m.fileTab[g.ID] = i
			return nil
		})
		x = c.put(y, x, " ", style{})
	}
}

func (m *Model) drawSideTabs(c *canvas, l layout, g model.Gist, dx int) {
	p := m.pal
	line := style{fg: p.surface1}
	x := c.put(l.boxY, dx+1, " ", line)
	labels := []string{"info", fmt.Sprintf("comments (%d)", g.Comments)}
	if m.stacked() {
		// The comments have their own line under the info
		labels = labels[:1]
	}
	for i, label := range labels {
		x0 := x
		s := " " + label + " "
		if m.side == i || m.stacked() {
			x = c.put(l.boxY, x, s, style{fg: p.text, bg: p.tab, bold: true})
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
	inner := m.contentRoom()
	top := contentTop(l)
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

// totalSize is how many bytes the files of g are.
func totalSize(g model.Gist) int64 {
	var n int64
	for _, f := range g.Files {
		n += f.Size
	}
	return n
}

// infoRow is a line of the info: a name and what it is.
type infoRow struct {
	k, v string
	st   style
}

// infoRows are the lines of the info of g, with f the file shown; a row with no
// name is a gap.
func (m *Model) infoRows(g model.Gist, f model.File, read bool) []infoRow {
	p := m.pal
	now := m.now()
	total := totalSize(g)
	vis, visSt := "secret", style{fg: p.peach}
	if g.Public {
		vis, visSt = "public", style{fg: p.text}
	}
	plain := style{fg: p.text}
	var rows []infoRow
	if m.cur == tabStarred {
		rows = append(rows, infoRow{"owner", g.Owner, plain})
	}
	revisions := "…"
	if c2, ok := m.content(g); ok {
		revisions = strconv.Itoa(len(c2.History))
	}
	rows = append(rows,
		infoRow{"gist", g.ID[:min(7, len(g.ID))], plain},
		infoRow{"visibility", vis, visSt},
		infoRow{"files", fmt.Sprintf("%d · %s", len(g.Files), size(total)), plain},
		infoRow{"revisions", revisions, plain},
		infoRow{"comments", strconv.Itoa(g.Comments), plain},
		infoRow{"created", when(now.Sub(g.Created)), plain},
		infoRow{"updated", when(now.Sub(g.Updated)), plain},
	)
	if len(g.Files) > 0 {
		rows = append(rows, infoRow{}, infoRow{"file", f.Name, plain})
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
			rows = append(rows, infoRow{"type", kind, plain})
		}
		rows = append(rows, infoRow{"size", sizeOf, plain})
	}
	return rows
}

func (m *Model) drawInfo(c *canvas, l layout, g model.Gist, f model.File, read bool, x int) {
	p := m.pal
	w := m.paneWidth()
	for i, r := range m.infoRows(g, f, read) {
		y := l.boxY + 1 + i
		if y >= l.boxY+l.boxH-1 {
			break
		}
		if r.k == "" {
			continue
		}
		c.put(y, x, r.k, style{fg: p.overlay})
		c.putCut(y, x+11, []seg{{r.v, r.st}}, w-12, style{fg: p.overlay})
	}
}

// commentsTop is the first row of the comments when they are under the info:
// a gap and the line of their name after it. The info of every file of g is as
// long, so that the comments stay where they are as the files are stepped.
func (m *Model) commentsTop(l layout, g model.Gist) int {
	return l.boxY + 1 + len(m.infoRows(g, model.File{Language: "x"}, false)) + 2
}

// commentRoom is how many lines of the comments are in sight.
func (m *Model) commentRoom(g model.Gist) int {
	if !m.stacked() {
		return m.innerRows()
	}
	l := m.layout()
	return max(0, l.boxY+l.boxH-1-m.commentsTop(l, g))
}

// drawCommentsRule draws on row y across the pane the line the comments start
// under, with their name, bright when J and K move them.
func (m *Model) drawCommentsRule(c *canvas, l layout, g model.Gist, dx, y int) {
	p := m.pal
	line := style{fg: p.surface1}
	c.put(y, dx, "├"+strings.Repeat("─", max(0, c.w-dx-2))+"┤", line)
	label := fmt.Sprintf(" comments (%d) ", g.Comments)
	st := style{fg: p.overlay}
	if m.side == sideComments {
		st = style{fg: p.text, bg: p.tab, bold: true}
	}
	x0 := dx + 2
	x := c.put(y, x0, label, st)
	m.addHit(y, x0, x, func() tea.Cmd {
		m.toggleSide()
		return m.readCommentsNow()
	})
}

// commentLines are the comments as lines of the pane, oldest first.
func (m *Model) commentLines(g model.Gist) [][]seg {
	p := m.pal
	cs := m.preview.comments[g.ID]
	w := m.paneWidth() - 3
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

// drawComments draws the comments of g from column x in rows lines from top.
func (m *Model) drawComments(c *canvas, g model.Gist, x, top, rows int) {
	p := m.pal
	if rows <= 0 {
		return
	}
	w := c.w - 2 - x
	// Alone in the pane, a word on the comments is a line down, as the info is
	at := top + 1
	if m.stacked() {
		at = top
	}
	if g.Comments == 0 {
		c.put(at, x, "no comments", style{fg: p.dim})
		return
	}
	if _, ok := m.preview.comments[g.ID]; !ok {
		c.put(at, x, m.spinChar(), style{fg: p.blue})
		c.put(at, x+2, "loading comments…", style{fg: p.subtext0})
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
	m.drawThumb(c, c.w-1, top, rows, off, len(lines), m.pal.overlay)
}

// thumbAt places a scrollbar's thumb on a track of room rows, for a view of room
// lines starting at first out of total, as the rows [from, to). It touches an
// end of the track only when the view is at that end.
func thumbAt(first, room, total int) (from, to int) {
	size := max(1, (room*room+total/2)/total)
	span, last := room-size, total-room
	from = (first*span + last/2) / last
	if first > 0 && from == 0 && span > 1 {
		from = 1
	}
	if first < last && from == span && span > 1 {
		from = span - 1
	}
	return from, from + size
}

// drawThumb draws a scrollbar on the line at column x, in fg, when there is more
// than shows. It has to stand out from the frame it is drawn on.
func (m *Model) drawThumb(c *canvas, x, top, room, first, total int, fg color.Color) {
	if room <= 0 || total <= room {
		return
	}
	from, to := thumbAt(first, room, total)
	for i := from; i < to; i++ {
		c.put(top+i, x, "┃", style{fg: fg})
	}
}
