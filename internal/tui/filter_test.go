package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/config"
	"github.com/babarot/gist/v2/internal/fakegist"
	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/text"
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
	if !strings.Contains(s, "1 of 3") {
		t.Errorf("got:\n%s", s)
	}
	if line := strings.Split(s, "\n")[filterRow]; !strings.HasPrefix(line, "/ retry") {
		t.Errorf("the filter's line is %q", line)
	}
}

func TestTheFilterLineMakesRoomInTheList(t *testing.T) {
	m := loaded(t, filterGists(), 120, 34)
	l := m.layout()
	press(t, m, "/")
	got := m.layout()
	if got.listY != l.listY+1 || got.listRows != l.listRows-1 || got.boxY != l.boxY || got.boxH != l.boxH {
		t.Errorf("went from %+v to %+v", l, got)
	}
	if len(strings.Split(screen(m), "\n")) != 34 {
		t.Error("the screen is not the height of the terminal")
	}
	press(t, m, "esc")
	if m.layout() != l {
		t.Errorf("did not come back: %+v", m.layout())
	}
}

func TestAnEventDoesNotHideTheFilter(t *testing.T) {
	m := loaded(t, filterGists(), 120, 34)
	press(t, m, "/")
	typeText(t, m, "retry")
	enter(t, m)
	press(t, m, "y")
	s := screen(m)
	if !strings.Contains(s, "✓ copied") || !strings.Contains(s, "/ retry") {
		t.Errorf("got:\n%s", s)
	}
}

func TestAClickFindsItsRowWithTheFilterLine(t *testing.T) {
	m := loaded(t, filterGists(), 120, 34)
	press(t, m, "/")
	typeText(t, m, "e")
	enter(t, m)
	y := m.layout().listY
	click(t, m, 20, y+1)
	if m.tab().cur != 1 {
		t.Errorf("cursor at %d", m.tab().cur)
	}
}

