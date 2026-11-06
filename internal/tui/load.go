package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/cache"
	"github.com/babarot/gist/v2/internal/model"
)

// perPage is how many gists a page of the list holds, for telling how many
// there will be.
const perPage = 30

// pagesBuffered is how many pages can wait for the screen, which must not hold
// up the reading.
const pagesBuffered = 64

type viewerMsg struct {
	login string
	err   error
}

func (m *Model) readViewer() tea.Cmd {
	return func() tea.Msg {
		login, err := m.opt.Source.Viewer(m.ctx)
		return viewerMsg{login, err}
	}
}

// gotViewer starts the reading. Lists kept for another account are dropped.
func (m *Model) gotViewer(msg viewerMsg) tea.Cmd {
	if msg.err != nil {
		m.line.Fail(m.now(), msg.err.Error())
		return nil
	}
	if msg.login != m.login {
		for i := range m.tabs {
			t := &m.tabs[i]
			t.gists, t.list, t.loaded, t.cur, t.top = nil, cache.List{}, false, 0, 0
		}
		m.openStores(msg.login)
	}
	cmds := []tea.Cmd{m.readMine()}
	if m.cur == tabStarred {
		cmds = append(cmds, m.readStarred())
	}
	return tea.Batch(cmds...)
}

type pageMsg struct {
	seq  int
	page model.ListPage
	next <-chan tea.Msg
}

type listDoneMsg struct {
	seq   int
	pages []model.ListPage
	err   error
}

// readMine reads the list, pages coming in one by one. Only the pages that
// changed since the list kept are read again.
func (m *Model) readMine() tea.Cmd {
	t := &m.tabs[tabMine]
	if t.reading {
		return nil
	}
	t.reading, t.first, t.got, t.estimate = true, !t.loaded, 0, 0
	t.seq++
	seq, etags := t.seq, t.list.ETags()
	pages := make(chan tea.Msg, pagesBuffered)
	go func() {
		defer close(pages)
		done, err := m.opt.Source.List(m.ctx, etags, func(p model.ListPage) {
			select {
			case pages <- pageMsg{seq: seq, page: p}:
			case <-m.ctx.Done():
			}
		})
		pages <- listDoneMsg{seq, done, err}
	}()
	return waitFor(pages)
}

// waitFor is the next message of a reading.
func waitFor(from <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-from
		if !ok {
			return nil
		}
		if p, isPage := msg.(pageMsg); isPage {
			p.next = from
			return p
		}
		return msg
	}
}

// gotPage adds a page to a list being read for the first time. A list already
// shown changes all at once at the end.
func (m *Model) gotPage(msg pageMsg) tea.Cmd {
	t := &m.tabs[tabMine]
	if msg.seq != t.seq {
		return nil
	}
	if t.first {
		gists := append(slices.Clip(t.gists), msg.page.Gists...)
		cache.Sort(gists)
		t.loaded = true
		m.replace(t, gists)
		t.got = len(gists)
		if msg.page.Number == 1 {
			t.estimate = msg.page.Last * perPage
		}
	}
	return waitFor(msg.next)
}

// gotList ends a reading: the list becomes what was read, and is kept.
func (m *Model) gotList(msg listDoneMsg) tea.Cmd {
	t := &m.tabs[tabMine]
	if msg.seq != t.seq {
		return nil
	}
	t.reading = false
	if msg.err != nil {
		m.line.Fail(m.now(), msg.err.Error())
		return nil
	}
	first := t.first
	old := t.gists
	l := cache.Merge(t.list, msg.pages, m.now())
	t.first = false
	t.list, t.loaded = l, true
	m.keep(t, l)
	if first {
		m.replace(t, l.Gists)
		m.line.Say(m.now(), fmt.Sprintf("✓ %d gists · saved to the cache", len(l.Gists)))
		return nil
	}
	added, updated, gone := m.compare(t, old, l.Gists)
	m.replace(t, l.Gists)
	m.line.Say(m.now(), summary(added, updated, gone))
	return nil
}

// keep saves the list. A list not kept costs the next launch a reading, which
// is no reason to stop.
func (m *Model) keep(t *tab, l cache.List) {
	if t.store == nil {
		return
	}
	if err := t.store.Save(l); err != nil {
		m.opt.Log.Warn("the list cannot be kept", "err", err.Error())
	}
}

// compare marks what is new or changed and counts it.
func (m *Model) compare(t *tab, old, now []model.Gist) (added, updated, gone int) {
	before := make(map[string]model.Gist, len(old))
	for _, g := range old {
		before[g.ID] = g
	}
	mark := func(id string) {
		t.marks[id] = true
		t.fresh[id] = m.now()
	}
	for _, g := range now {
		was, ok := before[g.ID]
		switch {
		case !ok:
			added++
			mark(g.ID)
		case g.Updated.After(was.Updated):
			updated++
			mark(g.ID)
		}
		delete(before, g.ID)
	}
	return added, updated, len(before)
}

func summary(added, updated, gone int) string {
	var parts []string
	for _, c := range []struct {
		n    int
		what string
	}{{added, "new"}, {updated, "updated"}, {gone, "gone"}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, c.what))
		}
	}
	if len(parts) == 0 {
		return "up to date"
	}
	return "✓ " + strings.Join(parts, " · ")
}

type starredMsg struct {
	seq   int
	gists []model.Gist
	err   error
}

func (m *Model) readStarred() tea.Cmd {
	t := &m.tabs[tabStarred]
	if t.reading {
		return nil
	}
	t.reading = true
	t.seq++
	seq := t.seq
	return func() tea.Msg {
		gists, err := m.opt.Source.Starred(m.ctx)
		return starredMsg{seq, gists, err}
	}
}

func (m *Model) gotStarred(msg starredMsg) {
	t := &m.tabs[tabStarred]
	if msg.seq != t.seq {
		return
	}
	t.reading = false
	if msg.err != nil {
		m.line.Fail(m.now(), msg.err.Error())
		return
	}
	cache.Sort(msg.gists)
	had := t.loaded
	added, updated, gone := m.compare(t, t.gists, msg.gists)
	if !had {
		clear(t.marks)
		clear(t.fresh)
	}
	l := cache.List{Gists: msg.gists, Read: m.now()}
	t.list, t.loaded = l, true
	m.keep(t, l)
	m.replace(t, msg.gists)
	if had {
		m.line.Say(m.now(), summary(added, updated, gone))
	}
}
