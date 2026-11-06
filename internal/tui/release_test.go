package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/text"
)

type releaseFake struct {
	list    []model.Release
	err     error
	reads   int
	checked []string
	told    []string
}

func (f *releaseFake) Releases(context.Context) ([]model.Release, error) {
	f.reads++
	return f.list, f.err
}

func (f *releaseFake) Checked(latest string) error {
	f.checked = append(f.checked, latest)
	return nil
}

func (f *releaseFake) Told(v string) error {
	f.told = append(f.told, v)
	return nil
}

func twoReleases() *releaseFake {
	return &releaseFake{list: []model.Release{
		{Version: "2.1.0", Published: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), Body: "## What's Changed\n* Add a thing by @alice in https://github.com/babarot/gist/pull/7\n"},
		{Version: "2.0.0", Published: time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC), Body: "* The first one\n"},
	}}
}

func releaseModel(t *testing.T, r *Release, w, h int) *Model {
	t.Helper()
	o := options(t, &source{pages: onePage(manyGists(1))})
	o.Release = r
	m := newModel(t, o, w, h)
	settle(t, m, m.Init())
	return m
}

func TestTheKeyListShowsTheVersion(t *testing.T) {
	m := releaseModel(t, &Release{Version: "2.0.0 (abc1234)", Running: "2.0.0", Source: twoReleases()}, 120, 34)
	press(t, m, "?")
	s := screen(m)
	for _, want := range []string{"gist 2.0.0 (abc1234) · https://github.com/babarot/gist", "w release notes"} {
		if !strings.Contains(s, want) {
			t.Errorf("no %q in:\n%s", want, s)
		}
	}
}

func TestReleaseNotesAreReadWhenOpened(t *testing.T) {
	src := twoReleases()
	m := releaseModel(t, &Release{Version: "2.0.0", Running: "2.0.0", How: "brew upgrade babarot/tap/gist", Source: src}, 120, 34)
	if src.reads != 0 {
		t.Errorf("read %d times before anyone asked", src.reads)
	}
	press(t, m, "?")
	press(t, m, "w")
	if m.uiState() != uiReleaseNotes {
		t.Fatalf("state is %s", m.uiState())
	}
	s := screen(m)
	for _, want := range []string{"release notes", "2.1.0  2026-12-01  not installed · brew upgrade babarot/tap/gist", "Add a thing (#7)", "2.0.0  2026-11-07  installed", "The first one"} {
		if !strings.Contains(s, want) {
			t.Errorf("no %q in:\n%s", want, s)
		}
	}
	if len(src.checked) != 0 {
		t.Error("a build that does not tell kept a reading")
	}
	press(t, m, "esc")
	if m.uiState() != uiHelp {
		t.Errorf("esc left %s", m.uiState())
	}
	press(t, m, "w")
	if src.reads != 1 {
		t.Errorf("read %d times", src.reads)
	}
	press(t, m, "?")
	if m.uiState() != uiList {
		t.Errorf("? left %s", m.uiState())
	}
}

func TestABuildFromSourceMarksNoRelease(t *testing.T) {
	m := releaseModel(t, &Release{Version: "2.0.0 (abc1234)", Running: "2.0.0", FromSource: true, Source: twoReleases()}, 120, 34)
	press(t, m, "?")
	press(t, m, "w")
	s := screen(m)
	if !strings.Contains(s, "this build: 2.0.0 (abc1234), built from source") || strings.Contains(s, "installed") {
		t.Errorf("got:\n%s", s)
	}
}

func TestReleaseNotesThatCannotBeRead(t *testing.T) {
	src := &releaseFake{err: errors.New("no network")}
	m := releaseModel(t, &Release{Version: "2.0.0", Running: "2.0.0", Source: src}, 120, 34)
	press(t, m, "?")
	press(t, m, "w")
	if s := screen(m); !strings.Contains(s, "no network") || !strings.Contains(s, "github.com/babarot/gist/releases") {
		t.Errorf("got:\n%s", s)
	}
	press(t, m, "esc")
	src.err, src.list = nil, twoReleases().list
	press(t, m, "w")
	if s := screen(m); !strings.Contains(s, "The first one") {
		t.Errorf("not read again:\n%s", s)
	}
}

func TestScrollingTheNotes(t *testing.T) {
	src := &releaseFake{}
	for range 30 {
		src.list = append(src.list, model.Release{Version: "1.0.0", Body: "* one\n* two\n* three\n"})
	}
	src.list[29].Version = "0.0.1"
	m := releaseModel(t, &Release{Version: "2.0.0", Running: "2.0.0", Source: src}, 100, 24)
	press(t, m, "?")
	press(t, m, "w")
	if strings.Contains(screen(m), "0.0.1") {
		t.Fatal("the last shows already")
	}
	press(t, m, "G")
	if !strings.Contains(screen(m), "0.0.1") {
		t.Errorf("not at the end:\n%s", screen(m))
	}
	top := m.relBox.top
	press(t, m, "j")
	if m.relBox.top != top {
		t.Errorf("past the end: %d", m.relBox.top)
	}
	press(t, m, "k")
	press(t, m, "g")
	if m.relBox.top != 0 {
		t.Errorf("top is %d", m.relBox.top)
	}
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if m.relBox.top != scrollStep {
		t.Errorf("after the wheel, top is %d", m.relBox.top)
	}
}

