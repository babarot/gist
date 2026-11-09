package tui

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/tui/keymap"
)

// Keys are looked up in layers, top first, and a layer that takes a key stops
// it there: ctrl+c, then what is open over the screen, then the list. A layer
// gives no key to the one below, so a box cannot act on the list under it.
func (m *Model) key(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	if k == keymap.Interrupt {
		return m.quit()
	}
	// What is on its way must end first: nothing is sure of its state until then
	if m.sending {
		return nil
	}
	switch m.uiState() {
	case uiAuthFail:
		if m.keys.Matches(keymap.Quit, k) {
			return m.quit()
		}
		return nil
	case uiHelp:
		if k == keymap.Back || m.keys.Matches(keymap.Help, k) {
			m.helpOpen = false
		}
		if m.keys.Matches(keymap.ReleaseNotes, k) {
			return m.openReleaseNotes()
		}
		return nil
	case uiReleaseNotes:
		m.releaseBoxKey(k)
		return nil
	case uiConfirmPush:
		return m.diffKey(k)
	case uiConfirmConflict:
		return m.conflictKey(k)
	case uiInput, uiInputConflict:
		return m.inputKey(msg)
	case uiConfirmDelete:
		return m.deleteKey(k)
	case uiConfirmDiscard:
		m.discardKey(k)
		return nil
	case uiFilter:
		return m.filterKey(msg)
	case uiNew:
		return m.formKey(msg)
	}
	return m.listKey(k)
}

// quit ends. An edit past the editor is in a draft already, and the lock goes
// with the process. One whose files were not read back yet is read back first:
// they would be left where nobody knows of them.
func (m *Model) quit() tea.Cmd {
	if e := m.editing; e != nil && e.work != nil && !e.ready {
		if _, err := e.work.ReadBack(e.session); err != nil {
			m.quitErr = err
		}
	}
	// What the form had goes with it, as closing the form does
	if m.form != nil && m.form.scratch != nil {
		_ = m.form.scratch.Remove()
	}
	m.cancel()
	return tea.Quit
}

func (m *Model) listKey(k string) tea.Cmd {
	on := func(a keymap.Action) bool { return m.keys.Matches(a, k) }
	t := m.tab()
	switch {
	case on(keymap.Quit):
		return m.quit()
	case on(keymap.Help):
		m.helpOpen = true
	case on(keymap.Down):
		m.moveTo(t.cur + 1)
	case on(keymap.Up):
		m.moveTo(t.cur - 1)
	case on(keymap.PageDown):
		m.moveTo(t.cur + m.layout().listRows)
	case on(keymap.PageUp):
		m.moveTo(t.cur - m.layout().listRows)
	case on(keymap.Top):
		m.moveTo(0)
	case on(keymap.Bottom):
		m.moveTo(len(t.rows) - 1)
	case on(keymap.NextFile):
		m.stepFile(1)
	case on(keymap.PrevFile):
		m.stepFile(-1)
	case on(keymap.Mine):
		m.cur = tabMine
		m.keepCursorShown()
	case on(keymap.Starred):
		return m.showStarred()
	case on(keymap.Refresh):
		return m.refresh()
	case on(keymap.Browse):
		return m.browse()
	case on(keymap.CopyURL):
		return m.copyURL()
	case on(keymap.CopyFile):
		return m.copyFile()
	case on(keymap.ScrollDown):
		m.scroll(scrollStep)
	case on(keymap.ScrollUp):
		m.scroll(-scrollStep)
	case on(keymap.ToggleSide):
		m.toggleSide()
		return m.readCommentsNow()
	case on(keymap.ToggleMD):
		m.plain = !m.plain
	case on(keymap.PreviewGrow):
		m.resizeBox(m.layout().boxH + 1)
	case on(keymap.PreviewShrink):
		m.resizeBox(m.layout().boxH - 1)
	case on(keymap.Edit):
		return m.startEdit()
	case on(keymap.Delete):
		return m.openDelete()
	case on(keymap.Describe):
		return m.openInput(inputDescription)
	case on(keymap.Rename):
		return m.openInput(inputFilename)
	case on(keymap.Discard):
		return m.openDiscard()
	case on(keymap.Filter):
		return m.openFilter()
	case on(keymap.New):
		return m.openForm()
	case on(keymap.Select):
		m.toggleSelect()
	case on(keymap.SelectShown):
		m.selectShown()
	case on(keymap.ClearSelection):
		m.clearOrRestore()
	case k == keymap.Back && len(m.selected) > 0:
		m.clearSelection()
	case k == keymap.Back:
		m.setFilter("")
	}
	return nil
}

// scrollStep is how many lines J and K move.
const scrollStep = 3

func (m *Model) toggleSide() {
	if m.side == sideInfo {
		m.side = sideComments
	} else {
		m.side = sideInfo
	}
}

// stepFile moves along the file tabs of the gist, stopping at the ends.
func (m *Model) stepFile(by int) {
	g, ok := m.current()
	if !ok {
		return
	}
	m.fileTab[g.ID] = max(0, min(m.selectedFile(g)+by, len(g.Files)-1))
}

