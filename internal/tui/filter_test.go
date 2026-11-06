package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/config"
	"github.com/babarot/gist/v2/internal/fakegist"
	"github.com/babarot/gist/v2/internal/model"
)

func filterGists() []model.Gist {
	public := gist("1111111", "retry helper", 5*time.Minute, "main.go", "README.md")
	public.Public = true
	secret := gist("2222222", "日本語の設計書", time.Hour, "00-index.md")
	return []model.Gist{public, secret, gist("3333333", "", 2*time.Hour, "deadlink")}
}

func TestFilterNarrowsAsItIsTyped(t *testing.T) {
	m := loaded(t, filterGists(), 120, 34)
	press(t, m, "/")
	if m.uiState() != uiFilter {
		t.Fatalf("state is %s", m.uiState())
	}
	typeText(t, m, "retry")
	s := screen(m)
	if !strings.Contains(s, "retry helper") || strings.Contains(s, "日本語") {
		t.Errorf("got:\n%s", s)
	}
	if !strings.Contains(s, "1 of 3") || !strings.Contains(s, "/ retry") {
		t.Errorf("got:\n%s", s)
	}
}

func TestFilterKeepsAndClears(t *testing.T) {
	m := loaded(t, filterGists(), 120, 34)
	press(t, m, "/")
	typeText(t, m, "retry")
	enter(t, m)
	if m.uiState() != uiList || len(m.tab().rows) != 1 {
		t.Fatalf("state %s, %d rows", m.uiState(), len(m.tab().rows))
	}
	m.line.KeyPressed()
	press(t, m, "j")
	if !strings.Contains(screen(m), "esc clear filter") {
		t.Errorf("got:\n%s", screen(m))
	}
	press(t, m, "esc")
	if len(m.tab().rows) != 3 {
		t.Errorf("%d rows", len(m.tab().rows))
	}
}

func TestEscWhileTypingClears(t *testing.T) {
	m := loaded(t, filterGists(), 120, 34)
	press(t, m, "/")
	typeText(t, m, "retry")
	press(t, m, "esc")
	if m.uiState() != uiList || len(m.tab().rows) != 3 {
		t.Errorf("state %s, %d rows", m.uiState(), len(m.tab().rows))
	}
}

func TestNoMatch(t *testing.T) {
	m := loaded(t, filterGists(), 120, 34)
	press(t, m, "/")
	typeText(t, m, "zzz")
	if s := screen(m); !strings.Contains(s, "no gist matches /zzz") {
		t.Errorf("got:\n%s", s)
	}
	if _, ok := m.selected(); ok {
		t.Error("a gist is under the cursor")
	}
}

func TestFilterTerms(t *testing.T) {
	for q, want := range map[string]int{
		"file:deadlink": 1,
		"desc:retry":    1,
		"vis:public":    1,
		"vis:secret":    2,
		"lang:go":       3,
		"file:zzz":      0,
		"設計書":           1,
	} {
		m := loaded(t, filterGists(), 120, 34)
		press(t, m, "/")
		typeText(t, m, q)
		if got := len(m.tab().rows); got != want {
			t.Errorf("%s: %d rows", q, got)
		}
	}
}

func TestMoveWhileTyping(t *testing.T) {
	m := loaded(t, filterGists(), 120, 34)
	press(t, m, "/", "down", "down")
	if m.tab().cur != 2 {
		t.Errorf("cursor at %d", m.tab().cur)
	}
}

func TestKeysAreTextWhileTyping(t *testing.T) {
	m := loaded(t, filterGists(), 120, 34)
	press(t, m, "/", "q", "D", "j")
	if m.uiState() != uiFilter || m.filtering.box.Value() != "qDj" {
		t.Errorf("state %s, value %q", m.uiState(), m.filtering.box.Value())
	}
}

func TestCursorStaysOnItsGistThroughAFilter(t *testing.T) {
	m := loaded(t, filterGists(), 120, 34)
	press(t, m, "j")
	press(t, m, "/")
	typeText(t, m, "設")
	if cursorID(m) != "2222222" {
		t.Errorf("on %s", cursorID(m))
	}
	press(t, m, "esc")
	if cursorID(m) != "2222222" {
		t.Errorf("on %s", cursorID(m))
	}
}

func TestFilterHoldsThroughARefresh(t *testing.T) {
	gists := filterGists()
	src := &source{pages: onePage(gists)}
	m := newModel(t, options(t, src), 120, 34)
	settle(t, m, m.Init())
	press(t, m, "/")
	typeText(t, m, "retry")
	enter(t, m)
	src.pages = onePage(append(gists, gist("4444444", "another retry", time.Second, "x.go")))
	press(t, m, "R")
	if got := len(m.tab().rows); got != 2 {
		t.Errorf("%d rows", got)
	}
}

