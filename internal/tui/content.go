package tui

import (
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/cache"
	"github.com/babarot/gist/v2/internal/highlight"
	"github.com/babarot/gist/v2/internal/markdown"
	"github.com/babarot/gist/v2/internal/model"
)

// Which pane is beside the content.
const (
	sideInfo = iota
	sideComments
)

const (
	// maxReads is how many gists are read at once.
	maxReads = 4
	// commentsAfter is how long the cursor rests on a gist before its comments
	// are read, so holding j does not read those of every row it passes.
	commentsAfter = 250 * time.Millisecond
)

// fileKey names a file of a gist, and what was worked out from it.
type fileKey struct {
	gist, file string
}

// drawnKey names a file drawn at a width, from a content.
type drawnKey struct {
	fileKey
	updated time.Time
	width   int
}

// previewState is what the preview has read and worked out.
type previewState struct {
	dir  string
	disk *cache.Contents
	// contents are the gists read whole, and reading those on their way.
	contents map[string]model.Gist
	reading  map[string]bool
	failed   map[string]bool
	// notKept are the gists the disk did not have, which wait for GitHub to
	// confirm the account before being asked for.
	notKept map[string]bool
	running int
	// comments are by gist, the same.
	comments        map[string][]model.Comment
	readingComments map[string]bool
	failedComments  map[string]bool
	// dwellOn and dwellSeq tell the gist the cursor rests on.
	dwellOn  string
	dwellSeq int

	code   map[fileKey]codeLines
	langs  map[fileKey]fileLang
	md     map[drawnKey][]markdown.Line
	notes  map[drawnKey][]markdown.Line
	scroll map[fileKey]int
	// commentScroll is by gist.
	commentScroll map[string]int
}

// fileLang is the language of a file, for the content it was told from.
type fileLang struct {
	updated time.Time
	name    string
}

// codeLines is a file highlighted, for the content it was made from.
type codeLines struct {
	updated time.Time
	lines   [][]highlight.Token
}

func newPreviewState(dir string) previewState {
	return previewState{
		dir:             dir,
		disk:            cache.OpenContents(dir),
		contents:        map[string]model.Gist{},
		reading:         map[string]bool{},
		failed:          map[string]bool{},
		notKept:         map[string]bool{},
		failedComments:  map[string]bool{},
		langs:           map[fileKey]fileLang{},
		comments:        map[string][]model.Comment{},
		readingComments: map[string]bool{},
		code:            map[fileKey]codeLines{},
		md:              map[drawnKey][]markdown.Line{},
		notes:           map[drawnKey][]markdown.Line{},
		scroll:          map[fileKey]int{},
		commentScroll:   map[string]int{},
	}
}

// forget drops what was read of a gist and worked out from it.
func (p *previewState) forget(id string) {
	delete(p.contents, id)
	delete(p.comments, id)
	delete(p.failed, id)
	delete(p.failedComments, id)
	delete(p.notKept, id)
	for k := range p.md {
		if k.gist == id {
			delete(p.md, k)
		}
	}
	for k := range p.notes {
		if k.gist == id {
			delete(p.notes, k)
		}
	}
	for k := range p.code {
		if k.gist == id {
			delete(p.code, k)
		}
	}
	for k := range p.langs {
		if k.gist == id {
			delete(p.langs, k)
		}
	}
}

// maxDrawn is how many drawn files or comments are held. Each can be drawn again
// from its content, so past this they all go.
const maxDrawn = 256

// content is the gist read whole, if what was read is of the gist as listed or
// newer: a gist can change after the list was read, and a read that came back
// newer must count, or it would be read again for ever.
func (m *Model) content(g model.Gist) (model.Gist, bool) {
	c, ok := m.preview.contents[g.ID]
	return c, ok && !c.Updated.Before(g.Updated)
}

type contentMsg struct {
	id      string
	gist    model.Gist
	notKept bool
	err     error
}

// readContents reads the gists shown that have not been: the cursor's first,
// then the rows on the screen. What the disk kept for the same update is used
// without asking GitHub.
func (m *Model) readContents() tea.Cmd {
	if m.opt.Source == nil {
		return nil
	}
	var cmds []tea.Cmd
	t := m.tab()
	last := min(t.top+m.layout().listRows, len(t.gists))
	wanted := make([]model.Gist, 0, last-t.top+1)
	if g, ok := m.selected(); ok {
		wanted = append(wanted, g)
	}
	wanted = append(wanted, t.gists[min(t.top, last):last]...)
	for _, g := range wanted {
		if m.preview.running == maxReads {
			break
		}
		if _, ok := m.content(g); ok || m.preview.reading[g.ID] || m.preview.failed[g.ID] {
			continue
		}
		if m.preview.notKept[g.ID] && !m.viewerRead {
			continue
		}
		m.preview.reading[g.ID] = true
		m.preview.running++
		cmds = append(cmds, m.readContent(g))
	}
	return tea.Batch(cmds...)
}

