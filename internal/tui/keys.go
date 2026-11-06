package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/tui/keymap"
)

// Keys are looked up in layers, top first, and a layer that takes a key stops
// it there: ctrl+c, then what is open over the screen, then the list. A layer
// gives no key to the one below, so a box cannot act on the list under it.
func (m *Model) key(k string) tea.Cmd {
	if k == keymap.Interrupt {
		return m.quit()
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
		return nil
	}
	return m.listKey(k)
}

func (m *Model) quit() tea.Cmd {
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
	case on(keymap.Top):
		m.moveTo(0)
	case on(keymap.Bottom):
		m.moveTo(len(t.gists) - 1)
	case on(keymap.NextFile):
		m.stepFile(1)
	case on(keymap.PrevFile):
		m.stepFile(-1)
	case on(keymap.Mine):
		m.cur = tabMine
	case on(keymap.Starred):
		return m.showStarred()
	case on(keymap.Refresh):
		return m.refresh()
	case on(keymap.Browse):
		return m.browse()
	case on(keymap.CopyURL):
		return m.copyURL()
	case on(keymap.ScrollDown):
		m.scroll(scrollStep)
	case on(keymap.ScrollUp):
		m.scroll(-scrollStep)
	case on(keymap.ToggleSide):
		m.toggleSide()
		return m.readCommentsNow()
	case on(keymap.ToggleMD):
		m.plain = !m.plain
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
	g, ok := m.selected()
	if !ok {
		return
	}
	m.fileTab[g.ID] = max(0, min(m.selectedFile(g)+by, len(g.Files)-1))
}

// showStarred shows the starred tab, reading it the first time it is shown.
func (m *Model) showStarred() tea.Cmd {
	m.cur = tabStarred
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
	g, ok := m.selected()
	if !ok {
		return nil
	}
	return func() tea.Msg { return browsedMsg{m.opt.Browse(g.URL)} }
}

func (m *Model) copyURL() tea.Cmd {
	g, ok := m.selected()
	if !ok {
		return nil
	}
	m.line.Say(m.now(), "✓ copied "+g.URL)
	return tea.SetClipboard(g.URL)
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
		return []hint{{keymap.Back, "close"}}
	}
	hints := []hint{k("browser", keymap.Browse), k("copy url", keymap.CopyURL)}
	if g, ok := m.selected(); ok {
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
	if m.cur == tabStarred {
		hints = append(hints, k("mine", keymap.Mine))
	} else {
		hints = append(hints, k("starred", keymap.Starred))
	}
	return append(hints, k("refresh", keymap.Refresh), k("keys", keymap.Help), k("quit", keymap.Quit))
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
		{[]keymap.Action{keymap.ScrollDown, keymap.ScrollUp}, "scroll the preview"},
		{[]keymap.Action{keymap.PrevFile, keymap.NextFile}, "prev / next file"},
		{[]keymap.Action{keymap.Mine}, "mine"},
		{[]keymap.Action{keymap.Starred}, "starred"},
	}},
	{"view", []helpRow{
		{[]keymap.Action{keymap.ToggleSide}, "info / comments"},
		{[]keymap.Action{keymap.ToggleMD}, "markdown / plain"},
		{[]keymap.Action{keymap.Browse}, "open in the browser"},
		{[]keymap.Action{keymap.CopyURL}, "copy the URL"},
		{[]keymap.Action{keymap.Refresh}, "refresh now"},
		{[]keymap.Action{keymap.Help}, "this help"},
		{[]keymap.Action{keymap.Quit}, "quit"},
	}},
}