// showStarred shows the starred tab, reading it the first time it is shown.
func (m *Model) showStarred() tea.Cmd {
	m.cur = tabStarred
	m.keepCursorShown()
	if t := m.tab(); !t.readOnce && !t.reading && m.viewerRead {
		return m.readStarred()
	}
	return nil
}

func (m *Model) refresh() tea.Cmd {
	clear(m.preview.failed)
	clear(m.preview.failedComments)
	if !m.viewerRead {
		if m.readingViewer {
			return nil
		}
		return m.readViewer()
	}
	if m.cur == tabStarred {
		return m.readStarred()
	}
	return m.readMine()
}

type browsedMsg struct{ err error }

func (m *Model) browse() tea.Cmd {
	if len(m.selected) > 0 {
		return m.browseSelected()
	}
	g, ok := m.current()
	if !ok {
		return nil
	}
	return func() tea.Msg { return browsedMsg{m.opt.Browse(g.URL)} }
}

func (m *Model) copyURL() tea.Cmd {
	if len(m.selected) > 0 {
		return m.copySelectedURLs()
	}
	g, ok := m.current()
	if !ok {
		return nil
	}
	m.line.Say(m.now(), "✓ copied "+g.URL)
	return tea.SetClipboard(g.URL)
}

// maxCopy is the most that goes to the clipboard: it goes through the terminal,
// and past about this some terminals drop it, saying nothing.
const maxCopy = 1 << 20

// copyMsg is a file read to be copied.
type copyMsg struct {
	name, content string
	err           error
}

// copyFile copies the file shown, as it is written: what the preview has is
// cleaned for drawing, and cut at a megabyte.
func (m *Model) copyFile() tea.Cmd {
	if m.oneAtATime() {
		return nil
	}
	g, ok := m.current()
	if !ok || len(g.Files) == 0 {
		return nil
	}
	f := g.Files[m.selectedFile(g)]
	if f.Size > maxCopy {
		m.line.Refuse(m.now(), f.Name+" is too large to copy: use gist view")
		return nil
	}
	ctx, src := m.ctx, m.opt.Source
	return func() tea.Msg {
		content, err := src.Raw(ctx, f)
		return copyMsg{f.Name, content, err}
	}
}

func (m *Model) gotCopy(msg copyMsg) tea.Cmd {
	switch {
	case msg.err != nil:
		m.line.Fail(m.now(), fmt.Sprintf("%s cannot be read: %v", msg.name, msg.err))
		return nil
	case !utf8.ValidString(msg.content) || strings.ContainsRune(msg.content, 0):
		m.line.Refuse(m.now(), "a binary file cannot be copied: use gist view")
		return nil
	}
	m.line.Say(m.now(), fmt.Sprintf("✓ copied %s (%s)", msg.name, size(int64(len(msg.content)))))
	return tea.SetClipboard(msg.content)
}

// hint is a key and what it does, for the key line.
type hint struct{ key, what string }

