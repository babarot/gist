package tui

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/babarot/gist/v2/internal/cache"
	"github.com/babarot/gist/v2/internal/config"
	"github.com/babarot/gist/v2/internal/fakegist"
	"github.com/babarot/gist/v2/internal/logging"
	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/tui/keymap"
	"github.com/babarot/gist/v2/internal/tui/statusline"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata")

// now is the fixed time of the tests.
var now = time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

func gist(id, description string, age time.Duration, files ...string) model.Gist {
	g := model.Gist{ID: id + strings.Repeat("0", 32-len(id)), Description: description, Owner: "alice",
		URL: "https://gist.github.com/alice/" + id, Created: now.Add(-age * 2), Updated: now.Add(-age)}
	for _, f := range files {
		g.Files = append(g.Files, model.File{Name: f, Size: 1234, Language: "Go"})
	}
	return g
}

func manyGists(n int) []model.Gist {
	var out []model.Gist
	for i := range n {
		out = append(out, gist(fmt.Sprintf("%07x", i+1), fmt.Sprintf("gist number %d", i), time.Duration(i+1)*time.Hour, "a.go"))
	}
	return out
}

// fixture is the list of the screen tests, with what breaks widths.
func fixture() []model.Gist {
	public := gist("1111111", "a public gist with a comment", 5*time.Minute, "main.go", "README.md", "go.mod")
	public.Public, public.Comments = true, 3
	return []model.Gist{
		public,
		gist("2222222", "日本語の説明がある Gist と、とても長い説明の末尾がどこで切れるかを確かめるための文字列を足していく", 3*time.Hour, "00-index.md", "10-design.md"),
		gist("3333333", "", 2*24*time.Hour, "no-description.txt", "second.txt"),
		gist("4444444", "emoji \u2764\ufe0f and a family \U0001F468\u200d\U0001F469\u200d\U0001F466 and e\u0301", 120*24*time.Hour, "emoji.md"),
		gist("5555555", "escape \x1b[31msequence\x1b[0m\tand a tab", 3650*24*time.Hour, "x\x1b]0;title\x07.txt"),
	}
}

// source is a made-up GitHub, whose pages come one at a time when held.
type source struct {
	mu      sync.Mutex
	login   string
	pages   []model.ListPage
	starred []model.Gist
	err     error
	calls   [][]string
	// viewerErr fails the question of who the token belongs to.
	viewerErr error
	// files are the files Get gives, by name, and gets the gists it was asked for.
	files  map[string]model.File
	gets   []string
	getErr error
	// comments are by gist, and commentReads the gists they were asked for.
	comments     map[string][]model.Comment
	commentReads []string
	commentsErr  error
	// newer is how much newer than the list a gist comes back.
	newer time.Duration
}

func (s *source) Viewer(context.Context) (string, error) {
	if s.viewerErr != nil {
		return "", s.viewerErr
	}
	if s.login == "" {
		return "alice", nil
	}
	return s.login, nil
}

func (s *source) List(_ context.Context, etags []string, got func(model.ListPage)) ([]model.ListPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, etags)
	if s.err != nil {
		return nil, s.err
	}
	for _, p := range s.pages {
		got(p)
	}
	return s.pages, nil
}

func (s *source) Starred(context.Context) ([]model.Gist, error) { return s.starred, s.err }

// Get is the gist with content for each file, from s.files by name.
func (s *source) Get(_ context.Context, id string) (model.Gist, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gets = append(s.gets, id)
	if s.getErr != nil {
		return model.Gist{}, s.getErr
	}
	defer func() {
		if s.newer > 0 && len(s.gets) > 5 {
			panic("read for ever")
		}
	}()
	for _, g := range slices.Concat(s.listed(), s.starred) {
		if g.ID != id {
			continue
		}
		g.History = []string{"r2", "r1"}
		g.Updated = g.Updated.Add(s.newer)
		g.Files = slices.Clone(g.Files)
		for i := range g.Files {
			f := &g.Files[i]
			if c, ok := s.files[f.Name]; ok {
				*f = c
				continue
			}
			f.Content = "content of " + f.Name + "\n"
		}
		return g, nil
	}
	return model.Gist{}, errors.New("no such gist")
}

func (s *source) listed() []model.Gist {
	var out []model.Gist
	for _, p := range s.pages {
		out = append(out, p.Gists...)
	}
	return out
}

func (s *source) Delete(context.Context, string) error { return nil }

func (s *source) Create(context.Context, model.NewGist) (model.Gist, error) {
	return model.Gist{}, errors.New("not in these tests")
}

func (s *source) Comments(_ context.Context, id string) ([]model.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commentReads = append(s.commentReads, id)
	return s.comments[id], s.commentsErr
}

type layoutStore struct{ h int }

func (l *layoutStore) PreviewHeight() int { return l.h }
func (l *layoutStore) SavePreviewHeight(h int) error {
	l.h = h
	return nil
}