func TestSwitchingToAFilteredTabKeepsTheCursorShown(t *testing.T) {
	src := &source{pages: onePage(manyGists(40)), starred: manyGists(40)}
	m := newModel(t, options(t, src), 120, 34)
	settle(t, m, m.Init())
	press(t, m, "/")
	typeText(t, m, "gist")
	enter(t, m)
	press(t, m, "G", "2", "1")
	t0 := m.tab()
	if t0.cur < t0.top || t0.cur >= t0.top+m.layout().listRows {
		t.Errorf("cursor %d, top %d, rows %d", t0.cur, t0.top, m.layout().listRows)
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

func TestPageWhileTyping(t *testing.T) {
	m := loaded(t, manyGists(40), 120, 34)
	press(t, m, "/")
	rows := m.layout().listRows
	page(t, m, "pgdown")
	if m.tab().cur != rows || m.uiState() != uiFilter {
		t.Errorf("cursor at %d, state %s", m.tab().cur, m.uiState())
	}
}

func TestFilterCursorCountsJapaneseAsTwoCells(t *testing.T) {
	m := loaded(t, filterGists(), 120, 34)
	press(t, m, "/")
	typeText(t, m, "日本語")
	if got := m.cursor().X; got != filterText+6 {
		t.Errorf("at %d", got)
	}
}

func TestFormCursorCountsJapaneseAsTwoCells(t *testing.T) {
	w := form(t)
	typeText(t, w.m, "日本語.md")
	x, _, _, _ := w.m.formBox()
	if got := w.m.cursor().X; got != x+formX+9 {
		t.Errorf("at %d, want %d", got, x+formX+9)
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
	if cur == nil || cur.Y != filterRow || cur.X != 3 {
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

func TestFormStartsFromTheFlags(t *testing.T) {
	o := options(t, &source{})
	o.Config.New.Visibility = config.VisibilityPublic
	o.NewGist = &model.NewGist{Description: "from the flags"}
	m := newModel(t, o, 120, 34)
	press(t, m, "n")
	if m.form.public || m.form.description.Value() != "from the flags" {
		t.Errorf("public %v, description %q", m.form.public, m.form.description.Value())
	}
}

func TestChangesMadeDuringAReadingAreNotUndone(t *testing.T) {
	for name, act := range map[string]func(*world){
		"create": func(w *world) {
			press(t, w.m, "n")
			fill(t, w, "new.txt", "made here", "n\n")
			ctrl(t, w.m, 's')
		},
		"push": func(w *world) {
			w.type_("edited\n")
			enter(t, w.m)
			press(t, w.m, "y")
		},
		"delete": func(w *world) { press(t, w.m, "D", "y") },
	} {
		t.Run(name, func(t *testing.T) {
			w, g := one(t, map[string]string{"a.txt": "a\n"})
			reading := hold(w.m, keyMsg("R"))
			act(w)
			settle(t, w.m, reading)
			wantRows := 1
			switch name {
			case "create":
				wantRows = 2
			case "delete":
				wantRows = 0
			}
			if got := len(w.m.tab().gists); got != wantRows {
				t.Errorf("%d gists", got)
			}
			if name == "push" {
				c, ok := w.m.selected()
				if !ok || !c.Updated.Equal(w.s.Get(g.ID).Updated) {
					t.Errorf("the row is from before the push: %v", c.Updated)
				}
			}
			if w.m.tab().reading {
				t.Error("it never finished reading")
			}
		})
	}
}

func TestQuittingWithTheFormsEditorLeavesNoFile(t *testing.T) {
	w := form(t)
	var dir string
	w.m.runEditor = func(args []string) tea.Cmd {
		dir = filepath.Dir(args[len(args)-1])
		return nil
	}
	ctrl(t, w.m, 'e')
	_, cmd := w.m.Update(keyMsg("ctrl+c"))
	if !isQuit(cmd) {
		t.Fatal("did not quit")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("left %s: %v", dir, err)
	}
}

func TestFormOnlyShowsOnlyTheForm(t *testing.T) {
	o := options(t, &source{})
	o.FormOnly = true
	m := newModel(t, o, 120, 34)
	s := screen(m)
	if !strings.Contains(s, "new gist") || strings.Contains(s, "mine (") {
		t.Errorf("got:\n%s", s)
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

func langGists() []model.Gist {
	a := gist("1111111", "one", time.Minute, "main.go", "util.go")
	a.Files[0].Language, a.Files[1].Language = "Go", "Go"
	a.Public = true
	b := gist("2222222", "two", time.Hour, "my notes.md")
	b.Files[0].Language = "Markdown"
	c := gist("3333333", "three", 2*time.Hour, "init.vim")
	c.Files[0].Language = "Vim Script"
	return []model.Gist{a, b, c}
}

func pressTab(t *testing.T, m *Model) {
	t.Helper()
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	settle(t, m, cmd)
}

func TestTabCompletesAKey(t *testing.T) {
	m := loaded(t, langGists(), 120, 34)
	press(t, m, "/")
	typeText(t, m, "la")
	if !strings.Contains(screen(m), "/ lang:") {
		t.Errorf("not offered:\n%s", screen(m))
	}
	pressTab(t, m)
	if got := m.tab().query; got != "lang:" {
		t.Errorf("got %q", got)
	}
}

func TestTabCompletesValuesAndGoesRound(t *testing.T) {
	m := loaded(t, langGists(), 120, 34)
	press(t, m, "/")
	typeText(t, m, "lang:")
	var got []string
	for range 4 {
		pressTab(t, m)
		got = append(got, m.tab().query)
	}
	// What most files have first; a name with a space by its first word
	want := []string{"lang:Go", "lang:Markdown", "lang:Vim", "lang:Go"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v", got)
	}
	if !strings.Contains(screen(m), "(1/3)") {
		t.Errorf("no count:\n%s", screen(m))
	}
	if len(m.tab().rows) != 1 {
		t.Errorf("%d rows", len(m.tab().rows))
	}
}

func TestAValueNeverLeavesNoRows(t *testing.T) {
	m := loaded(t, langGists(), 120, 34)
	press(t, m, "/")
	typeText(t, m, "file:init lang:")
	pressTab(t, m)
	if got := m.tab().query; got != "file:init lang:Vim" {
		t.Errorf("got %q", got)
	}
	pressTab(t, m) // the only one
	if got := m.tab().query; got != "file:init lang:Vim" {
		t.Errorf("got %q", got)
	}
}

func TestVisAndFileValues(t *testing.T) {
	m := loaded(t, langGists(), 120, 34)
	press(t, m, "/")
	typeText(t, m, "vis:p")
	pressTab(t, m)
	if got := m.tab().query; got != "vis:public" {
		t.Errorf("got %q", got)
	}
	press(t, m, "esc")
	press(t, m, "/")
	typeText(t, m, "file:my")
	pressTab(t, m)
	if got := m.tab().query; got != "file:my" || len(m.tab().rows) != 1 {
		t.Errorf("got %q, %d rows", got, len(m.tab().rows))
	}
}

func TestADescriptionHasNoValues(t *testing.T) {
	m := loaded(t, langGists(), 120, 34)
	press(t, m, "/")
	typeText(t, m, "desc:")
	pressTab(t, m)
	if got := m.tab().query; got != "desc:" {
		t.Errorf("got %q", got)
	}
}

func TestTabInTheMiddleDoesNothing(t *testing.T) {
	m := loaded(t, langGists(), 120, 34)
	press(t, m, "/")
	typeText(t, m, "lang:")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	settle(t, m, cmd)
	pressTab(t, m)
	if got := m.tab().query; got != "lang:" {
		t.Errorf("got %q", got)
	}
}

func TestTheKeysShowOnAnEmptyFilter(t *testing.T) {
	m := loaded(t, langGists(), 120, 34)
	press(t, m, "/")
	if !strings.Contains(screen(m), "/ file: desc: vis: lang:") {
		t.Errorf("got:\n%s", screen(m))
	}
	if m.tab().query != "" {
		t.Error("the ghost is not the filter")
	}
	press(t, m, "esc")
	if strings.Contains(screen(m), "file: desc:") {
		t.Error("left behind")
	}
}

func TestTheGhostFitsANarrowTerminal(t *testing.T) {
	for w := range 40 {
		m := loaded(t, langGists(), w+20, 20)
		press(t, m, "/")
		for line := range strings.SplitSeq(screen(m), "\n") {
			if text.Width(line) > w+20 {
				t.Errorf("width %d: %q", w+20, line)
			}
		}
	}
}

func TestFilteringBySize(t *testing.T) {
	gs := fixture()
	gs[0].Files[0].Size = 300 << 10
	if got := rowsOf(gs, "size:>200KB"); len(got) != 1 || got[0].gist.ID != gs[0].ID {
		t.Errorf("got %d rows", len(got))
	}
	if got := rowsOf(gs, "size:<200KB"); len(got) != len(gs)-1 {
		t.Errorf("got %d rows", len(got))
	}
}

func TestTheGhostSaysHowToWriteASize(t *testing.T) {
	m := loaded(t, langGists(), 120, 34)
	press(t, m, "/")
	for _, step := range []struct{ typed, want string }{
		{"size:", "/ size:>200KB, <1MB, also >= and <="},
		{">", "/ size:>200KB"},
		{"2", "/ size:>2 bytes, or KB, MB"},
	} {
		typeText(t, m, step.typed)
		if !strings.Contains(screen(m), step.want) {
			t.Errorf("no %q in:\n%s", step.want, screen(m))
		}
	}
	pressTab(t, m)
	if got := m.tab().query; got != "size:>2" {
		t.Errorf("tab filled in the example: %q", got)
	}
	press(t, m, "esc")
	press(t, m, "/")
	typeText(t, m, "size:200kb")
	if s := screen(m); !strings.Contains(s, "not a size: try size:>200KB") {
		t.Errorf("got:\n%s", s)
	}
}