func TestFilterIsOfItsTab(t *testing.T) {
	src := &source{pages: onePage(filterGists()), starred: []model.Gist{gist("7777777", "tmux", time.Hour, "a")}}
	m := newModel(t, options(t, src), 120, 34)
	settle(t, m, m.Init())
	press(t, m, "/")
	typeText(t, m, "zzz")
	enter(t, m)
	press(t, m, "2")
	if got := len(m.tab().rows); got != 1 {
		t.Errorf("%d rows on the other tab", got)
	}
}

func TestMatchesAreMarked(t *testing.T) {
	m := loaded(t, filterGists(), 120, 34)
	segs := m.marked("retry helper", []int{0, 1, 6}, style{fg: m.pal.text})
	if len(segs) != 4 || segs[0].text != "re" || segs[0].st.fg != m.pal.yellow || segs[1].text != "try " {
		t.Errorf("got %+v", segs)
	}
}

func TestFilterUsesTheTerminalsCursor(t *testing.T) {
	m := loaded(t, filterGists(), 120, 34)
	press(t, m, "/", "x")
	cur := m.cursor()
	if cur == nil || cur.Y != m.layout().statusY || cur.X != 3 {
		t.Errorf("at %+v", cur)
	}
}

func TestMoreFilesThanShownCanMatch(t *testing.T) {
	m := loaded(t, filterGists(), 120, 34)
	press(t, m, "/")
	typeText(t, m, "readme")
	if len(m.tab().rows) != 1 {
		t.Errorf("%d rows", len(m.tab().rows))
	}
}

func form(t *testing.T) *world {
	t.Helper()
	w := newWorld(t, func(s *fakegist.Server) { s.Add("old", false, map[string]string{"o.txt": "o\n"}) })
	press(t, w.m, "n")
	return w
}

func fill(t *testing.T, w *world, name, desc, content string) {
	t.Helper()
	typeText(t, w.m, name)
	press(t, w.m, "tab")
	typeText(t, w.m, desc)
	w.type_(content)
	ctrl(t, w.m, 'e')
}

func ctrl(t *testing.T, m *Model, r rune) {
	t.Helper()
	_, cmd := m.Update(tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl})
	settle(t, m, cmd)
}

func TestCreate(t *testing.T) {
	w := form(t)
	if w.m.uiState() != uiNew {
		t.Fatalf("state is %s", w.m.uiState())
	}
	fill(t, w, "hello.py", "日本語の説明", "print('hi')\n")
	w.wants("1 line", "print('hi')", "secret")
	ctrl(t, w.m, 's')
	if w.m.uiState() != uiList {
		t.Fatalf("state is %s:\n%s", w.m.uiState(), w.screen())
	}
	g, ok := w.m.selected()
	if !ok || g.Description != "日本語の説明" {
		t.Fatalf("on %+v", g)
	}
	got := w.s.Get(g.ID)
	if got.Public || got.Revisions[0].Files["hello.py"] != "print('hi')\n" {
		t.Errorf("got %+v", got)
	}
	w.wants("✓ created gist "+g.ID[:7]+" (secret) · y copies its URL", "hello.py")
}

func TestCreatePublic(t *testing.T) {
	w := form(t)
	fill(t, w, "a.txt", "", "a\n")
	press(t, w.m, "tab")
	press(t, w.m, " ")
	w.wants("listed on your profile and searchable")
	ctrl(t, w.m, 's')
	g, _ := w.m.selected()
	if !w.s.Get(g.ID).Public {
		t.Error("not public")
	}
}

func TestNewVisibilityFromTheConfig(t *testing.T) {
	o := options(t, &source{})
	o.Config.New.Visibility = config.VisibilityPublic
	m := newModel(t, o, 120, 34)
	press(t, m, "n")
	if !m.form.public {
		t.Error("the config was not read")
	}
}

func TestCreateRefuses(t *testing.T) {
	w := form(t)
	ctrl(t, w.m, 's')
	w.wants("a filename is required")
	typeText(t, w.m, "a.txt")
	ctrl(t, w.m, 's')
	w.wants("GitHub does not take an empty file: ctrl+e to write it")
	if w.m.form.focus != fieldContent {
		t.Errorf("focus on %d", w.m.form.focus)
	}
	for _, r := range w.s.Requests() {
		if strings.HasPrefix(r, "POST") {
			t.Errorf("sent %s", r)
		}
	}
}