func options(t *testing.T, src Source) Options {
	t.Helper()
	cfg := config.Default()
	cfg.Theme = config.ThemeDefault
	// Most of the screens here are of the description, which is long
	cfg.List.Lead = config.LeadDescription
	return Options{
		Source: src, Via: "gh", Host: "github.com", CacheDir: t.TempDir(), Config: cfg, Keys: keymap.Default(),
		Log: logging.Discard(), Now: func() time.Time { return now },
		Browse: func(string) error { return nil },
	}
}

func onePage(gists []model.Gist) []model.ListPage {
	return []model.ListPage{{Number: 1, Last: 1, ETag: "e1", Gists: gists}}
}

func newModel(t *testing.T, o Options, w, h int) *Model {
	t.Helper()
	m := New(o)
	m.clockless = true
	t.Cleanup(m.cancel)
	_, cmd := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	settle(t, m, cmd)
	return m
}

// settle runs a command and what comes of it, as the program would.
func settle(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	for _, msg := range run(cmd) {
		if _, quit := msg.(tea.QuitMsg); quit {
			continue
		}
		_, next := m.Update(msg)
		settle(t, m, next)
	}
}

func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	switch msg := cmd().(type) {
	case nil:
		return nil
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range msg {
			out = append(out, run(c)...)
		}
		return out
	default:
		return []tea.Msg{msg}
	}
}

// loaded is a model that has read gists.
func loaded(t *testing.T, gists []model.Gist, w, h int) *Model {
	t.Helper()
	m := newModel(t, options(t, &source{pages: onePage(gists)}), w, h)
	settle(t, m, m.Init())
	return m
}

func press(t *testing.T, m *Model, keys ...string) {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		case "ctrl+c":
			msg = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		default:
			msg = tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
		}
		_, cmd := m.Update(msg)
		settle(t, m, cmd)
	}
}

func isQuit(cmd tea.Cmd) bool {
	for _, msg := range run(cmd) {
		if _, ok := msg.(tea.QuitMsg); ok {
			return true
		}
	}
	return false
}

func screen(m *Model) string { return ansi.Strip(m.render()) }

func cursorID(m *Model) string {
	g, _ := m.selected()
	return g.ID[:7]
}

func TestFirstReadShowsTheList(t *testing.T) {
	m := loaded(t, fixture(), 120, 34)
	s := screen(m)
	for _, want := range []string{"gist // @alice", "1 mine (5)", "2 starred (…)", "via gh", "DESCRIPTION", "COMMENTS", "FILES", "VIS", "UPDATED",
		"a public gist with a comment", "public", "secret", "5m", "3h", "2d", "4mo", "10y", "no-description.txt +1", "✓ 5 gists · saved to the cache"} {
		if !strings.Contains(s, want) {
			t.Errorf("no %q in:\n%s", want, s)
		}
	}
}

func TestListIsNewestFirst(t *testing.T) {
	gists := fixture()
	slices.Reverse(gists)
	m := loaded(t, gists, 120, 34)
	if cursorID(m) != "1111111" {
		t.Errorf("the cursor is on %s", cursorID(m))
	}
}

func TestPagesComeInAsTheyAreRead(t *testing.T) {
	m := newModel(t, options(t, &source{}), 120, 34)
	m.login = "alice"
	cmd := m.readMine()
	gists := manyGists(45)
	t0 := &m.tabs[tabMine]
	m.gotPage(pageMsg{seq: t0.seq, page: model.ListPage{Number: 1, Last: 8, Gists: gists[:30]}, next: closed()})
	s := screen(m)
	if !strings.Contains(s, "1 mine (30+)") || !strings.Contains(s, "loading 30 of ~240") {
		t.Errorf("after a page:\n%s", s)
	}
	m.tabs[tabMine].reading = false
	_ = cmd
}

func closed() <-chan tea.Msg {
	c := make(chan tea.Msg)
	close(c)
	return c
}

func TestCursorStaysWhenAGistComesInAbove(t *testing.T) {
	m := loaded(t, manyGists(40), 120, 34)
	press(t, m, "j", "j", "j", "j", "j")
	id := cursorID(m)
	height := m.tab().cur - m.tab().top
	t2 := m.tab()
	newer := gist("9999999", "made just now", time.Minute, "new.go")
	m.replace(t2, append([]model.Gist{newer}, t2.gists...))
	if cursorID(m) != id {
		t.Errorf("the cursor went to %s", cursorID(m))
	}
	if got := t2.cur - t2.top; got != height {
		t.Errorf("its height went from %d to %d", height, got)
	}
}

func TestCursorSettlesWhenItsGistGoes(t *testing.T) {
	gists := manyGists(3)
	m := loaded(t, gists, 120, 34)
	press(t, m, "G")
	m.replace(m.tab(), gists[:1])
	if m.tab().cur != 0 {
		t.Errorf("cursor at %d", m.tab().cur)
	}
}

