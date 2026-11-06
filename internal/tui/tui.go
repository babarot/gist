// Package tui is gist's terminal UI. It reads keys through the keymap by action
// name, never by key, and learns about GitHub only through the Source the app
// package hands it.
package tui

import (
	"context"
	"log/slog"
	"path/filepath"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/cache"
	"github.com/babarot/gist/v2/internal/config"
	"github.com/babarot/gist/v2/internal/edit"
	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/tui/keymap"
	"github.com/babarot/gist/v2/internal/tui/statusline"
)

// Source is what the TUI needs from GitHub. The app package passes the client.
type Source interface {
	Viewer(ctx context.Context) (string, error)
	List(ctx context.Context, etags []string, got func(model.ListPage)) ([]model.ListPage, error)
	Starred(ctx context.Context) ([]model.Gist, error)
	Get(ctx context.Context, id string) (model.Gist, error)
	Comments(ctx context.Context, id string) ([]model.Comment, error)
	Delete(ctx context.Context, id string) error
}

// Layout keeps how the screen was left, for the next launch. The app package
// passes the state store.
type Layout interface {
	PreviewHeight() int
	SavePreviewHeight(h int) error
}

// Options are what the TUI starts with.
type Options struct {
	// Source is GitHub; nil with NoToken.
	Source Source
	// NoToken says no token was found, which is all the TUI then shows.
	NoToken bool
	// Via says where the token came from.
	Via string
	// Host is the GitHub in use, which keeps lists of two apart.
	Host string
	// CacheDir is where lists are kept.
	CacheDir string
	// Layout keeps the screen's layout.
	Layout Layout
	// Editor changes gists.
	Editor *edit.Editor
	// Config is the config file.
	Config config.File
	// Keys are the keys.
	Keys keymap.Map
	// Select is the gist to put the cursor on, when it comes.
	Select string
	// Browse opens a URL in the browser.
	Browse func(url string) error
	// Log is the log.
	Log *slog.Logger
	// Now is the time, for tests; nil uses the clock.
	Now func() time.Time
}

// Tabs of the list.
const (
	tabMine = iota
	tabStarred
	numTabs
)

// tab is one of the lists and where the user is in it.
type tab struct {
	gists []model.Gist
	// list and store are what is kept on disk.
	list  cache.List
	store *cache.Store
	// loaded says gists can be shown, reading says they are being read, and
	// first that it is the first time, so they come in as they are read.
	// readOnce says it was read from GitHub in this run.
	loaded, reading, first, readOnce bool
	// got and estimate are how far a first reading is.
	got, estimate int
	// seq tells the reading under way from one given up.
	seq int
	// cur and top are the row of the cursor and the row shown first.
	cur, top int
	// marks are the gists new or changed since last seen, and fresh when each
	// last changed, for as long as it stands out.
	marks map[string]bool
	fresh map[string]time.Time
}

// Model is the TUI.
type Model struct {
	opt     Options
	ctx     context.Context
	cancel  context.CancelFunc
	pal     palette
	keys    keymap.Map
	line    statusline.Line
	noToken bool
	via     string

	width, height int
	login         string
	// viewerRead says GitHub confirmed login. Nothing is read until it has.
	viewerRead, readingViewer bool
	tabs                      [numTabs]tab
	cur                       int
	fileTab                   map[string]int
	selectID                  string
	helpOpen                  bool
	preview                   previewState
	editing                   *editing
	input                     *inputState
	deleting                  *deleting
	discarding                *discarding
	// sending says something is on its way that the screen depends on the end of,
	// so that no key can act until it ends.
	sending bool
	// quitErr is what to say once the terminal is back.
	quitErr      error
	warnedDrafts bool
	// writing are the gists being written to, and drafts those with drafts.
	writing map[string]string
	drafts  map[string]bool
	// runEditor gives the terminal to the editor; tests give another.
	runEditor func(args []string) tea.Cmd
	side      int
	plain     bool
	boxHeight int
	lastClick time.Time
	lastRow   int
	dragging  bool

	spinFrame int
	spinning  bool
	timerAt   time.Time
	// clockless runs no clocks, for the tests, which run each command to its
	// end.
	clockless bool
	hits      []hit
}

