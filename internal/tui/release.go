package tui

import (
	"cmp"
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/markdown"
	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/release"
	"github.com/babarot/gist/v2/internal/text"
	"github.com/babarot/gist/v2/internal/tui/keymap"
)

// Release is what gist knows of its own releases at start, and where it reads
// them. The app package fills it in.
type Release struct {
	// Version is the build as it is shown, and Running the version it is of.
	Version, Running string
	// How says how this install is updated.
	How string
	// Tell tells of a newer release. Without it the releases are read only when
	// their notes are opened.
	Tell bool
	// FromSource is a build that is no release: Running names the last release
	// but its code may be newer, so no release is marked as installed.
	FromSource bool
	// Latest is the newest release a reading before this run found, and Told the
	// newest already told of.
	Latest, Told string
	// Due reads the releases at start.
	Due    bool
	Source ReleaseSource
}

// ReleaseSource reads gist's releases and keeps what was found.
type ReleaseSource interface {
	Releases(ctx context.Context) ([]model.Release, error)
	// Checked keeps that the releases were read now and the newest found; empty
	// when the reading failed, which keeps the newest known.
	Checked(latest string) error
	// Told keeps the newest release told of, so it is told once.
	Told(version string) error
}

// releases is what this run knows of them.
type releases struct {
	latest, told string
	// list is nil until the releases are read, and err why reading them failed.
	list    []model.Release
	loading bool
	err     error
	// drawn are the rows of the box, for the width and the list they were made
	// for.
	drawn      []noteRow
	drawnWidth int
}

// noteRow is a row of the box of release notes: a line of notes, or segs.
type noteRow struct {
	line *markdown.Line
	segs []seg
}

// releaseBox is the box of release notes, shown in place of the key list.
type releaseBox struct {
	top int
}

type releasesMsg struct {
	list []model.Release
	err  error
}

// releaseStart tells of a newer release already known, and reads the releases
// when they are due. Nothing waits for it.
func (m *Model) releaseStart() tea.Cmd {
	r := m.opt.Release
	if r == nil || !r.Tell {
		return nil
	}
	m.rel.latest, m.rel.told = r.Latest, r.Told
	m.tellRelease()
	if r.Due {
		return m.readReleases()
	}
	return nil
}

func (m *Model) readReleases() tea.Cmd {
	r := m.opt.Release
	if r == nil || m.rel.list != nil || m.rel.loading {
		return nil
	}
	m.rel.loading, m.rel.err = true, nil
	ctx := m.ctx
	return func() tea.Msg {
		list, err := r.Source.Releases(ctx)
		return releasesMsg{list, err}
	}
}

// newerRelease is the newest release known past the one running, or "".
func (m *Model) newerRelease() string {
	if r := m.opt.Release; r != nil && r.Tell && release.Newer(m.rel.latest, r.Running) {
		return m.rel.latest
	}
	return ""
}

// tellRelease tells once of a newer release.
func (m *Model) tellRelease() {
	v := m.newerRelease()
	if v == "" || v == m.rel.told {
		return
	}
	m.rel.told = v
	if err := m.opt.Release.Source.Told(v); err != nil {
		m.opt.Log.Warn("the release told of cannot be kept", "err", err.Error())
	}
	m.line.Say(m.now(), fmt.Sprintf("gist %s is out · %s", v, m.opt.Release.How))
}

// gotReleases takes in a reading. One that failed shows only in the log and the
// box of notes: the release is not what the user is doing.
func (m *Model) gotReleases(msg releasesMsg) {
	r := m.opt.Release
	m.rel.loading = false
	latest := ""
	if msg.err != nil {
		m.rel.err = msg.err
		m.opt.Log.Debug("the releases were not read", "err", msg.err.Error())
	} else {
		m.rel.list = msg.list
		if m.rel.list == nil {
			m.rel.list = []model.Release{}
		}
		m.rel.drawn = nil
		for _, rel := range msg.list {
			if latest == "" || release.Newer(rel.Version, latest) {
				latest = rel.Version
			}
		}
	}
	if !r.Tell {
		return
	}
	if latest != "" {
		m.rel.latest = latest
	}
	if err := r.Source.Checked(latest); err != nil {
		m.opt.Log.Warn("when the releases were read cannot be kept", "err", err.Error())
	}
	m.tellRelease()
}

// openReleaseNotes opens the box in place of the key list, reading the releases when
// they are not read yet, or again after a reading that failed.
func (m *Model) openReleaseNotes() tea.Cmd {
	if m.opt.Release == nil {
		return nil
	}
	m.relBox = &releaseBox{}
	return m.readReleases()
}