func TestANewerReleaseIsToldOnce(t *testing.T) {
	src := twoReleases()
	r := &Release{Version: "2.0.0", Running: "2.0.0", How: "brew upgrade babarot/tap/gist", Tell: true, Due: true, Source: src}
	m := releaseModel(t, r, 120, 34)
	if src.reads != 1 || len(src.told) != 1 || src.told[0] != "2.1.0" {
		t.Fatalf("read %d, told %v", src.reads, src.told)
	}
	if len(src.checked) != 1 || src.checked[0] != "2.1.0" {
		t.Errorf("kept %v", src.checked)
	}
	press(t, m, "?")
	if s := screen(m); !strings.Contains(s, "2.1.0 is out · brew upgrade babarot/tap/gist") {
		t.Errorf("got:\n%s", s)
	}
	again := releaseModel(t, &Release{Version: "2.0.0", Running: "2.0.0", How: "x", Tell: true, Latest: "2.1.0", Told: "2.1.0", Source: src}, 120, 34)
	if src.reads != 1 || len(src.told) != 1 {
		t.Errorf("read %d, told %v", src.reads, src.told)
	}
	if strings.Contains(screen(again), "is out") {
		t.Error("told twice")
	}
}

func TestAKnownReleaseIsToldWithoutReading(t *testing.T) {
	src := twoReleases()
	m := releaseModel(t, &Release{Version: "2.0.0", Running: "2.0.0", How: "x", Tell: true, Latest: "2.1.0", Source: src}, 120, 34)
	if src.reads != 0 || len(src.told) != 1 {
		t.Errorf("read %d, told %v", src.reads, src.told)
	}
	_ = m
}

func TestAFailedReadingKeepsWhatWasKnown(t *testing.T) {
	src := &releaseFake{err: errors.New("no")}
	m := releaseModel(t, &Release{Version: "2.0.0", Running: "2.0.0", How: "x", Tell: true, Due: true, Latest: "2.1.0", Told: "2.1.0", Source: src}, 120, 34)
	if len(src.checked) != 1 || src.checked[0] != "" {
		t.Errorf("kept %v", src.checked)
	}
	if m.rel.latest != "2.1.0" {
		t.Errorf("latest is %q", m.rel.latest)
	}
}

func TestNoReleasesLeavesTheKeyListAsItWas(t *testing.T) {
	m := releaseModel(t, nil, 120, 34)
	press(t, m, "?")
	press(t, m, "w")
	if m.uiState() != uiHelp || strings.Contains(screen(m), "release notes") {
		t.Errorf("state %s:\n%s", m.uiState(), screen(m))
	}
}

func TestTheBoxFitsEveryTerminal(t *testing.T) {
	long := &Release{Version: strings.Repeat("9", 90), Running: "2.0.0", How: strings.Repeat("h", 90), Tell: true, Latest: "9.0.0", Source: twoReleases()}
	for w := 30; w < 130; w += 7 {
		for h := 8; h < 40; h += 5 {
			m := releaseModel(t, long, w, h)
			press(t, m, "?")
			check := func() {
				for line := range strings.SplitSeq(screen(m), "\n") {
					if text.Width(line) > w {
						t.Fatalf("%dx%d: %q", w, h, line)
					}
				}
			}
			check()
			press(t, m, "w")
			check()
			press(t, m, "esc")
			press(t, m, "esc")
		}
	}
}

func TestTheThumbStandsOutFromTheBoxsFrame(t *testing.T) {
	src := &releaseFake{}
	for range 30 {
		src.list = append(src.list, model.Release{Version: "1.0.0", Body: "* one\n* two\n"})
	}
	m := releaseModel(t, &Release{Version: "2.0.0", Running: "2.0.0", Source: src}, 100, 24)
	press(t, m, "?")
	press(t, m, "w")
	c := newCanvas(100, 24)
	m.drawReleases(c)
	found := false
	for _, row := range c.rows {
		for _, cl := range row {
			if cl.text == "┃" {
				found = true
				if cl.style.fg == m.pal.blue || cl.style.fg == m.pal.overlay {
					t.Fatalf("the thumb is in %v", cl.style.fg)
				}
			}
		}
	}
	if !found {
		t.Error("no thumb")
	}
}