// keyHints are the keys that work now.
func (m *Model) keyHints() []hint {
	k := func(what string, actions ...keymap.Action) hint { return hint{m.keys.Hint(actions...), what} }
	switch m.uiState() {
	case uiAuthFail:
		return []hint{k("quit", keymap.Quit)}
	case uiHelp:
		if m.opt.Release != nil {
			return []hint{k("release notes", keymap.ReleaseNotes), {keymap.Back, "close"}}
		}
		return []hint{{keymap.Back, "close"}}
	case uiReleaseNotes:
		return []hint{k("scroll", keymap.Down, keymap.Up), k("top / bottom", keymap.Top, keymap.Bottom), {keymap.Back, "back"}}
	case uiConfirmPush:
		return []hint{k("push", keymap.Confirm), k("edit again", keymap.EditAgain), k("scroll", keymap.ScrollDown, keymap.ScrollUp), {keymap.Back, "keep as draft"}}
	case uiConfirmConflict:
		return []hint{k("overwrite", keymap.Overwrite), k("reload", keymap.Reload), {keymap.Back, "keep as draft"}}
	case uiConfirmDelete:
		if m.deleting != nil && m.deleting.many != nil {
			return []hint{k("delete", keymap.Confirm), k("scroll", keymap.Down, keymap.Up), {keymap.Back, "no"}}
		}
		if m.deleting != nil && len(m.deleting.gist.Files) > 1 {
			return []hint{k("delete gist", keymap.Confirm), k("only this file", keymap.OnlyFile), {keymap.Back, "no"}}
		}
		return []hint{k("delete", keymap.Confirm), {keymap.Back, "no"}}
	case uiConfirmDiscard:
		return []hint{k("discard", keymap.Confirm), {keymap.Back, "keep"}}
	case uiInput:
		return []hint{{"enter", "save"}, {keymap.Back, "cancel"}}
	case uiInputConflict:
		return []hint{k("overwrite", keymap.Overwrite), k("reload", keymap.Reload), {keymap.Back, "cancel"}}
	case uiFilter:
		return []hint{{"enter", "keep"}, {"tab", "complete"}, {keymap.Back, "clear"}, {"↑/↓", "move"}}
	case uiNew:
		return []hint{{"tab", "next"}, k("$EDITOR", keymap.OpenEditor), k("create", keymap.Create), {keymap.Back, "cancel"}}
	}
	if n := len(m.selected); n > 0 {
		return []hint{k("delete "+strconv.Itoa(n), keymap.Delete), k("copy URLs", keymap.CopyURL), k("browser", keymap.Browse),
			k("select", keymap.Select), k("select shown", keymap.SelectShown),
			{m.keys.Hint(keymap.ClearSelection) + "/" + keymap.Back, "clear"}, k("filter", keymap.Filter), k("refresh", keymap.Refresh), k("keys", keymap.Help)}
	}
	starred := m.cur == tabStarred
	var hints []hint
	if m.canRestore() {
		hints = append(hints, k("restore selection", keymap.ClearSelection))
	}
	if m.tab().query != "" {
		hints = append(hints, hint{keymap.Back, "clear filter"})
	}
	if !starred {
		hints = append(hints, k("edit", keymap.Edit), k("new", keymap.New))
	}
	hints = append(hints, k("browser", keymap.Browse), k("copy url", keymap.CopyURL))
	if !starred {
		hints = append(hints, k("delete", keymap.Delete), k("desc", keymap.Describe), k("rename", keymap.Rename))
	}
	if g, ok := m.current(); ok {
		if !starred && m.drafts[g.ID] {
			hints = append(hints, k("discard draft", keymap.Discard))
		}
		if len(g.Files) > 1 {
			hints = append(hints, k("file", keymap.PrevFile, keymap.NextFile))
		}
		if m.side == sideComments {
			hints = append(hints, k("info", keymap.ToggleSide), k("comments", keymap.ScrollDown, keymap.ScrollUp))
		} else {
			hints = append(hints, k("comments", keymap.ToggleSide))
		}
		if m.onMarkdown(g) {
			what := "plain"
			if m.plain {
				what = "rendered"
			}
			hints = append(hints, k(what, keymap.ToggleMD))
		}
	}
	hints = append(hints, k("filter", keymap.Filter))
	if starred {
		hints = append(hints, k("mine", keymap.Mine))
	}
	hints = append(hints, k("refresh", keymap.Refresh))
	return append(hints, k("keys", keymap.Help))
}

// helpGroup is a column of the key list.
type helpGroup struct {
	title string
	rows  []helpRow
}

type helpRow struct {
	actions []keymap.Action
	what    string
}

// helpGroups are the key list. The tests check each action is in it.
var helpGroups = []helpGroup{
	{"move", []helpRow{
		{[]keymap.Action{keymap.Down, keymap.Up}, "down / up"},
		{[]keymap.Action{keymap.Top, keymap.Bottom}, "top / bottom"},
		{[]keymap.Action{keymap.PageDown, keymap.PageUp}, "page down / up"},
		{[]keymap.Action{keymap.ScrollDown, keymap.ScrollUp}, "scroll the preview"},
		{[]keymap.Action{keymap.PrevFile, keymap.NextFile}, "prev / next file"},
		{[]keymap.Action{keymap.Mine}, "mine"},
		{[]keymap.Action{keymap.Starred}, "starred"},
	}},
	{"act", []helpRow{
		{[]keymap.Action{keymap.Edit}, "edit, then push"},
		{[]keymap.Action{keymap.New}, "new gist"},
		{[]keymap.Action{keymap.Describe}, "edit the description"},
		{[]keymap.Action{keymap.Rename}, "rename the file"},
		{[]keymap.Action{keymap.Delete}, "delete"},
		{[]keymap.Action{keymap.Discard}, "discard the draft"},
		{[]keymap.Action{keymap.Select}, "select"},
		{[]keymap.Action{keymap.SelectShown}, "select shown"},
		{[]keymap.Action{keymap.ClearSelection}, "clear / restore"},
	}},
	{"view", []helpRow{
		{[]keymap.Action{keymap.ToggleSide}, "info / comments"},
		{[]keymap.Action{keymap.ToggleMD}, "markdown / plain"},
		{[]keymap.Action{keymap.PreviewGrow, keymap.PreviewShrink}, "preview height"},
		{[]keymap.Action{keymap.Filter}, "fuzzy filter"},
		{[]keymap.Action{keymap.Browse}, "open in the browser"},
		{[]keymap.Action{keymap.CopyURL}, "copy the URL"},
		{[]keymap.Action{keymap.CopyFile}, "copy the file"},
		{[]keymap.Action{keymap.Refresh}, "refresh now"},
		{[]keymap.Action{keymap.Help}, "this help"},
		{[]keymap.Action{keymap.Quit}, "quit"},
	}},
}