func TestCreateFailureKeepsTheForm(t *testing.T) {
	w := form(t)
	fill(t, w, "a.txt", "", "written\n")
	w.s.Fail("POST", 500)
	ctrl(t, w.m, 's')
	if w.m.uiState() != uiNew || w.m.form.content != "written\n" {
		t.Errorf("state %s, content %q", w.m.uiState(), w.m.form.content)
	}
	w.wants("refused by the test")
}

func TestEscClosesTheForm(t *testing.T) {
	w := form(t)
	press(t, w.m, "esc")
	if w.m.uiState() != uiList {
		t.Errorf("state is %s", w.m.uiState())
	}
}

func TestFormFields(t *testing.T) {
	w := form(t)
	press(t, w.m, "tab", "tab")
	if w.m.form.focus != fieldVisibility {
		t.Errorf("focus on %d", w.m.form.focus)
	}
	press(t, w.m, "tab", "tab")
	if w.m.form.focus != fieldName {
		t.Errorf("it did not go round: %d", w.m.form.focus)
	}
	_, cmd := w.m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	settle(t, w.m, cmd)
	if w.m.form.focus != fieldContent {
		t.Errorf("focus on %d", w.m.form.focus)
	}
	enter(t, w.m)
	if w.opened != 1 {
		t.Errorf("opened %d times", w.opened)
	}
}

func TestFormEditorStartsFromWhatWasWritten(t *testing.T) {
	w := form(t)
	fill(t, w, "a.txt", "", "first\n")
	ctrl(t, w.m, 'e')
	if w.gave != "" && w.gave != "first\n" {
		t.Errorf("the editor got %q", w.gave)
	}
}

func TestFormEditorFails(t *testing.T) {
	w := form(t)
	w.editorErr = os.ErrNotExist
	ctrl(t, w.m, 'e')
	if !strings.Contains(w.m.form.err, "the editor failed") {
		t.Errorf("err is %q", w.m.form.err)
	}
}

func TestStarredTabMakesNoGist(t *testing.T) {
	w := newWorld(t, func(s *fakegist.Server) { s.Star("bob", "theirs", map[string]string{"a": "a\n"}) })
	press(t, w.m, "2", "n")
	if w.m.uiState() == uiNew {
		t.Error("the form opened")
	}
	w.wants("switch to 1 mine to create a gist")
}

func TestCreateClearsAFilterThatWouldHideIt(t *testing.T) {
	w := newWorld(t, func(s *fakegist.Server) { s.Add("old", false, map[string]string{"o.txt": "o\n"}) })
	press(t, w.m, "/")
	typeText(t, w.m, "zzz")
	enter(t, w.m)
	press(t, w.m, "n")
	fill(t, w, "a.txt", "made here", "a\n")
	ctrl(t, w.m, 's')
	if g, ok := w.m.selected(); !ok || g.Description != "made here" {
		t.Errorf("on %+v", g)
	}
}

func TestFormOnly(t *testing.T) {
	w := newWorld(t, func(*fakegist.Server) {})
	before := len(w.s.Requests())
	o := w.m.opt
	o.FormOnly = true
	m := New(o)
	m.clockless = true
	t.Cleanup(m.cancel)
	w.m = m
	m.runEditor = func(args []string) tea.Cmd {
		if err := os.WriteFile(args[len(args)-1], []byte("a\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return func() tea.Msg { return editedMsg{} }
	}
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	settle(t, m, cmd)
	if m.uiState() != uiNew {
		t.Fatalf("state is %s", m.uiState())
	}
	typeText(t, m, "a.txt")
	ctrl(t, m, 'e')
	_, cmd = m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	msgs := run(cmd)
	quit := false
	for _, msg := range msgs {
		_, next := m.Update(msg)
		quit = quit || isQuit(next)
	}
	if !quit || m.created == nil {
		t.Errorf("quit %v, created %+v", quit, m.created)
	}
	if got := w.s.Requests()[before:]; len(got) != 1 || !strings.HasPrefix(got[0], "POST") {
		t.Errorf("asked for %v", got)
	}
}

func TestFormOnlyCanceled(t *testing.T) {
	o := options(t, &source{})
	o.FormOnly = true
	m := newModel(t, o, 120, 34)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !isQuit(cmd) || m.created != nil {
		t.Errorf("created %+v", m.created)
	}
}

func TestFormFitsTheTerminal(t *testing.T) {
	for _, size := range [][2]int{{60, 24}, {80, 24}, {30, 10}, {10, 4}} {
		w := form(t)
		_, cmd := w.m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		settle(t, w.m, cmd)
		if got := len(strings.Split(screen(w.m), "\n")); got != size[1] {
			t.Errorf("%v: %d lines", size, got)
		}
	}
}