func TestKeptListShowsAtOnce(t *testing.T) {
	o := options(t, &source{})
	gists := manyGists(3)
	if err := cache.Open(filepath.Join(o.CacheDir, cache.MineFile), o.Host, "alice").Save(cache.List{Gists: gists}); err != nil {
		t.Fatal(err)
	}
	m := newModel(t, o, 120, 34)
	if s := screen(m); !strings.Contains(s, "gist number 0") || !strings.Contains(s, "@alice") {
		t.Errorf("before anything is read:\n%s", s)
	}
}

func TestKeptListOfAnotherAccountGoes(t *testing.T) {
	o := options(t, &source{login: "bob", pages: onePage(manyGists(1))})
	if err := cache.Open(filepath.Join(o.CacheDir, cache.MineFile), o.Host, "alice").Save(cache.List{Gists: fixture()}); err != nil {
		t.Fatal(err)
	}
	m := newModel(t, o, 120, 34)
	settle(t, m, m.Init())
	if s := screen(m); strings.Contains(s, "public gist") || !strings.Contains(s, "@bob") {
		t.Errorf("got:\n%s", s)
	}
}

func TestRefreshAsksWithTheETags(t *testing.T) {
	src := &source{pages: onePage(manyGists(2))}
	m := newModel(t, options(t, src), 120, 34)
	settle(t, m, m.Init())
	src.pages = []model.ListPage{{Number: 1, ETag: "e1", NotModified: true}}
	press(t, m, "R")
	if !slices.Equal(src.calls[1], []string{"e1"}) {
		t.Errorf("calls are %v", src.calls)
	}
	if s := screen(m); !strings.Contains(s, "up to date") || !strings.Contains(s, "gist number 1") {
		t.Errorf("got:\n%s", s)
	}
}

func TestRefreshMarksWhatChanged(t *testing.T) {
	gists := manyGists(3)
	src := &source{pages: onePage(gists)}
	m := newModel(t, options(t, src), 120, 34)
	settle(t, m, m.Init())

	changed := gists[1]
	changed.Updated = now
	added := gist("9999999", "new", time.Second, "new.go")
	src.pages = onePage([]model.Gist{added, changed})
	press(t, m, "R")
	s := screen(m)
	if !strings.Contains(s, "✓ 1 new · 1 updated · 2 gone") {
		t.Errorf("got:\n%s", s)
	}
	if !m.tab().marks[added.ID] || !m.tab().marks[changed.ID] {
		t.Errorf("marks are %v", m.tab().marks)
	}
	if strings.Count(s, "●") != 2 {
		t.Errorf("got:\n%s", s)
	}
	press(t, m, "j", "k")
	if m.tab().marks[changed.ID] {
		t.Error("the mark stayed under the cursor")
	}
}

func TestFailedReadSaysSoAndKeepsTheList(t *testing.T) {
	src := &source{pages: onePage(manyGists(2))}
	m := newModel(t, options(t, src), 120, 34)
	settle(t, m, m.Init())
	src.err = errors.New("the network went away")
	press(t, m, "R")
	s := screen(m)
	if !strings.Contains(s, "the network went away") || !strings.Contains(s, "gist number 0") {
		t.Errorf("got:\n%s", s)
	}
}

func TestAFailedStartCanBeTriedAgain(t *testing.T) {
	src := &source{viewerErr: errors.New("the network went away"), pages: onePage(manyGists(1))}
	m := newModel(t, options(t, src), 120, 34)
	settle(t, m, m.Init())
	if s := screen(m); !strings.Contains(s, "the network went away") {
		t.Errorf("got:\n%s", s)
	}
	if m.busy() {
		t.Error("it looks busy with nothing going on")
	}
	src.viewerErr = nil
	press(t, m, "R")
	if s := screen(m); !strings.Contains(s, "gist number 0") {
		t.Errorf("got:\n%s", s)
	}
}

func TestStarredIsReadWhenFirstShown(t *testing.T) {
	src := &source{pages: onePage(manyGists(1))}
	src.starred = []model.Gist{gist("7777777", "tmux with vim keys", time.Hour, "tmux.conf")}
	src.starred[0].Owner = "kanata-dev"
	m := newModel(t, options(t, src), 120, 34)
	settle(t, m, m.Init())
	if !strings.Contains(screen(m), "2 starred (…)") {
		t.Error("read before it was shown")
	}
	press(t, m, "2")
	s := screen(m)
	if !strings.Contains(s, "2 starred (1)") || !strings.Contains(s, "kanata-dev  tmux with vim keys") ||
		!regexp.MustCompile(`OWNER +DESCRIPTION`).MatchString(s) {
		t.Errorf("got:\n%s", s)
	}
	press(t, m, "1")
	if !strings.Contains(screen(m), "gist number 0") {
		t.Error("the tab did not change back")
	}
}

func TestEmptyList(t *testing.T) {
	m := loaded(t, nil, 120, 34)
	if s := screen(m); !strings.Contains(s, "no gists yet") {
		t.Errorf("got:\n%s", s)
	}
}

