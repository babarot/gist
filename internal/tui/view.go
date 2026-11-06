package tui

import (
	"fmt"
	"image/color"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/config"
	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/text"
	"github.com/babarot/gist/v2/internal/tui/statusline"
)

// View draws the screen.
func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

const spinner = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"

func (m *Model) spinChar() string {
	r := []rune(spinner)
	return string(r[m.spinFrame%len(r)])
}

// layout is where the parts of the screen are, for a terminal of this size.
type layout struct {
	listY, listRows int
	boxY, boxH      int
	statusY, keysY  int
}

const (
	headerRows  = 2 // the header and the column names
	footerRows  = 2 // the status line and the key line
	minListRows = 3
	minBoxRows  = 5
)

func (m *Model) layout() layout {
	room := m.height - headerRows - footerRows
	box := min(m.boxHeight, room-minListRows)
	box = max(min(minBoxRows, room), box)
	return layout{
		listY: headerRows, listRows: max(0, room-box),
		boxY: headerRows + max(0, room-box), boxH: max(0, box),
		statusY: m.height - footerRows, keysY: m.height - 1,
	}
}

func (m *Model) render() string {
	m.hits = nil
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	c := newCanvas(m.width, m.height)
	if m.uiState() == uiAuthFail {
		m.drawAuthFail(c)
		m.drawFooter(c, m.layout())
		return c.String()
	}
	l := m.layout()
	m.drawHeader(c)
	m.drawColumns(c)
	m.drawList(c, l)
	m.drawPreview(c, l)
	m.drawFooter(c, l)
	if m.uiState() == uiHelp {
		m.drawHelp(c)
	}
	return c.String()
}

func (m *Model) drawHeader(c *canvas) {
	p := m.pal
	c.clear(0, 0, c.w, p.surface0)
	x := c.put(0, 0, "gist", style{fg: p.blue, bold: true})
	login := "…"
	if m.login != "" {
		login = "@" + m.login
	}
	x = c.put(0, x, " // "+login, style{fg: p.subtext0})
	x += 4
	for i, name := range []string{"mine", "starred"} {
		t := &m.tabs[i]
		count := "…"
		if t.loaded {
			count = strconv.Itoa(len(t.gists))
			if t.first {
				count += "+"
			}
		}
		x0 := x
		label := fmt.Sprintf(" %d ", i+1)
		rest := name + " (" + count + ") "
		if m.cur == i {
			x = c.put(0, x, label, style{fg: p.subtext0, bg: p.surface1})
			x = c.put(0, x, rest, style{fg: p.text, bg: p.surface1, bold: true})
		} else {
			x = c.put(0, x, label, style{fg: p.overlay})
			x = c.put(0, x, rest, style{fg: p.subtext0})
		}
		m.addHit(0, x0, x, func() tea.Cmd {
			if i == tabStarred {
				return m.showStarred()
			}
			m.cur = i
			return nil
		})
		x += 2
	}
	var right []seg
	dot := seg{" · ", style{fg: p.overlay}}
	t := m.tab()
	switch {
	case t.reading && t.first && t.loaded:
		right = append(right, seg{m.spinChar() + " ", style{fg: p.blue}},
			seg{fmt.Sprintf("loading %d of ~%d", t.got, t.estimate), style{fg: p.subtext0}}, dot)
	case t.reading:
		what := "refreshing"
		if !t.loaded {
			what = "loading"
		}
		right = append(right, seg{m.spinChar() + " ", style{fg: p.blue}}, seg{what, style{fg: p.subtext0}}, dot)
	}
	right = append(right, seg{"via " + m.via, style{fg: p.overlay}})
	w := 0
	for _, s := range right {
		w += text.Width(s.text)
	}
	if rx := c.w - w - 1; rx > x {
		for _, s := range right {
			rx = c.put(0, rx, s.text, s.st)
		}
	}
}

// columns is where the columns of the list are, for a terminal of width w.
// Narrow terminals lose COMMENTS, then FILES.
type columns struct {
	lead, leadW     int // the first column: the description, or the file
	desc, descW     int // the description, when the first column is the file
	files, comments int // right edges, or 0 when dropped
	vis, updated    int // left edge, right edge
}

const (
	rowLead     = 4 // the cursor and the mark
	colGap      = 2
	filesCol    = 5
	commentsCol = 8
	visCol      = 6
	updatedCol  = 7
	fileColMax  = 32
	// The widths under which a column goes.
	commentsFrom = 90
	filesFrom    = 80
)

