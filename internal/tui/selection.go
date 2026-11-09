package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/github"
	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/text"
	"github.com/babarot/gist/v2/internal/tui/keymap"
)

// Only the gists of the mine tab are selected: those of the starred tab are
// read only. With a selection, a key that acts on gists acts on it or refuses,
// never on the gist under the cursor instead, as in gh-desk (its ADR-042).

// toggleSelect selects the gist under the cursor, or no longer, and goes down.
func (m *Model) toggleSelect() {
	g, ok := m.current()
	if !ok || m.readOnly(g) {
		return
	}
	if m.selected[g.ID] {
		delete(m.selected, g.ID)
	} else {
		m.selected[g.ID] = true
	}
	m.moveTo(m.tab().cur + 1)
}

// selectShown selects every gist the filter leaves.
func (m *Model) selectShown() {
	g, ok := m.current()
	if !ok || m.readOnly(g) {
		return
	}
	for _, r := range m.tab().rows {
		m.selected[r.gist.ID] = true
	}
}

// clearOrRestore takes the selection away, or with none, brings the last back.
func (m *Model) clearOrRestore() {
	if len(m.selected) > 0 {
		m.clearSelection()
		return
	}
	m.restoreSelection()
}

// clearSelection takes the whole selection away, for the clear key to bring back.
func (m *Model) clearSelection() {
	m.lastSelected = m.selectedIDs()
	clear(m.selected)
}

// selectedIDs are the gists selected, in the order of the mine tab.
func (m *Model) selectedIDs() []string {
	var ids []string
	for _, g := range m.tabs[tabMine].gists {
		if m.selected[g.ID] {
			ids = append(ids, g.ID)
		}
	}
	return ids
}

// restoreSelection brings the last selection back, without what has left the
// list since.
func (m *Model) restoreSelection() {
	n := 0
	for _, id := range m.lastSelected {
		if m.mine(id) {
			m.selected[id] = true
			n++
		}
	}
	m.lastSelected = nil
	if n == 0 {
		m.line.Refuse(m.now(), "no selection to bring back")
		return
	}
	m.line.Say(m.now(), fmt.Sprintf("selected again: %s", gistCount(n)))
}

// canRestore says the clear key would bring a selection back.
func (m *Model) canRestore() bool {
	return len(m.selected) == 0 && slices.ContainsFunc(m.lastSelected, m.mine)
}

// mine says the mine tab has the gist.
func (m *Model) mine(id string) bool {
	return slices.ContainsFunc(m.tabs[tabMine].gists, func(g model.Gist) bool { return g.ID == id })
}

// pruneSelection drops from the selection what has left the mine tab.
func (m *Model) pruneSelection() {
	for id := range m.selected {
		if !m.mine(id) {
			delete(m.selected, id)
		}
	}
}

// oneAtATime refuses a key that acts on one gist while gists are selected.
func (m *Model) oneAtATime() bool {
	if len(m.selected) == 0 {
		return false
	}
	m.line.Refuse(m.now(), "one gist at a time · "+m.keys.Hint(keymap.ClearSelection)+" clears the selection")
	return true
}

// selectedGists are the gists selected, in the order of the mine tab, those
// being written to among them: what reads them does not get in their way.
func (m *Model) selectedGists() []model.Gist {
	var gs []model.Gist
	for _, g := range m.tabs[tabMine].gists {
		if m.selected[g.ID] {
			gs = append(gs, g)
		}
	}
	return gs
}

// copySelectedURLs copies the URLs of the gists selected, a line each. The
// selection stays: nothing was changed, so the next key can act on it too.
func (m *Model) copySelectedURLs() tea.Cmd {
	gs := m.selectedGists()
	if len(gs) == 0 {
		return nil
	}
	urls := make([]string, len(gs))
	for i, g := range gs {
		urls[i] = g.URL
	}
	if len(urls) == 1 {
		m.line.Say(m.now(), "✓ copied "+urls[0])
	} else {
		m.line.Say(m.now(), fmt.Sprintf("✓ copied %d URLs", len(urls)))
	}
	return tea.SetClipboard(strings.Join(urls, "\n"))
}

// maxBrowse is the most gists the browser key opens at once: V can select
// hundreds, and each is a tab of the browser.
const maxBrowse = 10

// browseSelected opens the gists selected in the browser, or none past
// maxBrowse. The selection stays.
func (m *Model) browseSelected() tea.Cmd {
	gs := m.selectedGists()
	switch {
	case len(gs) == 0:
		return nil
	case len(gs) > maxBrowse:
		m.line.Refuse(m.now(), fmt.Sprintf("open at most %d in the browser: %d selected", maxBrowse, len(gs)))
		return nil
	}
	cmds := make([]tea.Cmd, len(gs))
	for i, g := range gs {
		cmds[i] = func() tea.Msg { return browsedMsg{m.opt.Browse(g.URL)} }
	}
	m.line.Say(m.now(), fmt.Sprintf("opened %d in the browser", len(gs)))
	return tea.Batch(cmds...)
}

// shownSelected marks the gist as selected on the tab shown.
func (m *Model) shownSelected(id string) bool { return m.cur == tabMine && m.selected[id] }