func (m *Model) readContent(g model.Gist) tea.Cmd {
	disk, src, ctx, ask := m.preview.disk, m.opt.Source, m.ctx, m.viewerRead
	return func() tea.Msg {
		if c, ok := disk.Load(g.ID, g.Updated); ok {
			return contentMsg{id: g.ID, gist: c}
		}
		// A kept list can be another account's until GitHub says whose the
		// token is, so nothing is asked for before that
		if !ask {
			return contentMsg{id: g.ID, notKept: true}
		}
		c, err := src.Get(ctx, g.ID)
		if err != nil {
			return contentMsg{id: g.ID, err: err}
		}
		// Kept for the next launch. Losing this costs one read
		_ = disk.Save(c)
		return contentMsg{id: g.ID, gist: c}
	}
}

func (m *Model) gotContent(msg contentMsg) {
	delete(m.preview.reading, msg.id)
	m.preview.running--
	switch {
	case msg.notKept:
		m.preview.notKept[msg.id] = true
	case msg.err != nil:
		m.preview.failed[msg.id] = true
		m.line.Fail(m.now(), msg.err.Error())
	case !m.listed(msg.id):
		// It went from the list while being read, after what was kept of it
		// was removed: the read just wrote it back
		m.removeKept(msg.id)
	default:
		m.preview.contents[msg.id] = msg.gist
	}
}

// listed says a tab has the gist.
func (m *Model) listed(id string) bool {
	for i := range m.tabs {
		if slices.ContainsFunc(m.tabs[i].gists, func(g model.Gist) bool { return g.ID == id }) {
			return true
		}
	}
	return false
}

func (m *Model) removeKept(id string) {
	if err := m.preview.disk.Remove(id); err != nil {
		m.opt.Log.Warn("the kept content cannot be removed", "gist", id, "err", err.Error())
	}
}

type dwellMsg struct {
	seq int
	id  string
}

// dwell starts the wait for the cursor to rest on a gist.
func (m *Model) dwell() tea.Cmd {
	g, ok := m.selected()
	if !ok || m.opt.Source == nil {
		return nil
	}
	if m.side == sideComments {
		m.preview.dwellOn = g.ID
		return m.readComments(g)
	}
	if g.ID == m.preview.dwellOn {
		return nil
	}
	m.preview.dwellOn = g.ID
	m.preview.dwellSeq++
	if m.clockless {
		return nil
	}
	seq, id := m.preview.dwellSeq, g.ID
	return tea.Tick(commentsAfter, func(time.Time) tea.Msg { return dwellMsg{seq, id} })
}

// dwelled reads the comments of the gist the cursor stayed on.
func (m *Model) dwelled(msg dwellMsg) tea.Cmd {
	if msg.seq != m.preview.dwellSeq {
		return nil
	}
	if g, ok := m.selected(); ok && g.ID == msg.id {
		return m.readComments(g)
	}
	return nil
}

// readCommentsNow reads the comments of the gist under the cursor, once they
// are asked for.
func (m *Model) readCommentsNow() tea.Cmd {
	g, ok := m.selected()
	if !ok || m.side != sideComments {
		return nil
	}
	return m.readComments(g)
}

type commentsMsg struct {
	id       string
	comments []model.Comment
	err      error
}

// readComments reads a gist's comments unless it has none or they were read.
func (m *Model) readComments(g model.Gist) tea.Cmd {
	_, done := m.preview.comments[g.ID]
	if g.Comments == 0 || done || m.preview.readingComments[g.ID] || m.preview.failedComments[g.ID] {
		return nil
	}
	m.preview.readingComments[g.ID] = true
	src, ctx, id := m.opt.Source, m.ctx, g.ID
	return func() tea.Msg {
		cs, err := src.Comments(ctx, id)
		return commentsMsg{id, cs, err}
	}
}

func (m *Model) gotComments(msg commentsMsg) {
	delete(m.preview.readingComments, msg.id)
	if msg.err != nil {
		m.preview.failedComments[msg.id] = true
		m.line.Fail(m.now(), msg.err.Error())
		return
	}
	m.preview.comments[msg.id] = msg.comments
}