func TestMovingStopsAtTheEnds(t *testing.T) {
	m := loaded(t, manyGists(3), 120, 34)
	press(t, m, "k")
	if m.tab().cur != 0 {
		t.Errorf("at %d", m.tab().cur)
	}
	press(t, m, "j", "j", "j", "j")
	if m.tab().cur != 2 {
		t.Errorf("at %d", m.tab().cur)
	}
	press(t, m, "g")
	if m.tab().cur != 0 {
		t.Errorf("at %d", m.tab().cur)
	}
	press(t, m, "G")
	if m.tab().cur != 2 {
		t.Errorf("at %d", m.tab().cur)
	}
}

func TestListScrolls(t *testing.T) {
	m := loaded(t, manyGists(40), 120, 34)
	rows := m.layout().listRows
	for range rows + 2 {
		press(t, m, "j")
	}
	if m.tab().top != 3 {
		t.Errorf("top is %d", m.tab().top)
	}
	if !strings.Contains(screen(m), fmt.Sprintf("gist number %d", rows+2)) {
		t.Errorf("the cursor's row is not shown:\n%s", screen(m))
	}
}

func TestPageKeys(t *testing.T) {
	for _, tt := range []struct{ down, up string }{{"pgdown", "pgup"}, {"ctrl+d", "ctrl+u"}} {
		m := loaded(t, manyGists(40), 120, 34)
		rows := m.layout().listRows
		page(t, m, tt.down)
		if m.tab().cur != rows {
			t.Errorf("%s: cursor at %d, want %d", tt.down, m.tab().cur, rows)
		}
		page(t, m, tt.down)
		page(t, m, tt.up)
		if m.tab().cur != rows {
			t.Errorf("%s: cursor at %d, want %d", tt.up, m.tab().cur, rows)
		}
		if got := m.tab().cur; got < m.tab().top || got >= m.tab().top+rows {
			t.Errorf("the cursor's row is not shown: %d from %d", got, m.tab().top)
		}
	}
}

func TestPageKeysStopAtTheEnds(t *testing.T) {
	m := loaded(t, manyGists(3), 120, 34)
	page(t, m, "pgdown")
	if m.tab().cur != 2 {
		t.Errorf("cursor at %d", m.tab().cur)
	}
	page(t, m, "ctrl+u")
	if m.tab().cur != 0 {
		t.Errorf("cursor at %d", m.tab().cur)
	}
}

// page presses a key of those press does not write.
func page(t *testing.T, m *Model, k string) {
	t.Helper()
	msg := map[string]tea.KeyPressMsg{
		"pgdown": {Code: tea.KeyPgDown},
		"pgup":   {Code: tea.KeyPgUp},
		"ctrl+d": {Code: 'd', Mod: tea.ModCtrl},
		"ctrl+u": {Code: 'u', Mod: tea.ModCtrl},
	}[k]
	if got := msg.String(); got != k {
		t.Fatalf("the key reads %q, not %q", got, k)
	}
	_, cmd := m.Update(msg)
	settle(t, m, cmd)
}

func TestFileTabsStopAtTheEnds(t *testing.T) {
	m := loaded(t, fixture(), 120, 34)
	press(t, m, "h")
	g, _ := m.selected()
	if m.selectedFile(g) != 0 {
		t.Errorf("at %d", m.selectedFile(g))
	}
	press(t, m, "l", "l", "l", "l")
	if m.selectedFile(g) != 2 {
		t.Errorf("at %d", m.selectedFile(g))
	}
	press(t, m, "j", "k")
	if m.selectedFile(g) != 2 {
		t.Error("the file was not kept for the gist")
	}
}

func TestOpenGist(t *testing.T) {
	o := options(t, &source{pages: onePage(manyGists(40))})
	o.Select = manyGists(40)[35].ID
	m := newModel(t, o, 120, 34)
	settle(t, m, m.Init())
	if g, _ := m.selected(); g.ID != o.Select {
		t.Errorf("the cursor is on %s", g.ID)
	}
	if !strings.Contains(screen(m), "gist number 35") {
		t.Errorf("its row is not shown:\n%s", screen(m))
	}
}

func TestBrowseAndCopy(t *testing.T) {
	o := options(t, &source{pages: onePage(manyGists(1))})
	var opened []string
	o.Browse = func(url string) error {
		opened = append(opened, url)
		return errors.New("no browser")
	}
	m := newModel(t, o, 120, 34)
	settle(t, m, m.Init())
	press(t, m, "o")
	if len(opened) != 1 || !strings.Contains(screen(m), "no browser") {
		t.Errorf("opened %v:\n%s", opened, screen(m))
	}
	copied := loaded(t, manyGists(1), 120, 34)
	press(t, copied, "y")
	if !strings.Contains(screen(copied), "✓ copied https://gist.github.com") {
		t.Errorf("got:\n%s", screen(copied))
	}
}

func TestThemeFollowsTheTerminal(t *testing.T) {
	o := options(t, &source{})
	o.Config.Theme = config.ThemeAuto
	m := newModel(t, o, 120, 34)
	if !m.pal.dark {
		t.Error("not dark until told")
	}
	m.Update(tea.BackgroundColorMsg{Color: themes[config.ThemeLatte].base})
	if m.pal.dark {
		t.Error("stayed dark on a light terminal")
	}
}