// deleteTargets are the gists of the selection a delete would send, in the
// order of the mine tab: from the gists the tab has, never from the IDs alone,
// and without those being sent already.
func (m *Model) deleteTargets() []model.Gist {
	var gs []model.Gist
	for _, g := range m.tabs[tabMine].gists {
		if m.selected[g.ID] && m.writing[g.ID] == "" {
			gs = append(gs, g)
		}
	}
	return gs
}

// openDeleteMany asks about deleting the gists selected.
func (m *Model) openDeleteMany() tea.Cmd {
	ids := m.selectedIDs()
	if len(ids) == 0 {
		return nil
	}
	if len(m.deleteTargets()) == 0 {
		m.line.Refuse(m.now(), "wait for what is being sent")
		return nil
	}
	m.deleting = &deleting{many: ids}
	return nil
}

func (m *Model) deleteManyKey(k string) tea.Cmd {
	d := m.deleting
	on := func(a keymap.Action) bool { return m.keys.Matches(a, k) }
	switch {
	case k == keymap.Back:
		m.deleting = nil
	case on(keymap.Confirm):
		m.deleting = nil
		return m.deleteMany()
	case on(keymap.ScrollDown), on(keymap.Down):
		d.top++
	case on(keymap.ScrollUp), on(keymap.Up):
		d.top--
	case on(keymap.PageDown):
		d.top += m.deleteRows(len(d.many), 0)
	case on(keymap.PageUp):
		d.top -= m.deleteRows(len(d.many), 0)
	}
	return nil
}

// deleteBatch is the gists of a selection being deleted, one after another:
// writes sent at once would meet GitHub's secondary rate limit sooner.
type deleteBatch struct {
	// queue is what is left to send, and ids all of it, for the clear key to
	// select again what was not deleted.
	queue []model.Gist
	ids   []string
	// total is how many, and deleted and failed how many of them so far.
	total, deleted, failed int
	firstErr               error
}

// deleteMany sends the delete of the gists selected. They leave the selection
// at once, as gh-desk's ADR-094 has it, for the clear key to bring back.
func (m *Model) deleteMany() tea.Cmd {
	gs := m.deleteTargets()
	switch {
	case len(gs) == 0 && len(m.selected) == 0:
		m.line.Refuse(m.now(), "nothing to delete: the gists have left the list")
		return nil
	case len(gs) == 0:
		m.line.Refuse(m.now(), "wait for what is being sent")
		return nil
	}
	b := m.batch
	start := b == nil
	if start {
		b = &deleteBatch{}
		m.batch = b
	}
	for _, g := range gs {
		delete(m.selected, g.ID)
		m.writing[g.ID] = "delete"
		b.ids = append(b.ids, g.ID)
	}
	b.queue = append(b.queue, gs...)
	b.total += len(gs)
	m.lastSelected = slices.Clone(b.ids)
	m.batchProgress()
	if !start {
		return nil
	}
	return m.nextDelete()
}

func (m *Model) nextDelete() tea.Cmd {
	b := m.batch
	g := b.queue[0]
	b.queue = b.queue[1:]
	return m.deleteGist(g, true)
}

func (m *Model) batchProgress() {
	b := m.batch
	m.line.Say(m.now(), fmt.Sprintf("deleting %d of %d", b.deleted+b.failed+1, b.total))
}

// gotBatchDeleted takes in the answer for one gist of a batch, and sends the
// next. A rate limit, or the end of the run, stops the rest, which leave
// writing so they can be deleted again.
func (m *Model) gotBatchDeleted(msg deletedMsg) tea.Cmd {
	b := m.batch
	delete(m.writing, msg.gist)
	if msg.err == nil {
		b.deleted++
		m.dropGist(msg.gist)
	} else {
		b.failed++
		if b.firstErr == nil {
			b.firstErr = msg.err
		}
		m.opt.Log.Warn("a gist of the selection cannot be deleted", "gist", msg.gist, "err", msg.err.Error())
	}
	stopped := 0
	if msg.err != nil && (github.IsRateLimited(msg.err) || errors.Is(msg.err, context.Canceled) || m.ctx.Err() != nil) {
		for _, g := range b.queue {
			delete(m.writing, g.ID)
		}
		stopped, b.queue = len(b.queue), nil
	}
	if len(b.queue) > 0 {
		m.batchProgress()
		return m.nextDelete()
	}
	m.batch = nil
	if b.deleted == b.total {
		m.line.Say(m.now(), "✓ deleted "+gistCount(b.total))
		return nil
	}
	left := b.total - b.deleted
	what := fmt.Sprintf("%d failed", b.failed)
	if stopped > 0 {
		what = fmt.Sprintf("%d not sent", stopped)
		if b.failed > 0 {
			what = fmt.Sprintf("%d failed, %d not sent", b.failed, stopped)
		}
	}
	// The error goes last: it can be long, and is cut first
	m.line.Fail(m.now(), fmt.Sprintf("deleted %d of %d · %s · %s selects %s again · %v",
		b.deleted, b.total, what, m.keys.Hint(keymap.ClearSelection), plural(left, "it", "them"), b.firstErr))
	return nil
}