func (m *Model) columns() columns {
	w := m.width
	var c columns
	c.updated = w - 1
	c.vis = c.updated - updatedCol - colGap - visCol
	right := c.vis - colGap
	if w >= commentsFrom {
		c.comments = right
		right -= commentsCol + colGap
	}
	if w >= filesFrom {
		c.files = right
		right -= filesCol + colGap
	}
	c.lead = rowLead
	room := max(0, right-rowLead)
	c.leadW = room
	if m.opt.Config.List.Lead == config.LeadFile {
		c.leadW = min(fileColMax, room*3/10)
		c.desc = c.lead + c.leadW + colGap
		c.descW = max(0, room-c.leadW-colGap)
	}
	return c
}

func (m *Model) drawColumns(c *canvas) {
	p := m.pal
	cols := m.columns()
	head := style{fg: p.subtext1, bold: true}
	owner := ""
	if m.cur == tabStarred {
		owner = "OWNER / "
	}
	if m.opt.Config.List.Lead == config.LeadFile {
		c.put(1, cols.lead, owner+"FILE", head)
		c.put(1, cols.desc, "DESCRIPTION", head)
	} else {
		c.put(1, cols.lead, owner+"DESCRIPTION", head)
	}
	if cols.comments > 0 {
		c.put(1, cols.comments-len("COMMENTS"), "COMMENTS", head)
	}
	if cols.files > 0 {
		c.put(1, cols.files-len("FILES"), "FILES", head)
	}
	c.put(1, cols.vis, "VIS", head)
	c.put(1, cols.updated-len("UPDATED"), "UPDATED", head)
}

func (m *Model) drawList(c *canvas, l layout) {
	p := m.pal
	t := m.tab()
	if !t.loaded {
		if t.reading || m.readingViewer {
			what := "loading your gists…"
			if m.cur == tabStarred {
				what = "loading starred gists…"
			}
			c.put(l.listY+1, 2, m.spinChar(), style{fg: p.blue})
			c.put(l.listY+1, 4, what, style{fg: p.subtext0})
		}
		return
	}
	if len(t.gists) == 0 {
		c.put(l.listY+1, 4, "no gists yet", style{fg: p.dim})
		return
	}
	cols := m.columns()
	now := m.now()
	for i := range l.listRows {
		row := t.top + i
		if row >= len(t.gists) {
			break
		}
		g := t.gists[row]
		y := l.listY + i
		var bg color.Color
		at, fresh := t.fresh[g.ID]
		switch {
		case row == t.cur:
			bg = p.surface0
		case fresh && now.Sub(at) < freshFor:
			bg = p.fresh
		}
		c.fill(y, 0, c.w, bg)
		if row == t.cur {
			c.put(y, 0, "▎", style{fg: p.blue, bg: bg})
		}
		if t.marks[g.ID] {
			c.put(y, 2, "●", style{fg: p.blue, bg: bg})
		}
		m.drawRow(c, y, g, cols, bg, now)
		m.addHit(y, 0, c.w, func() tea.Cmd {
			m.moveTo(row)
			return nil
		})
	}
}

func (m *Model) drawRow(c *canvas, y int, g model.Gist, cols columns, bg color.Color, now time.Time) {
	p := m.pal
	on := func(st style) style {
		st.bg = bg
		return st
	}
	more := on(style{fg: p.overlay})
	var lead []seg
	if m.cur == tabStarred && g.Owner != "" {
		lead = append(lead, seg{g.Owner + "  ", on(style{fg: p.overlay})})
	}
	first := ""
	if len(g.Files) > 0 {
		first = g.Files[0].Name
	}
	extra := seg{}
	if n := len(g.Files) - 1; n > 0 {
		extra = seg{fmt.Sprintf(" +%d", n), on(style{fg: p.dim})}
	}
	if m.opt.Config.List.Lead == config.LeadFile {
		c.putCut(y, cols.lead, append(lead, seg{first, on(style{fg: p.text})}, extra), cols.leadW, more)
		if g.Description != "" {
			c.putCut(y, cols.desc, []seg{{g.Description, on(style{fg: p.subtext0})}}, cols.descW, more)
		}
	} else {
		if g.Description != "" {
			lead = append(lead, seg{g.Description, on(style{fg: p.text})})
		} else {
			lead = append(lead, seg{first, on(style{fg: p.overlay})}, extra)
		}
		c.putCut(y, cols.lead, lead, cols.leadW, more)
	}
	dim := on(style{fg: p.subtext0})
	if cols.comments > 0 && g.Comments > 0 {
		s := strconv.Itoa(g.Comments)
		c.put(y, cols.comments-len(s), s, dim)
	}
	if cols.files > 0 {
		s := strconv.Itoa(len(g.Files))
		c.put(y, cols.files-len(s), s, dim)
	}
	if g.Public {
		c.put(y, cols.vis, "public", on(style{fg: p.overlay}))
	} else {
		c.put(y, cols.vis, "secret", on(style{fg: p.peach}))
	}
	age := ago(now.Sub(g.Updated))
	st := dim
	if now.Sub(g.Updated) < time.Hour {
		st = on(style{fg: p.text})
	}
	c.put(y, cols.updated-len(age), age, st)
}