// releaseBoxKey scrolls the box, goes back to the key list, or closes both.
func (m *Model) releaseBoxKey(k string) {
	on := func(a keymap.Action) bool { return m.keys.Matches(a, k) }
	page := max(m.releaseRows()-1, 1)
	switch {
	case k == keymap.Back || on(keymap.ReleaseNotes):
		m.relBox = nil
	case on(keymap.Help) || on(keymap.Quit):
		m.relBox, m.helpOpen = nil, false
	case on(keymap.Down):
		m.scrollReleases(1)
	case on(keymap.Up):
		m.scrollReleases(-1)
	case on(keymap.PageDown):
		m.scrollReleases(page)
	case on(keymap.PageUp):
		m.scrollReleases(-page)
	case on(keymap.Bottom):
		m.scrollReleases(1 << 20)
	case on(keymap.Top):
		m.scrollReleases(-(1 << 20))
	}
}

func (m *Model) scrollReleases(d int) {
	n := len(m.noteRows(m.releaseWidth() - 4))
	m.relBox.top = max(min(m.relBox.top+d, n-m.releaseRows()), 0)
}

func (m *Model) releaseWidth() int { return min(max(m.width-8, 30), 100) }

// releaseRows is how many rows of notes the box shows.
func (m *Model) releaseRows() int { return max(m.height-6, 3) }

// noteRows are the rows of the box, inner cells wide: every release, newest
// first, the one running marked installed, and a newer one with how to update.
func (m *Model) noteRows(inner int) []noteRow {
	p := m.pal
	say := func(s string, c style) []noteRow { return []noteRow{{segs: []seg{{s, c}}}} }
	switch {
	case m.rel.list == nil && m.rel.err != nil:
		return append(say("the releases cannot be read: "+m.rel.err.Error(), style{fg: p.red}),
			noteRow{}, noteRow{segs: []seg{{"they are at " + release.Page, style{fg: p.subtext0}}}})
	case m.rel.list == nil:
		return say("reading the releases…", style{fg: p.subtext0})
	case len(m.rel.list) == 0:
		return say("no releases yet", style{fg: p.subtext0})
	}
	if m.rel.drawn != nil && m.rel.drawnWidth == inner {
		return m.rel.drawn
	}
	r := m.opt.Release
	var out []noteRow
	if r.FromSource {
		out = append(out, say("this build: "+cmp.Or(r.Version, r.Running)+", built from source", style{fg: p.subtext0})...)
		out = append(out, noteRow{})
	}
	for i, rel := range m.rel.list {
		if i > 0 {
			out = append(out, noteRow{})
		}
		head := []seg{{rel.Version, style{fg: p.blue, bold: true}}}
		if !rel.Published.IsZero() {
			head = append(head, seg{"  " + rel.Published.Format("2006-01-02"), style{fg: p.subtext0}})
		}
		switch c, err := release.Compare(rel.Version, r.Running); {
		case err != nil || r.FromSource:
		case c == 0:
			head = append(head, seg{"  installed", style{fg: p.green}})
		case c > 0:
			head = append(head, seg{"  not installed · " + r.How, style{fg: p.yellow}})
		}
		out = append(out, noteRow{segs: head})
		lines := markdown.Render(release.Notes(rel.Body), inner)
		for j := range lines {
			out = append(out, noteRow{line: &lines[j]})
		}
	}
	m.rel.drawn, m.rel.drawnWidth = out, inner
	return out
}

// drawReleases draws the box of release notes.
func (m *Model) drawReleases(c *canvas) {
	p := m.pal
	w := m.releaseWidth()
	inner := w - 4
	rows := m.noteRows(inner)
	h := min(len(rows), m.releaseRows())
	top := max(min(m.relBox.top, len(rows)-h), 0)
	m.relBox.top = top
	x, y := (c.w-w)/2, max((c.h-h-2)/2, 0)
	c.box(x, y, w, h+2, p.blue, nil)
	c.put(y, x+1, " release notes ", style{fg: p.subtext1, bold: true})
	for i, r := range rows[top : top+h] {
		if r.line != nil {
			m.putMarkdown(c, y+1+i, x+2, *r.line, inner)
		} else if len(r.segs) > 0 {
			c.putCut(y+1+i, x+2, r.segs, inner, style{fg: p.overlay})
		}
	}
	// The frame of this box is bright, so the thumb is brighter still
	m.drawThumb(c, x+w-1, y+1, h, top, len(rows), p.text)
}

// drawAbout writes the version at the foot of the key list, y being its last
// free row, with the key of the release notes and any newer release.
func (m *Model) drawAbout(c *canvas, x, y, w int) {
	r := m.opt.Release
	if r == nil {
		return
	}
	p := m.pal
	about := "gist " + r.Version + " · https://github.com/babarot/gist"
	if v := m.newerRelease(); v != "" {
		about = v + " is out · " + r.How
	}
	left := c.put(y, x, m.keys.Hint(keymap.ReleaseNotes), style{fg: p.blue, bold: true})
	left = c.put(y, left, " release notes", style{fg: p.text})
	st := style{fg: p.subtext0}
	if m.newerRelease() != "" {
		st.fg = p.green
	}
	room := x + w - left - 2
	if n := text.Width(about); n <= room {
		c.put(y, x+w-n, about, st)
		return
	}
	c.putCut(y, left+2, []seg{{about, st}}, room, style{fg: p.overlay})
}