const defaultBoxHeight = 17

// New makes the model. What was kept of the lists shows at once, under the
// account that last read them, which the first answer from GitHub confirms.
func New(o Options) *Model {
	if o.Now == nil {
		o.Now = time.Now
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Model{
		opt:       o,
		ctx:       ctx,
		cancel:    cancel,
		pal:       themes[config.ThemeDefault],
		keys:      o.Keys,
		noToken:   o.NoToken,
		via:       o.Via,
		fileTab:   map[string]int{},
		writing:   map[string]string{},
		drafts:    map[string]bool{},
		runEditor: runProgram,
		selectID:  o.Select,
		boxHeight: defaultBoxHeight,
		plain:     o.Config.Preview.Markdown == config.MarkdownPlain,
		preview:   newPreviewState(filepath.Join(o.CacheDir, "contents")),
	}
	if o.Config.Theme == config.ThemeLatte {
		m.pal = themes[config.ThemeLatte]
	}
	if o.Layout != nil {
		if h := o.Layout.PreviewHeight(); h > 0 {
			m.boxHeight = h
		}
	}
	for i := range m.tabs {
		m.tabs[i].marks, m.tabs[i].fresh = map[string]bool{}, map[string]time.Time{}
	}
	m.readDrafts()
	if !m.noToken {
		m.openStores(cache.Viewer(m.storePath(tabMine)))
	}
	return m
}

func (m *Model) storePath(t int) string {
	return filepath.Join(m.opt.CacheDir, []string{cache.MineFile, cache.StarredFile}[t])
}

// openStores points the tabs at the kept lists of an account and shows them.
// The lists of another account never show under this one.
func (m *Model) openStores(login string) {
	m.login = login
	if login == "" {
		return
	}
	for i := range m.tabs {
		t := &m.tabs[i]
		t.store = cache.Open(m.storePath(i), m.opt.Host, login)
		l, ok, err := t.store.Load()
		if err != nil {
			m.opt.Log.Warn("the kept list cannot be read", "err", err.Error())
		}
		if ok {
			t.list, t.gists, t.loaded = l, l.Gists, true
		}
	}
	m.applySelect()
}

// Run runs the TUI until the user quits.
func Run(o Options) error {
	m := New(o)
	defer m.cancel()
	_, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	return m.quitErr
}

// Init asks for the terminal's background and who the token belongs to, which
// starts the reading.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{}
	if m.opt.Config.Theme == config.ThemeAuto {
		cmds = append(cmds, tea.RequestBackgroundColor)
	}
	if !m.noToken {
		cmds = append(cmds, m.readViewer(), m.spin())
	}
	return tea.Batch(cmds...)
}

// Update handles a message, and keeps the clocks running that it needs.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.update(msg)
	return m, tea.Batch(cmd, m.readContents(), m.dwell(), m.spin(), m.timer())
}

func (m *Model) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.keepCursorShown()
	case tea.BackgroundColorMsg:
		if m.opt.Config.Theme == config.ThemeAuto && !msg.IsDark() {
			m.pal = themes[config.ThemeLatte]
		}
	case tea.KeyPressMsg:
		m.line.KeyPressed()
		return m.key(msg)
	case tea.MouseMsg:
		return m.mouse(msg)
	case viewerMsg:
		return m.gotViewer(msg)
	case pageMsg:
		return m.gotPage(msg)
	case listDoneMsg:
		return m.gotList(msg)
	case starredMsg:
		m.gotStarred(msg)
	case contentMsg:
		m.gotContent(msg)
	case commentsMsg:
		m.gotComments(msg)
	case dwellMsg:
		return m.dwelled(msg)
	case beganMsg:
		return m.gotBegan(msg)
	case editedMsg:
		return m.gotEdited(msg)
	case pushedMsg:
		return m.gotPushed(msg)
	case reloadedMsg:
		m.gotReloaded(msg)
	case inputSentMsg:
		m.gotInputSent(msg)
	case deletedMsg:
		m.gotDeleted(msg)
	case fileRemovedMsg:
		m.gotFileRemoved(msg)
	case spinMsg:
		m.spinning = false
		m.spinFrame++
	case timerMsg:
		m.timerAt = time.Time{}
	case browsedMsg:
		if msg.err != nil {
			m.line.Fail(m.now(), msg.err.Error())
		}
	}
	return nil
}