// deleteFixed is the rows of the box of many that are not the gists: the
// borders, the blank rows, the two sentences, the notes and the question.
const deleteFixed = 8

// deleteRows is how many of n gists the box shows, with notes lines of notes.
func (m *Model) deleteRows(n, notes int) int {
	return max(1, min(n, m.height-4-deleteFixed-notes))
}

// deleteNotes are what the box of many says of the gists it lists.
func (m *Model) deleteNotes(gs []model.Gist) []string {
	shown := map[string]bool{}
	for _, r := range m.tabs[tabMine].rows {
		shown[r.gist.ID] = true
	}
	hidden, drafts := 0, 0
	for _, g := range gs {
		if m.writing[g.ID] != "" {
			continue
		}
		if !shown[g.ID] {
			hidden++
		}
		if m.drafts[g.ID] {
			drafts++
		}
	}
	var notes []string
	if hidden > 0 {
		notes = append(notes, fmt.Sprintf("%d %s not shown by the filter.", hidden, plural(hidden, "is", "are")))
	}
	if drafts > 0 {
		notes = append(notes, fmt.Sprintf("%d %s; %s too.", drafts, plural(drafts, "has a draft", "have drafts"), plural(drafts, "it goes", "they go")))
	}
	return notes
}

func (m *Model) drawDeleteMany(c *canvas) {
	p := m.pal
	d := m.deleting
	var gs []model.Gist
	for _, id := range d.many {
		if i := slices.IndexFunc(m.tabs[tabMine].gists, func(g model.Gist) bool { return g.ID == id }); i >= 0 {
			gs = append(gs, m.tabs[tabMine].gists[i])
		}
	}
	n := 0
	for _, g := range gs {
		if m.writing[g.ID] == "" {
			n++
		}
	}
	notes := m.deleteNotes(gs)
	rows := m.deleteRows(len(gs), len(notes))
	d.top = max(0, min(d.top, len(gs)-rows))
	w := min(76, c.w-8)
	h := deleteFixed + len(notes) + rows
	x, y := m.boxAt(w, h, 3)
	c.box(x, y, w, h, p.red, nil)
	title(c, x, y, "delete "+gistCount(n), style{fg: p.red, bold: true})
	lx, iw := x+3, w-6
	nameW := 0
	for _, g := range gs {
		nameW = max(nameW, text.Width(firstName(g)))
	}
	nameW = min(nameW, 28, iw/3)
	for i, g := range gs[d.top:min(d.top+rows, len(gs))] {
		ly := y + 2 + i
		sending := m.writing[g.ID] != ""
		id, name, desc := style{fg: p.overlay}, style{fg: p.text, bold: true}, style{fg: p.subtext0}
		tag, tagSt := "", style{fg: p.peach}
		switch {
		case sending:
			id, name, desc = style{fg: p.dim}, style{fg: p.dim}, style{fg: p.dim}
			tag, tagSt = "being sent", style{fg: p.dim}
		case m.drafts[g.ID]:
			tag = draftMark + " draft"
		}
		room := iw
		if tag != "" {
			room -= text.Width(tag) + 2
			c.put(ly, lx+iw-text.Width(tag), tag, tagSt)
		}
		fx := c.put(ly, lx, shortID(g.ID)+"  ", id)
		c.putCut(ly, fx, []seg{{firstName(g), name}}, min(nameW, max(0, lx+room-fx)), style{fg: p.overlay})
		dx := fx + nameW + 2
		if dw := lx + room - dx; dw > 0 {
			if g.Description != "" {
				c.putCut(ly, dx, []seg{{text.Line(g.Description), desc}}, dw, style{fg: p.overlay})
			} else {
				c.putCut(ly, dx, []seg{{"(no description)", style{fg: p.dim}}}, dw, style{fg: p.overlay})
			}
		}
	}
	m.drawThumb(c, x+w-1, y+2, rows, d.top, len(gs), p.text)
	ty := y + 2 + rows + 1
	c.putCut(ty, lx, []seg{{fmt.Sprintf("This deletes %s and their revisions on GitHub.", gistCount(n)), style{fg: p.text}}}, iw, style{fg: p.overlay})
	c.put(ty+1, lx, "It cannot be undone.", style{fg: p.red})
	for i, note := range notes {
		c.putCut(ty+2+i, lx, []seg{{note, style{fg: p.subtext0}}}, iw, style{fg: p.overlay})
	}
	fy := y + h - 2
	these := "these " + gistCount(n)
	if n == 1 {
		these = "this gist"
	}
	fx := c.put(fy, lx, "delete "+these+"? ", style{fg: p.text})
	m.keyLine(c, fy, fx, []hint{{m.keys.Hint(keymap.Confirm), "yes"}, {keymap.Back, "no"}})
}

// firstName is the first file of a gist, with how many more there are.
func firstName(g model.Gist) string {
	if len(g.Files) == 0 {
		return ""
	}
	s := text.Line(g.Files[0].Name)
	if n := len(g.Files) - 1; n > 0 {
		s += fmt.Sprintf(" +%d", n)
	}
	return s
}