func TestThemeSetInTheConfigStays(t *testing.T) {
	m := newModel(t, options(t, &source{}), 120, 34)
	m.Update(tea.BackgroundColorMsg{Color: themes[config.ThemeLatte].base})
	if !m.pal.dark {
		t.Error("the terminal's background won over the config")
	}
}

func TestQuit(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		m := loaded(t, manyGists(1), 120, 34)
		_, cmd := m.Update(keyMsg(k))
		if !isQuit(cmd) {
			t.Errorf("%s did not quit", k)
		}
	}
}

func keyMsg(k string) tea.KeyPressMsg {
	if k == "ctrl+c" {
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
}

func TestKeysInTheConfig(t *testing.T) {
	o := options(t, &source{pages: onePage(manyGists(3))})
	var err error
	if o.Keys, err = keymap.Default().With(map[string][]string{"down": {"x"}}); err != nil {
		t.Fatal(err)
	}
	m := newModel(t, o, 120, 34)
	settle(t, m, m.Init())
	press(t, m, "j")
	if m.tab().cur != 0 {
		t.Error("the old key still moves")
	}
	press(t, m, "x")
	if m.tab().cur != 1 {
		t.Error("the new key does not move")
	}
}

func TestAuthFail(t *testing.T) {
	o := options(t, nil)
	o.NoToken = true
	m := newModel(t, o, 120, 34)
	if cmd := m.Init(); cmd != nil {
		settle(t, m, cmd)
	}
	s := screen(m)
	for _, want := range []string{"could not find a GitHub token", "GH_TOKEN", "GITHUB_TOKEN", "hosts.yml", "gh auth token", "gh auth login", "Gists: read and write"} {
		if !strings.Contains(s, want) {
			t.Errorf("no %q in:\n%s", want, s)
		}
	}
	press(t, m, "?", "j")
	if m.uiState() != uiAuthFail {
		t.Errorf("state is %s", m.uiState())
	}
	if _, cmd := m.Update(keyMsg("q")); !isQuit(cmd) {
		t.Error("q did not quit")
	}
}

// states are a model in each state of the screen, on a list that has what
// breaks widths.
func states(t *testing.T, w, h int) map[uiState]*Model {
	t.Helper()
	list := func() *Model { return loaded(t, fixture(), w, h) }
	out := map[uiState]*Model{uiList: list()}
	comments := list()
	press(t, comments, "c")
	out[uiListComments] = comments
	help := list()
	press(t, help, "?")
	out[uiHelp] = help
	o := options(t, nil)
	o.NoToken = true
	out[uiAuthFail] = newModel(t, o, w, h)

	// What changes a gist needs a GitHub that takes it
	world := func() *world {
		t.Helper()
		wd := newWorld(t, func(s *fakegist.Server) {
			for _, g := range fixture() {
				names := map[string]string{}
				for _, f := range g.Files {
					names[f.Name] = "content\n"
				}
				s.Add(g.Description, g.Public, names)
			}
		})
		_, cmd := wd.m.Update(tea.WindowSizeMsg{Width: w, Height: h})
		settle(t, wd.m, cmd)
		return wd
	}
	push := world()
	push.type_("edited\n")
	enter(t, push.m)
	out[uiConfirmPush] = push.m

	conflict := world()
	conflict.type_("edited\n")
	enter(t, conflict.m)
	g, _ := conflict.m.selected()
	conflict.s.Edit(g.ID, g.Files[0].Name, "theirs\n")
	press(t, conflict.m, "y")
	out[uiConfirmConflict] = conflict.m

	del := world()
	press(t, del.m, "D")
	out[uiConfirmDelete] = del.m

	discard := world()
	discard.type_("edited\n")
	enter(t, discard.m)
	press(t, discard.m, "esc", "X")
	out[uiConfirmDiscard] = discard.m

	input := world()
	press(t, input.m, "E")
	out[uiInput] = input.m

	filtering := list()
	press(t, filtering, "/")
	out[uiFilter] = filtering

	form := list()
	press(t, form, "n")
	out[uiNew] = form

	inputConflict := world()
	press(t, inputConflict.m, "E")
	typeText(t, inputConflict.m, "x")
	g, _ = inputConflict.m.selected()
	inputConflict.s.SetDescription(g.ID, "theirs")
	enter(t, inputConflict.m)
	out[uiInputConflict] = inputConflict.m
	return out
}

// TestEveryStateHasAModelHere needs the states named here.
func TestEveryStateHasAModelHere(t *testing.T) {
	got := states(t, 120, 34)
	for s := range numUIStates {
		m, ok := got[s]
		if !ok || m.uiState() != s {
			t.Errorf("no case is in state %q: add one to states", s)
		}
	}
}

func TestKeyHintsSayWhereTheyAre(t *testing.T) {
	cases := states(t, 120, 34)
	list := fmt.Sprint(cases[uiList].keyHints())
	for s, m := range cases {
		got := fmt.Sprint(m.keyHints())
		switch {
		case len(m.keyHints()) == 0:
			t.Errorf("%s: no key hints", s)
		case s != uiList && got == list:
			t.Errorf("%s: the key hints are the list's", s)
		}
	}
}

func TestKeyLineKeepsWhereToFindTheRest(t *testing.T) {
	long := []hint{{"a", "one"}, {"b", "two"}, {"c", "three"}, {"?", "keys"}}
	got := fitHints(long, 16)
	if len(got) != 2 || got[0].key != "a" || got[1].key != "?" {
		t.Errorf("got %v", got)
	}
	if got := fitHints(long, 200); len(got) != 4 {
		t.Errorf("got %v", got)
	}
	if got := fitHints(long, 1); len(got) != 1 || got[0].key != "?" {
		t.Errorf("got %v", got)
	}
}

func TestHelpOpensWhereItShould(t *testing.T) {
	for s := range numUIStates {
		m := states(t, 120, 34)[s]
		if s == uiHelp {
			continue
		}
		press(t, m, "?")
		if got := m.uiState() == uiHelp; got != s.opensHelp() {
			t.Errorf("%s: ? opens the key list: %v, want %v", s, got, s.opensHelp())
		}
	}
}

func TestHelpNamesEveryAction(t *testing.T) {
	named := map[keymap.Action]bool{}
	for _, g := range helpGroups {
		for _, r := range g.rows {
			for _, a := range r.actions {
				named[a] = true
			}
		}
	}
	for _, d := range keymap.Defaults {
		// What is open says its own keys
		if !named[d.Action] && keymap.ScopeOf(d.Action) == keymap.ScopeList {
			t.Errorf("the key list does not name %s", d.Action)
		}
	}
}

func TestHelpKeepsKeysFromTheList(t *testing.T) {
	m := loaded(t, manyGists(3), 120, 34)
	press(t, m, "?", "j", "2")
	if m.tab().cur != 0 || m.cur != tabMine || m.uiState() != uiHelp {
		t.Errorf("a key reached the list: cursor %d, tab %d, %s", m.tab().cur, m.cur, m.uiState())
	}
	press(t, m, "esc")
	if m.uiState() != uiList {
		t.Errorf("state is %s", m.uiState())
	}
	press(t, m, "?", "?")
	if m.uiState() != uiList {
		t.Errorf("state is %s", m.uiState())
	}
}

func TestEveryLineFitsTheTerminal(t *testing.T) {
	for _, w := range []int{60, 80, 120, 200} {
		for _, h := range []int{24, 40, 8} {
			for s, m := range states(t, w, h) {
				lines := strings.Split(m.render(), "\n")
				if len(lines) != h {
					t.Errorf("%s %dx%d: %d lines", s, w, h, len(lines))
				}
				for i, l := range lines {
					if got := ansi.StringWidth(l); got > w {
						t.Errorf("%s %dx%d: line %d is %d wide: %q", s, w, h, i, got, ansi.Strip(l))
					}
				}
			}
		}
	}
}

func TestTheKeyListOverWideCharacters(t *testing.T) {
	var gists []model.Gist
	for i := range 20 {
		gists = append(gists, gist(fmt.Sprintf("%07x", i+1), strings.Repeat("日本語", 40), time.Hour, "日本語のファイル名.md"))
	}
	for w := 60; w <= 130; w++ {
		for _, shift := range []string{"", "a"} {
			for i := range gists {
				gists[i].Description = shift + strings.Repeat("日本語", 40)
			}
			m := loaded(t, gists, w, 34)
			press(t, m, "?")
			// A cell too many on a row moves the frame's right edge out of line
			boxW := min(96, w-8)
			edge := (w-boxW)/2 + boxW - 1
			for i, l := range strings.Split(screen(m), "\n") {
				at := strings.LastIndexAny(l, "│╮╯")
				if i < 4 || at < 0 || i > 11 {
					continue
				}
				if got := ansi.StringWidth(l[:at]); got != edge {
					t.Errorf("%d%s: row %d has the frame's edge at %d, want %d", w, shift, i, got, edge)
				}
			}
		}
	}
}

func TestSmallTerminalsDoNotBreak(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {10, 3}, {20, 5}, {30, 2}, {0, 0}} {
		m := loaded(t, fixture(), size[0], size[1])
		press(t, m, "j", "?")
		_ = m.render()
	}
}