// ago says how long d is, shortly.
func ago(d time.Duration) string {
	const day = 24 * time.Hour
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	case d < day:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	case d < 30*day:
		return fmt.Sprintf("%dd", int(d/day))
	case d < 365*day:
		return fmt.Sprintf("%dmo", int(d/(30*day)))
	}
	return fmt.Sprintf("%dy", int(d/(365*day)))
}

// size says how many bytes n is, shortly.
func size(n int64) string {
	switch {
	case n < 1<<10:
		return fmt.Sprintf("%d B", n)
	case n < 1<<20:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
}

func (m *Model) drawFooter(c *canvas, l layout) {
	p := m.pal
	kind, s := m.line.Shown(m.now(), nil)
	st := style{fg: p.subtext0}
	switch kind {
	case statusline.Info:
		st.fg = p.green
	case statusline.Refusal:
		st.fg = p.yellow
	case statusline.Failure:
		st.fg = p.red
	}
	c.put(l.statusY, 0, s, st)
	x := 0
	for i, h := range m.keyHints() {
		if h.key == "" {
			continue
		}
		if i > 0 {
			x = c.put(l.keysY, x, " · ", style{fg: p.overlay})
		}
		x = c.put(l.keysY, x, h.key, style{fg: p.blue, bold: true})
		x = c.put(l.keysY, x, " "+h.what, style{fg: p.overlay})
	}
}

func (m *Model) drawHelp(c *canvas) {
	p := m.pal
	w := min(96, c.w-8)
	rows := 0
	for _, g := range helpGroups {
		rows = max(rows, len(g.rows))
	}
	h := min(rows+6, c.h)
	x, y := (c.w-w)/2, min(4, max(0, c.h-h))
	c.box(x, y, w, h, p.blue, nil)
	c.put(y, x+1, " keys ", style{fg: p.subtext1, bold: true})
	colW := (w - 4) / len(helpGroups)
	for gi, g := range helpGroups {
		gx := x + 2 + gi*colW
		c.put(y+2, gx, g.title, style{fg: p.subtext0, bold: true})
		for i, r := range g.rows {
			keys := m.keys.Hint(r.actions...)
			c.putCut(y+3+i, gx, []seg{{keys, style{fg: p.blue, bold: true}}}, 6, style{fg: p.overlay})
			c.putCut(y+3+i, gx+7, []seg{{r.what, style{fg: p.text}}}, colW-8, style{fg: p.overlay})
		}
	}
}

func (m *Model) drawAuthFail(c *canvas) {
	p := m.pal
	c.clear(0, 0, c.w, p.surface0)
	x := c.put(0, 0, "gist", style{fg: p.blue, bold: true})
	c.put(0, x, " // not logged in", style{fg: p.subtext0})
	lx, y := 4, 3
	c.put(y, lx, "✗ gist could not find a GitHub token for github.com.", style{fg: p.red, bold: true})
	y += 2
	c.put(y, lx, "it looked in these places, in this order:", style{fg: p.text})
	y += 2
	for _, r := range [][2]string{
		{"GH_TOKEN", "not set"},
		{"GITHUB_TOKEN", "not set"},
		{"gh's config (hosts.yml)", "no token for github.com"},
		{"gh auth token (keychain)", "gh is not logged in"},
	} {
		c.put(y, lx+2, r[0], style{fg: p.subtext1})
		c.put(y, lx+30, r[1], style{fg: p.overlay})
		y++
	}
	y++
	x = c.put(y, lx, "run ", style{fg: p.text})
	x = c.put(y, x, "gh auth login", style{fg: p.peach, bold: true})
	x = c.put(y, x, ", or set ", style{fg: p.text})
	c.put(y, x, "GITHUB_TOKEN", style{fg: p.peach, bold: true})
	y += 2
	c.put(y, lx, `a fine-grained token needs "Gists: read and write".`, style{fg: p.dim})
}