func (m *Model) now() time.Time { return m.opt.Now() }

func (m *Model) tab() *tab { return &m.tabs[m.cur] }

// busy says something is being read, for which the spinner turns.
func (m *Model) busy() bool {
	return m.readingViewer || m.tabs[tabMine].reading || m.tabs[tabStarred].reading ||
		m.preview.running > 0 || len(m.preview.readingComments) > 0 || len(m.writing) > 0 || m.sending
}

type spinMsg struct{}

const spinEvery = 80 * time.Millisecond

func (m *Model) spin() tea.Cmd {
	if m.spinning || m.clockless || !m.busy() {
		return nil
	}
	m.spinning = true
	return tea.Tick(spinEvery, func(time.Time) tea.Msg { return spinMsg{} })
}

type timerMsg struct{}

// freshFor is how long a gist that just changed stands out.
const freshFor = 6 * time.Second

// timer wakes the screen when what it shows changes by time alone.
func (m *Model) timer() tea.Cmd {
	if m.clockless {
		return nil
	}
	now := m.now()
	due := m.line.Due(now)
	for i := range m.tabs {
		for _, at := range m.tabs[i].fresh {
			if end := at.Add(freshFor); end.After(now) && (due.IsZero() || end.Before(due)) {
				due = end
			}
		}
	}
	if due.IsZero() || (!m.timerAt.IsZero() && !m.timerAt.After(due)) {
		return nil
	}
	m.timerAt = due
	return tea.Tick(due.Sub(now), func(time.Time) tea.Msg { return timerMsg{} })
}

// selected is the gist under the cursor.
func (m *Model) selected() (model.Gist, bool) {
	t := m.tab()
	if t.cur < 0 || t.cur >= len(t.gists) {
		return model.Gist{}, false
	}
	return t.gists[t.cur], true
}

// selectedFile is the index of the file shown of g.
func (m *Model) selectedFile(g model.Gist) int {
	return min(m.fileTab[g.ID], max(0, len(g.Files)-1))
}

// applySelect puts the cursor on the gist asked for, once it is there.
func (m *Model) applySelect() {
	if m.selectID == "" {
		return
	}
	t := &m.tabs[tabMine]
	i := slices.IndexFunc(t.gists, func(g model.Gist) bool { return g.ID == m.selectID })
	if i < 0 {
		return
	}
	t.cur = i
	m.selectID = ""
	m.keepCursorShown()
}

// moveTo puts the cursor on row i of the tab, and marks that gist as seen.
func (m *Model) moveTo(i int) {
	t := m.tab()
	if len(t.gists) == 0 {
		return
	}
	t.cur = min(max(0, i), len(t.gists)-1)
	delete(t.marks, t.gists[t.cur].ID)
	m.keepCursorShown()
}

// keepCursorShown scrolls the list so that the cursor's row is in it.
func (m *Model) keepCursorShown() {
	rows := m.layout().listRows
	for i := range m.tabs {
		t := &m.tabs[i]
		t.cur = max(0, min(t.cur, len(t.gists)-1))
		if t.cur < t.top {
			t.top = t.cur
		}
		if rows > 0 && t.cur >= t.top+rows {
			t.top = t.cur - rows + 1
		}
		t.top = max(0, min(t.top, max(0, len(t.gists)-rows)))
	}
}

// replace puts gists in place of what the tab shows, the cursor staying on its
// gist, at the height it had on the screen. A row put in above it moves the
// list instead of the cursor, so the next key acts on the gist it was going to.
func (m *Model) replace(t *tab, gists []model.Gist) {
	var id string
	if t.cur >= 0 && t.cur < len(t.gists) {
		id = t.gists[t.cur].ID
	}
	height := t.cur - t.top
	t.gists = gists
	if i := slices.IndexFunc(gists, func(g model.Gist) bool { return g.ID == id }); i >= 0 {
		t.cur = i
		t.top = i - height
	}
	m.keepCursorShown()
	m.applySelect()
}