func TestOwnerIsAColumnOfItsOwn(t *testing.T) {
	src := &source{pages: onePage(manyGists(1))}
	a := gist("7777777", "first", time.Hour, "tmux.conf")
	a.Owner = "kanata-dev"
	b := gist("8888888", "second", 2*time.Hour, "b.txt")
	b.Owner = "bob"
	long := gist("9999999", "third", 3*time.Hour, "c.txt")
	long.Owner = strings.Repeat("x", 39)
	src.starred = []model.Gist{a, b, long}
	o := options(t, src)
	o.Config.List.Lead = config.Default().List.Lead
	m := newModel(t, o, 120, 34)
	settle(t, m, m.Init())
	press(t, m, "2")
	s := screen(m)
	if !regexp.MustCompile(`OWNER +FILE +DESCRIPTION`).MatchString(s) {
		t.Errorf("got:\n%s", s)
	}
	lines := strings.Split(s, "\n")
	col := strings.Index(lines[1], "FILE")
	for _, l := range []string{lines[2], lines[3], lines[4]} {
		if !strings.Contains(l[col:], ".") {
			t.Errorf("the file is not under FILE: %q", l)
		}
	}
	if !strings.Contains(s, "kanata-dev") || !strings.Contains(s, "bob") || !strings.Contains(s, "…") {
		t.Errorf("got:\n%s", s)
	}
	press(t, m, "1")
	if strings.Contains(screen(m), "OWNER") {
		t.Errorf("mine has an owner column:\n%s", screen(m))
	}
}

func TestOwnerColumnGoesOnANarrowTerminal(t *testing.T) {
	src := &source{pages: onePage(manyGists(1))}
	a := gist("7777777", "first", time.Hour, "tmux.conf")
	a.Owner = strings.Repeat("x", 39)
	src.starred = []model.Gist{a}
	m := newModel(t, options(t, src), 35, 24)
	settle(t, m, m.Init())
	press(t, m, "2")
	s := screen(m)
	if strings.Contains(s, "OWNER") {
		t.Errorf("got:\n%s", s)
	}
	if !strings.Contains(s, "secret") {
		t.Errorf("the row lost its columns:\n%s", s)
	}
}

func TestColumnsGoWhenNarrow(t *testing.T) {
	for _, tt := range []struct {
		w    int
		want []string
		gone []string
	}{
		{120, []string{"COMMENTS", "FILES", "VIS", "UPDATED"}, nil},
		{89, []string{"FILES", "VIS", "UPDATED"}, []string{"COMMENTS"}},
		{79, []string{"VIS", "UPDATED"}, []string{"COMMENTS", "FILES"}},
	} {
		s := screen(loaded(t, fixture(), tt.w, 34))
		for _, want := range tt.want {
			if !strings.Contains(s, want) {
				t.Errorf("%d: no %s", tt.w, want)
			}
		}
		for _, gone := range tt.gone {
			if strings.Contains(s, gone) {
				t.Errorf("%d: still %s", tt.w, gone)
			}
		}
	}
}

func TestLeadColumnIsTheFileByDefault(t *testing.T) {
	o := options(t, &source{pages: onePage(fixture())})
	o.Config.List.Lead = config.Default().List.Lead
	m := newModel(t, o, 120, 34)
	settle(t, m, m.Init())
	s := screen(m)
	if !regexp.MustCompile(`FILE +DESCRIPTION`).MatchString(s) || !regexp.MustCompile(`main\.go \+2 +a public gist`).MatchString(s) {
		t.Errorf("got:\n%s", s)
	}
}

func click(t *testing.T, m *Model, x, y int) {
	t.Helper()
	_ = m.render()
	_, cmd := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	settle(t, m, cmd)
}

func TestClickARow(t *testing.T) {
	m := loaded(t, manyGists(5), 120, 34)
	click(t, m, 20, m.layout().listY+3)
	if m.tab().cur != 3 {
		t.Errorf("cursor at %d", m.tab().cur)
	}
}

func TestClickATab(t *testing.T) {
	src := &source{pages: onePage(manyGists(1))}
	m := newModel(t, options(t, src), 120, 34)
	settle(t, m, m.Init())
	screenText, _, _ := strings.Cut(screen(m), "\n")
	click(t, m, ansi.StringWidth(screenText[:strings.Index(screenText, "starred")]), 0)
	if m.cur != tabStarred {
		t.Errorf("tab is %d", m.cur)
	}
}

func TestClickAFileTab(t *testing.T) {
	m := loaded(t, fixture(), 120, 34)
	l := m.layout()
	lines := strings.Split(screen(m), "\n")
	click(t, m, ansi.StringWidth(lines[l.boxY][:strings.Index(lines[l.boxY], "go.mod")]), l.boxY)
	if g, _ := m.selected(); m.selectedFile(g) != 2 {
		t.Errorf("file %d", m.selectedFile(g))
	}
}

func TestWheelMovesTheList(t *testing.T) {
	m := loaded(t, manyGists(5), 120, 34)
	l := m.layout()
	m.Update(tea.MouseWheelMsg{X: 5, Y: l.listY, Button: tea.MouseWheelDown})
	m.Update(tea.MouseWheelMsg{X: 5, Y: l.listY, Button: tea.MouseWheelDown})
	m.Update(tea.MouseWheelMsg{X: 5, Y: l.listY, Button: tea.MouseWheelUp})
	if m.tab().cur != 1 {
		t.Errorf("cursor at %d", m.tab().cur)
	}
	m.Update(tea.MouseWheelMsg{X: 5, Y: l.boxY + 1, Button: tea.MouseWheelDown})
	if m.tab().cur != 1 {
		t.Error("the wheel over the preview moved the list")
	}
}

func TestDragThePreviewsEdge(t *testing.T) {
	store := &layoutStore{}
	o := options(t, &source{pages: onePage(manyGists(1))})
	o.Layout = store
	m := newModel(t, o, 120, 34)
	settle(t, m, m.Init())
	l := m.layout()
	click(t, m, 60, l.boxY)
	if !m.dragging {
		t.Fatal("not dragging")
	}
	m.Update(tea.MouseMotionMsg{X: 100, Y: l.boxY - 4, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{X: 100, Y: l.boxY - 4, Button: tea.MouseLeft})
	if got := m.layout().boxH; got != l.boxH+4 {
		t.Errorf("height is %d, was %d", got, l.boxH)
	}
	if store.h != l.boxH+4 {
		t.Errorf("kept %d", store.h)
	}
	m2 := newModel(t, o, 120, 34)
	if got := m2.layout().boxH; got != l.boxH+4 {
		t.Errorf("the next launch has %d", got)
	}
}

func TestDragStopsWhereThePartsWouldGo(t *testing.T) {
	m := loaded(t, manyGists(1), 120, 34)
	click(t, m, 60, m.layout().boxY)
	m.Update(tea.MouseMotionMsg{X: 100, Y: 0})
	if got := m.layout().listRows; got != minListRows {
		t.Errorf("list has %d rows", got)
	}
	m.Update(tea.MouseMotionMsg{X: 100, Y: 33})
	if got := m.layout().boxH; got != minBoxRows {
		t.Errorf("preview has %d rows", got)
	}
}

func TestMouseStaysOutOfTheKeyList(t *testing.T) {
	m := loaded(t, manyGists(5), 120, 34)
	press(t, m, "?")
	click(t, m, 20, m.layout().listY+3)
	if m.tab().cur != 0 {
		t.Errorf("cursor at %d", m.tab().cur)
	}
}

func TestStatusGoesAfterItsTime(t *testing.T) {
	clock := now
	o := options(t, &source{pages: onePage(manyGists(1))})
	o.Now = func() time.Time { return clock }
	m := newModel(t, o, 120, 34)
	settle(t, m, m.Init())
	if !strings.Contains(screen(m), "saved to the cache") {
		t.Fatal("no status")
	}
	clock = clock.Add(statusline.InfoFor)
	if strings.Contains(screen(m), "saved to the cache") {
		t.Error("the status stayed")
	}
}

func TestEscapeSequencesDoNotReachTheTerminal(t *testing.T) {
	m := loaded(t, fixture(), 120, 34)
	press(t, m, "G")
	out := m.render()
	// What is left of the view's own styling has only the color sequences
	for _, bad := range []string{"\x1b[2J", "\x1b]0;", "\x07", "\x1b[31m"} {
		if strings.Contains(out, bad) {
			t.Errorf("%q reached the screen", bad)
		}
	}
	if !strings.Contains(screen(m), "escape sequence    and a tab") {
		t.Errorf("got:\n%s", screen(m))
	}
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v; run go test ./internal/tui -update", err)
	}
	if string(want) != got {
		t.Errorf("%s differs; run go test ./internal/tui -update and look at the diff\ngot:\n%s", path, got)
	}
}

func TestGolden(t *testing.T) {
	for _, width := range []int{60, 80, 120, 200} {
		golden(t, fmt.Sprintf("list-%d", width), screen(loaded(t, fixture(), width, 30)))
	}
	cases := states(t, 120, 30)
	golden(t, "help", screen(cases[uiHelp]))
	golden(t, "authfail", screen(cases[uiAuthFail]))
	ids := regexp.MustCompile(`\b[0-9a-f]{7}\b`)
	ages := regexp.MustCompile(`\b(now|\d+[smhdy]|\d+mo)( ago)?\b`)
	stable := func(m *Model) string { return ages.ReplaceAllString(ids.ReplaceAllString(screen(m), "<id>"), "<age>") }
	golden(t, "box-push", stable(cases[uiConfirmPush]))
	golden(t, "box-conflict", stable(cases[uiConfirmConflict]))
	golden(t, "box-delete", stable(cases[uiConfirmDelete]))
	golden(t, "box-discard", stable(cases[uiConfirmDiscard]))
	golden(t, "box-input", stable(cases[uiInput]))
	golden(t, "box-input-conflict", stable(cases[uiInputConflict]))
	golden(t, "box-new", stable(cases[uiNew]))
}

func TestGoldenThemes(t *testing.T) {
	dark := loaded(t, fixture(), 80, 24)
	golden(t, "theme-dark", dark.render())
	light := loaded(t, fixture(), 80, 24)
	light.pal = themes[config.ThemeLatte]
	golden(t, "theme-light", light.render())
}
