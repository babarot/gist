package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/babarot/gist/v2/internal/cache"
	"github.com/babarot/gist/v2/internal/config"
	"github.com/babarot/gist/v2/internal/model"
)

const sample = `# 設計書の見出し

これは日本語の段落で、幅で折り返す。**太字** と ` + "`コード`" + ` と [リンク](https://example.com) を含む（かっこ）。

## リスト

- 項目その1
- [x] 済んだ
- [ ] まだ

> 引用の行

| 列1 | 列2 |
|---|---|
| a | b |

![図](https://img.example.com/a.png)

` + "```go\nfunc main() {}\n```\n"

func file(name, content string) model.File {
	return model.File{Name: name, Content: content, Size: int64(len(content)), Type: "text/plain"}
}

// preview is a model on one gist, with these files read.
func preview(t *testing.T, w, h int, files ...model.File) (*Model, *source) {
	t.Helper()
	src := &source{files: map[string]model.File{}}
	names := make([]string, len(files))
	for i, f := range files {
		src.files[f.Name] = f
		names[i] = f.Name
	}
	g := gist("1111111", "a gist", time.Hour, names...)
	src.pages = onePage([]model.Gist{g})
	m := newModel(t, options(t, src), w, h)
	settle(t, m, m.Init())
	return m, src
}

func TestMarkdownIsDrawn(t *testing.T) {
	m, _ := preview(t, 120, 40, file("a.md", sample))
	m.boxHeight = 34
	s := screen(m)
	for _, want := range []string{"設計書の見出し", "太字", "コード", "リンク", "• 項目その1", "[✓] 済んだ", "[ ] まだ", "▎ 引用の行", "列1", "[image: 図 · img.example.com]", " go "} {
		if !strings.Contains(s, want) {
			t.Errorf("no %q in:\n%s", want, s)
		}
	}
	for _, gone := range []string{"# 設計書", "**", "- [x]", "![図]"} {
		if strings.Contains(s, gone) {
			t.Errorf("%q is drawn as written:\n%s", gone, s)
		}
	}
}

func TestMarkdownCanBeShownAsWritten(t *testing.T) {
	m, _ := preview(t, 120, 40, file("a.md", sample))
	press(t, m, "m")
	s := screen(m)
	if !strings.Contains(s, "1  # 設計書の見出し") || !strings.Contains(s, "m rendered") {
		t.Errorf("got:\n%s", s)
	}
	press(t, m, "m")
	if s := screen(m); strings.Contains(s, "# 設計書") {
		t.Errorf("got:\n%s", s)
	}
}

func TestMarkdownAsWrittenFromTheConfig(t *testing.T) {
	o := options(t, &source{})
	o.Config.Preview.Markdown = config.MarkdownPlain
	if m := newModel(t, o, 80, 24); !m.plain {
		t.Error("the config was not read")
	}
}

func TestCodeHasLineNumbers(t *testing.T) {
	m, _ := preview(t, 120, 40, file("main.go", "package main\n\tfunc main() {}\n"))
	s := screen(m)
	if !strings.Contains(s, "   1  package main") || !strings.Contains(s, "   2      func main() {}") {
		t.Errorf("got:\n%s", s)
	}
	if !strings.Contains(s, "type       go") || !strings.Contains(s, "2 lines") || !strings.Contains(s, "revisions  2") {
		t.Errorf("got:\n%s", s)
	}
}

func TestLongCodeLinesAreCut(t *testing.T) {
	m, _ := preview(t, 80, 24, file("a.txt", strings.Repeat("x", 300)+"\n"))
	line := ""
	for l := range strings.SplitSeq(screen(m), "\n") {
		if strings.Contains(l, "   1  x") {
			line = l
		}
	}
	if !strings.Contains(line, "…") || !strings.Contains(line, "│ visibility") {
		t.Errorf("got %q", line)
	}
}

func TestLargeFileShowsItsFirstLines(t *testing.T) {
	m, _ := preview(t, 120, 40, file("a.txt", strings.Repeat("line\n", 2300)))
	s := screen(m)
	if !strings.Contains(s, "1000 of 2300 lines") {
		t.Errorf("got:\n%s", s)
	}
	for range 400 {
		press(t, m, "J")
	}
	s = screen(m)
	if !strings.Contains(s, "1000  line") || strings.Contains(s, "1001  line") {
		t.Errorf("got:\n%s", s)
	}
}

func TestTruncatedFileSaysSo(t *testing.T) {
	f := file("big.txt", "start\n")
	f.Truncated = true
	m, _ := preview(t, 120, 40, f)
	if s := screen(m); !strings.Contains(s, " truncated ") {
		t.Errorf("got:\n%s", s)
	}
}

func TestBinaryFile(t *testing.T) {
	f := model.File{Name: "a.png", Type: "image/png", Size: 48 << 10, Binary: true}
	m, _ := preview(t, 120, 40, f)
	s := screen(m)
	if !strings.Contains(s, "binary file · image/png · 48.0 KB") || !strings.Contains(s, "o opens it in the browser") {
		t.Errorf("got:\n%s", s)
	}
}

func TestLoadingShowsTheFile(t *testing.T) {
	g := gist("1111111", "d", time.Hour, "a.go")
	m := newModel(t, options(t, &source{}), 120, 34)
	m.login = "alice"
	m.tabs[tabMine].loaded = true
	m.replace(&m.tabs[tabMine], []model.Gist{g})
	if s := screen(m); !strings.Contains(s, "loading a.go…") {
		t.Errorf("got:\n%s", s)
	}
}

func TestFailedReadCanBeTriedAgain(t *testing.T) {
	m, src := preview(t, 120, 34, file("a.go", "content of a.go\n"))
	id := m.tab().gists[0].ID
	if err := m.preview.disk.Remove(id); err != nil {
		t.Fatal(err)
	}
	m.preview.forget(id)
	src.getErr = errors.New("no way through")
	src.gets = nil
	press(t, m, "R")
	if s := screen(m); !strings.Contains(s, "no way through") || !strings.Contains(s, "could not be read") {
		t.Errorf("got:\n%s", s)
	}
	n := len(src.gets)
	press(t, m, "j")
	if len(src.gets) != n {
		t.Error("a failed read was asked for again by itself")
	}
	src.getErr = nil
	press(t, m, "R")
	if s := screen(m); !strings.Contains(s, "content of a.go") {
		t.Errorf("got:\n%s", s)
	}
}

func TestAGistNewerThanTheListIsReadOnce(t *testing.T) {
	g := gist("1111111", "d", time.Hour, "a.go")
	src := &source{pages: onePage([]model.Gist{g})}
	src.newer = time.Minute
	m := newModel(t, options(t, src), 120, 34)
	settle(t, m, m.Init())
	if len(src.gets) != 1 {
		t.Errorf("read %d times", len(src.gets))
	}
	if s := screen(m); !strings.Contains(s, "content of a.go") {
		t.Errorf("got:\n%s", s)
	}
}

func TestAReadThatEndsAfterItsGistWentIsNotKept(t *testing.T) {
	m, _ := preview(t, 120, 34, file("a.go", "x\n"))
	g := gist("9999999", "gone", time.Hour, "b.go")
	g.Files[0].Content = "secret"
	if err := m.preview.disk.Save(g); err != nil {
		t.Fatal(err)
	}
	m.preview.running++
	m.Update(contentMsg{id: g.ID, gist: g})
	if _, ok := m.preview.contents[g.ID]; ok {
		t.Error("held the content of a gist that is gone")
	}
	if _, ok := m.preview.disk.Load(g.ID, g.Updated); ok {
		t.Error("the content of a gist that is gone stayed on the disk")
	}
}

func TestUnstarredGistIsDropped(t *testing.T) {
	star := gist("7777777", "tmux", time.Hour, "tmux.conf")
	src := &source{pages: onePage(manyGists(1)), starred: []model.Gist{star}}
	o := options(t, src)
	m := newModel(t, o, 120, 34)
	settle(t, m, m.Init())
	press(t, m, "2")
	src.starred = nil
	press(t, m, "R")
	if _, ok := cache.OpenContents(filepath.Join(o.CacheDir, "contents")).Load(star.ID, star.Updated); ok {
		t.Error("the content of an unstarred gist stayed on the disk")
	}
}

func TestAnotherAccountsContentGoes(t *testing.T) {
	src := &source{pages: onePage(manyGists(1))}
	o := options(t, src)
	first := newModel(t, o, 120, 34)
	settle(t, first, first.Init())
	src.login = "bob"
	src.pages = onePage(nil)
	second := newModel(t, o, 120, 34)
	settle(t, second, second.Init())
	g := manyGists(1)[0]
	if _, ok := cache.OpenContents(filepath.Join(o.CacheDir, "contents")).Load(g.ID, g.Updated); ok {
		t.Error("the other account's content stayed on the disk")
	}
}

func TestReadsUnderWayStillCountAfterTheAccountChanged(t *testing.T) {
	src := &source{login: "bob", pages: onePage(nil)}
	m := newModel(t, options(t, src), 120, 34)
	m.login = "alice"
	m.preview.running, m.preview.reading["x"] = 1, true
	m.Update(viewerMsg{login: "bob"})
	if m.preview.running != 1 {
		t.Fatalf("%d running", m.preview.running)
	}
	m.Update(contentMsg{id: "x", notKept: true})
	if m.preview.running != 0 {
		t.Errorf("%d running", m.preview.running)
	}
}

func TestNothingIsAskedBeforeTheAccountIsKnown(t *testing.T) {
	src := &source{login: "bob", pages: onePage(nil)}
	o := options(t, src)
	if err := cache.Open(filepath.Join(o.CacheDir, cache.MineFile), o.Host, "alice").Save(cache.List{Gists: manyGists(3)}); err != nil {
		t.Fatal(err)
	}
	m := newModel(t, o, 120, 34)
	settle(t, m, m.Init())
	if len(src.gets) != 0 {
		t.Errorf("asked for %v", src.gets)
	}
}

func TestKeptListStillFillsFromTheDiskFirst(t *testing.T) {
	src := &source{pages: onePage(manyGists(1))}
	o := options(t, src)
	first := newModel(t, o, 120, 34)
	settle(t, first, first.Init())
	second := New(o)
	second.clockless = true
	t.Cleanup(second.cancel)
	_, cmd := second.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	settle(t, second, cmd)
	if !strings.Contains(screen(second), "content of a.go") {
		t.Errorf("before GitHub answered:\n%s", screen(second))
	}
}

func TestNewerContentIsDrawnOverOlder(t *testing.T) {
	for _, name := range []string{"a.md", "a.go"} {
		m, _ := preview(t, 120, 34, file(name, "old text\n"))
		g := m.tab().gists[0]
		if !strings.Contains(screen(m), "old text") {
			t.Fatalf("got:\n%s", screen(m))
		}
		newer := m.preview.contents[g.ID]
		newer.Updated = now
		newer.Files = []model.File{file(name, "new text\n")}
		m.preview.contents[g.ID] = newer
		if s := screen(m); !strings.Contains(s, "new text") || strings.Contains(s, "old text") {
			t.Errorf("%s: got:\n%s", name, s)
		}
	}
}

func TestMarkdownStopsAtItsLimit(t *testing.T) {
	m, _ := preview(t, 120, 34, file("a.md", strings.Repeat("line\n\n", 2000)))
	g := m.tab().gists[0]
	f, _ := m.shownFile(g)
	if got := len(m.rendered(g, f, 80)); got > maxLines {
		t.Errorf("%d lines drawn", got)
	}
	if s := screen(m); !strings.Contains(s, "1000 of 4000 lines") {
		t.Errorf("got:\n%s", s)
	}
}

func TestDrawnFilesAreBounded(t *testing.T) {
	m, _ := preview(t, 120, 34, file("a.md", "x\n"))
	g := m.tab().gists[0]
	f, _ := m.shownFile(g)
	for w := range maxDrawn * 3 {
		m.rendered(g, f, 10+w)
	}
	if got := len(m.preview.md); got > maxDrawn {
		t.Errorf("%d held", got)
	}
}

func TestContentWidthFitsWithoutAPane(t *testing.T) {
	m, _ := preview(t, 30, 24, file("a.md", strings.Repeat("x ", 400)+"\n"))
	if s := screen(m); strings.Contains(s, "…") {
		t.Errorf("a wrapped line was cut:\n%s", s)
	}
}

func TestContentIsReadAheadForTheRowsShown(t *testing.T) {
	src := &source{pages: onePage(manyGists(40))}
	m := newModel(t, options(t, src), 120, 34)
	settle(t, m, m.Init())
	rows := m.layout().listRows
	if len(src.gets) != rows {
		t.Errorf("read %d gists, want the %d shown", len(src.gets), rows)
	}
	for _, id := range src.gets {
		if !slices.ContainsFunc(m.tab().gists[:rows], func(g model.Gist) bool { return g.ID == id }) {
			t.Errorf("read %s, which is not shown", id)
		}
	}
}

func TestContentIsNotReadTwice(t *testing.T) {
	src := &source{pages: onePage(manyGists(3))}
	m := newModel(t, options(t, src), 120, 34)
	settle(t, m, m.Init())
	n := len(src.gets)
	press(t, m, "j", "k", "j")
	if len(src.gets) != n {
		t.Errorf("read %d more", len(src.gets)-n)
	}
}

func TestReadsAreLimited(t *testing.T) {
	m := newModel(t, options(t, &source{}), 120, 34)
	m.tabs[tabMine].loaded = true
	m.replace(&m.tabs[tabMine], manyGists(20))
	m.readContents()
	if m.preview.running != maxReads {
		t.Errorf("%d at once", m.preview.running)
	}
}

func TestChangedGistIsReadAgain(t *testing.T) {
	g := gist("1111111", "d", time.Hour, "a.go")
	src := &source{pages: onePage([]model.Gist{g})}
	m := newModel(t, options(t, src), 120, 34)
	settle(t, m, m.Init())
	changed := g
	changed.Updated = now
	src.pages = onePage([]model.Gist{changed})
	src.files = map[string]model.File{"a.go": file("a.go", "new content\n")}
	press(t, m, "R")
	if s := screen(m); !strings.Contains(s, "new content") {
		t.Errorf("got:\n%s", s)
	}
}

func TestKeptContentIsUsedWithoutAsking(t *testing.T) {
	g := gist("1111111", "d", time.Hour, "a.go")
	src := &source{pages: onePage([]model.Gist{g})}
	o := options(t, src)
	first := newModel(t, o, 120, 34)
	settle(t, first, first.Init())
	if len(src.gets) != 1 {
		t.Fatalf("read %d", len(src.gets))
	}
	second := newModel(t, o, 120, 34)
	settle(t, second, second.Init())
	if len(src.gets) != 1 {
		t.Error("asked GitHub for what was kept")
	}
	if s := screen(second); !strings.Contains(s, "content of a.go") {
		t.Errorf("got:\n%s", s)
	}
}

func TestContentOfAGoneGistIsDropped(t *testing.T) {
	g := gist("1111111", "d", time.Hour, "a.go")
	other := gist("2222222", "e", 2*time.Hour, "b.go")
	src := &source{pages: onePage([]model.Gist{g, other})}
	o := options(t, src)
	m := newModel(t, o, 120, 34)
	settle(t, m, m.Init())
	src.pages = onePage([]model.Gist{g})
	press(t, m, "R")
	if _, ok := cache.OpenContents(filepath.Join(o.CacheDir, "contents")).Load(other.ID, other.Updated); ok {
		t.Error("the content of a gist that is gone stayed on the disk")
	}
}

func TestFirstTab(t *testing.T) {
	for _, tt := range []struct {
		widths    []int
		cur, room int
		want      int
	}{
		{[]int{10, 10, 10}, 0, 100, 0},
		{[]int{10, 10, 10}, 2, 100, 0},
		{[]int{10, 10, 10}, 2, 32, 0}, // fits exactly: 10+1+10+1+10
		{[]int{10, 10, 10}, 2, 31, 1},
		{[]int{10, 10, 10}, 2, 21, 2}, // with the mark, one is too wide
		{[]int{10, 10, 10}, 2, 5, 2},  // the tab shown stays, however wide
		{[]int{10, 10, 10}, 0, 5, 0},
		{[]int{1}, 0, 0, 0},
	} {
		if got := firstTab(tt.widths, tt.cur, tt.room); got != tt.want {
			t.Errorf("%v at %d in %d: got %d, want %d", tt.widths, tt.cur, tt.room, got, tt.want)
		}
	}
}

// manyFiles is a gist of more files than the frame has room for.
func manyFiles(t *testing.T, w int) *Model {
	t.Helper()
	var files []model.File
	for i := range 11 {
		files = append(files, file(fmt.Sprintf("%02d-a-long-enough-name.md", i), "x\n"))
	}
	m, _ := preview(t, w, 34, files...)
	return m
}

func frameTop(m *Model) string { return strings.Split(screen(m), "\n")[m.layout().boxY] }

// fileStrip is the line under the frame's top, where the tabs of the files
// are, up to the line before the pane.
func fileStrip(m *Model) string {
	line := []rune(strings.Split(screen(m), "\n")[m.layout().boxY+1])
	return string(line[:min(m.paneX(), len(line))])
}

func TestManyFilesLeaveTheFrameTopALine(t *testing.T) {
	m := manyFiles(t, 120)
	top := frameTop(m)
	if !strings.Contains(top, " files (11) ") || strings.Contains(top, "a-long") || !strings.Contains(top, "────") {
		t.Errorf("got %q", top)
	}
	if s := fileStrip(m); !strings.Contains(s, "00-a-long") {
		t.Errorf("got %q", s)
	}
}

func TestOneFileHasItsTabUnderTheTopToo(t *testing.T) {
	m, _ := preview(t, 120, 34, file("only.go", "package main\n"))
	if top := frameTop(m); !strings.Contains(top, " files (1) ") || strings.Contains(top, "only.go") {
		t.Errorf("got %q", top)
	}
	if s := fileStrip(m); !strings.Contains(s, " only.go ") {
		t.Errorf("got %q", s)
	}
}

func TestFileTabsFollowTheFileShown(t *testing.T) {
	m := manyFiles(t, 120)
	if top := fileStrip(m); strings.Contains(top, "‹") || !strings.Contains(top, "…") {
		t.Errorf("at the first: %q", top)
	}
	for i := range 11 {
		if top := fileStrip(m); !strings.Contains(top, fmt.Sprintf("%02d-a-long", i)) {
			t.Errorf("file %d is not in %q", i, top)
		}
		press(t, m, "l")
	}
	if top := fileStrip(m); !strings.Contains(top, "‹") || strings.Contains(top, "…") {
		t.Errorf("at the last: %q", top)
	}
	for i := 10; i >= 0; i-- {
		if top := fileStrip(m); !strings.Contains(top, fmt.Sprintf("%02d-a-long", i)) {
			t.Errorf("going back, file %d is not in %q", i, top)
		}
		press(t, m, "h")
	}
}

func TestAClickOnAFileTabAfterTheyMoved(t *testing.T) {
	m := manyFiles(t, 120)
	press(t, m, "l", "l", "l", "l", "l", "l")
	top := fileStrip(m)
	x := ansi.StringWidth(top[:strings.Index(top, "05-a-long")])
	click(t, m, x, m.layout().boxY+1)
	if g, _ := m.current(); m.selectedFile(g) != 5 {
		t.Errorf("on file %d", m.selectedFile(g))
	}
}

func TestATabWiderThanTheFrameIsCut(t *testing.T) {
	m, _ := preview(t, 60, 24, file("a.txt", "x\n"), file(strings.Repeat("長", 40)+".txt", "y\n"))
	press(t, m, "l")
	top := fileStrip(m)
	if !strings.Contains(top, "長") || strings.Count(top, "…") != 1 {
		t.Errorf("got %q", top)
	}
	if got := ansi.StringWidth(top); got > 60 {
		t.Errorf("%d wide", got)
	}
}

func TestSideTabsStayWhereTheyAre(t *testing.T) {
	m := manyFiles(t, 120)
	for range 10 {
		press(t, m, "l")
	}
	if top := frameTop(m); !strings.Contains(top, "┬") || !strings.Contains(top, " info ") {
		t.Errorf("got %q", top)
	}
}

func TestScrollStopsAtTheEnds(t *testing.T) {
	m, _ := preview(t, 120, 34, file("a.txt", strings.Repeat("line\n", 40)))
	inner := m.contentRoom()
	press(t, m, "K")
	if s := screen(m); !strings.Contains(s, "   1  line") {
		t.Errorf("got:\n%s", s)
	}
	for range 20 {
		press(t, m, "J")
	}
	s := screen(m)
	if !strings.Contains(s, "  40  line") || strings.Contains(s, fmt.Sprintf("%4d  line", 40-inner)) {
		t.Errorf("got:\n%s", s)
	}
}

func TestScrollIsKeptForEachFile(t *testing.T) {
	long := strings.Repeat("line\n", 40)
	m, _ := preview(t, 120, 34, file("a.txt", long), file("b.txt", long))
	press(t, m, "J")
	press(t, m, "l")
	if s := screen(m); !strings.Contains(s, "   1  line") {
		t.Errorf("the next file was scrolled:\n%s", s)
	}
	press(t, m, "h")
	if s := screen(m); strings.Contains(s, "   1  line") {
		t.Errorf("the scroll was lost:\n%s", s)
	}
}

func TestThumbAt(t *testing.T) {
	for _, tt := range []struct {
		first, room, total, from, to int
	}{
		{0, 10, 100, 0, 1},   // a small thumb at the top
		{90, 10, 100, 9, 10}, // at the very end
		{1, 10, 100, 1, 2},   // moved at all, it leaves the top
		{89, 10, 100, 8, 9},  // not at the end, it does not touch it
		{0, 10, 20, 0, 5},    // half shows, half the track
		{10, 10, 20, 5, 10},
		{5, 10, 20, 3, 8},
	} {
		from, to := thumbAt(tt.first, tt.room, tt.total)
		if from != tt.from || to != tt.to {
			t.Errorf("%+v: got %d, %d", tt, from, to)
		}
	}
}

func thumbRows(m *Model, x int) []int {
	var rows []int
	for y, row := range strings.Split(screen(m), "\n") {
		r := []rune(ansi.Cut(row, x, x+1))
		if len(r) == 1 && r[0] == '┃' {
			rows = append(rows, y)
		}
	}
	return rows
}

func TestAScrollbarShowsWhereTheContentIs(t *testing.T) {
	m, _ := preview(t, 120, 34, file("a.txt", strings.Repeat("line\n", 100)))
	l := m.layout()
	x := m.paneX()
	top := thumbRows(m, x)
	if len(top) == 0 || top[0] != contentTop(l) {
		t.Fatalf("at the top: %v", top)
	}
	for range 200 {
		press(t, m, "J")
	}
	bottom := thumbRows(m, x)
	if len(bottom) == 0 || bottom[len(bottom)-1] != l.boxY+l.boxH-2 {
		t.Errorf("at the end: %v", bottom)
	}
}

func TestNoScrollbarWhenAllShows(t *testing.T) {
	m, _ := preview(t, 120, 34, file("a.txt", "one\ntwo\n"))
	if s := screen(m); strings.Contains(s, "┃") {
		t.Errorf("got:\n%s", s)
	}
}

func TestScrollbarWithoutAPane(t *testing.T) {
	m, _ := preview(t, 30, 24, file("a.txt", strings.Repeat("line\n", 100)))
	if rows := thumbRows(m, 29); len(rows) == 0 {
		t.Errorf("got:\n%s", screen(m))
	}
}

func TestScrollbarOfComments(t *testing.T) {
	m, _ := commented(t, 30)
	press(t, m, "c")
	if rows := thumbRows(m, 119); len(rows) == 0 {
		t.Errorf("got:\n%s", screen(m))
	}
}

func TestWheelOverTheContent(t *testing.T) {
	m, _ := preview(t, 120, 34, file("a.txt", strings.Repeat("line\n", 40)))
	l := m.layout()
	m.Update(wheelDown(5, l.boxY+2))
	if s := screen(m); strings.Contains(s, "   1  line") || !strings.Contains(s, "   2  line") {
		t.Errorf("got:\n%s", s)
	}
}

func wheelDown(x, y int) tea.MouseWheelMsg {
	return tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelDown}
}

func comments(n int) []model.Comment {
	var out []model.Comment
	for i := range n {
		out = append(out, model.Comment{ID: int64(i + 1), Author: "bob", Body: fmt.Sprintf("comment **%d**", i),
			Created: now.Add(-time.Hour), Updated: now.Add(-time.Hour)})
	}
	return out
}

func commented(t *testing.T, n int) (*Model, *source) {
	t.Helper()
	g := gist("1111111", "d", time.Hour, "a.go")
	g.Comments = n
	src := &source{pages: onePage([]model.Gist{g, gist("2222222", "e", 2*time.Hour, "b.go")}),
		comments: map[string][]model.Comment{g.ID: comments(n)}}
	m := newModel(t, options(t, src), 120, 34)
	settle(t, m, m.Init())
	return m, src
}

func TestCommentsPane(t *testing.T) {
	m, _ := commented(t, 2)
	press(t, m, "c")
	s := screen(m)
	if !strings.Contains(s, "bob · 1h ago") || !strings.Contains(s, "comment 0") || !strings.Contains(s, "comments (2)") {
		t.Errorf("got:\n%s", s)
	}
	if m.uiState() != uiListComments {
		t.Errorf("state is %s", m.uiState())
	}
	press(t, m, "c")
	if m.uiState() != uiList {
		t.Errorf("state is %s", m.uiState())
	}
}

func TestCommentsPaneKeepsTheWidth(t *testing.T) {
	m, _ := commented(t, 1)
	info := m.paneX()
	press(t, m, "c")
	if m.paneX() != info {
		t.Errorf("the pane went from %d to %d", info, m.paneX())
	}
}

func dragPane(t *testing.T, m *Model, to int) {
	t.Helper()
	y := m.layout().boxY + 3
	click(t, m, m.paneX(), y)
	if !m.draggingPane {
		t.Fatal("not dragging the pane")
	}
	m.Update(tea.MouseMotionMsg{X: to, Y: y, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{X: to, Y: y, Button: tea.MouseLeft})
}

func TestDragThePanesLine(t *testing.T) {
	store := &layoutStore{}
	src := &source{pages: onePage(manyGists(1))}
	o := options(t, src)
	o.Layout = store
	m := newModel(t, o, 120, 34)
	settle(t, m, m.Init())
	dragPane(t, m, 70)
	if m.paneX() != 70 || store.w != 120-3-70 {
		t.Errorf("the line is at %d, kept %d", m.paneX(), store.w)
	}
	if m2 := newModel(t, o, 120, 34); m2.paneX() != 70 {
		t.Errorf("the next launch has it at %d", m2.paneX())
	}
	if m.contentWidth() != 70-4 {
		t.Errorf("the content is %d wide", m.contentWidth())
	}
}

func TestPanesLineStopsWhereThePartsWouldGo(t *testing.T) {
	m := loaded(t, manyGists(1), 120, 34)
	dragPane(t, m, 2)
	if m.paneX() != minContent {
		t.Errorf("the line is at %d", m.paneX())
	}
	m.paneClick = time.Time{}
	dragPane(t, m, 119)
	if w := 120 - 3 - m.paneX(); w != infoWidth {
		t.Errorf("the pane is %d wide", w)
	}
}

func TestAWidePaneGivesWayOnASmallerScreen(t *testing.T) {
	store := &layoutStore{w: 150}
	o := options(t, &source{pages: onePage(manyGists(1))})
	o.Layout = store
	m := newModel(t, o, 120, 34)
	if m.paneX() != minContent {
		t.Errorf("the line is at %d", m.paneX())
	}
	if store.w != 150 {
		t.Errorf("kept %d", store.w)
	}
}

func TestDoubleClickThePanesLinePutsItBack(t *testing.T) {
	store := &layoutStore{}
	o := options(t, &source{pages: onePage(manyGists(1))})
	o.Layout = store
	m := newModel(t, o, 120, 34)
	settle(t, m, m.Init())
	start := m.paneX()
	dragPane(t, m, 60)
	click(t, m, m.paneX(), m.layout().boxY+3)
	if m.paneX() != start || store.w != 0 || m.draggingPane {
		t.Errorf("the line is at %d, kept %d", m.paneX(), store.w)
	}
}

func TestCommentsAreReadWhenTheCursorRests(t *testing.T) {
	m, src := commented(t, 2)
	if len(src.commentReads) != 0 {
		t.Fatalf("read before the wait: %v", src.commentReads)
	}
	seq := m.preview.dwellSeq
	_, cmd := m.Update(dwellMsg{seq, m.preview.dwellOn})
	settle(t, m, cmd)
	if len(src.commentReads) != 1 {
		t.Errorf("read %v", src.commentReads)
	}
	press(t, m, "c")
	if s := screen(m); !strings.Contains(s, "comment 0") || strings.Contains(s, "loading comments") {
		t.Errorf("not there when asked for:\n%s", s)
	}
	if len(src.commentReads) != 1 {
		t.Errorf("read again: %v", src.commentReads)
	}
}

func TestCommentsPassedOverAreNotRead(t *testing.T) {
	m, src := commented(t, 2)
	seq := m.preview.dwellSeq
	on := m.preview.dwellOn
	press(t, m, "j")
	_, cmd := m.Update(dwellMsg{seq, on})
	settle(t, m, cmd)
	if len(src.commentReads) != 0 {
		t.Errorf("read %v", src.commentReads)
	}
}

func TestNoCommentsAreNotRead(t *testing.T) {
	m, src := commented(t, 2)
	press(t, m, "j", "c")
	s := screen(m)
	if !strings.Contains(s, "no comments") || len(src.commentReads) != 0 {
		t.Errorf("read %v:\n%s", src.commentReads, s)
	}
}

func TestCommentsFollowTheCursorWhenOpen(t *testing.T) {
	m, src := commented(t, 2)
	press(t, m, "j", "c", "k")
	if len(src.commentReads) != 1 {
		t.Errorf("read %v", src.commentReads)
	}
	if s := screen(m); !strings.Contains(s, "comment 0") {
		t.Errorf("got:\n%s", s)
	}
}

func TestChangedCommentCountReadsThemAgain(t *testing.T) {
	m, src := commented(t, 2)
	press(t, m, "c")
	first := src.pages[0].Gists[0]
	first.Comments = 3
	src.comments[first.ID] = comments(3)
	src.pages = onePage([]model.Gist{first})
	press(t, m, "R")
	press(t, m, "c", "c")
	if s := screen(m); !strings.Contains(s, "comment 2") {
		t.Errorf("got:\n%s", s)
	}
}

func TestCommentsOpenAreReadAgainAfterTheCountChanged(t *testing.T) {
	m, src := commented(t, 2)
	press(t, m, "c")
	first := src.pages[0].Gists[0]
	first.Comments = 3
	src.comments[first.ID] = comments(3)
	src.pages = onePage([]model.Gist{first})
	press(t, m, "R")
	if s := screen(m); !strings.Contains(s, "comment 2") {
		t.Errorf("got:\n%s", s)
	}
}

func TestFailedCommentsAreNotAskedForForEver(t *testing.T) {
	m, src := commented(t, 2)
	src.commentsErr = errors.New("no way through")
	press(t, m, "c", "j", "k")
	if len(src.commentReads) != 1 {
		t.Errorf("read %d times", len(src.commentReads))
	}
	src.commentsErr = nil
	press(t, m, "R")
	if s := screen(m); !strings.Contains(s, "comment 0") {
		t.Errorf("got:\n%s", s)
	}
}

func TestMouseWorksWithCommentsOpen(t *testing.T) {
	m, _ := commented(t, 30)
	press(t, m, "c")
	l := m.layout()
	m.Update(wheelDown(m.paneX()+5, l.boxY+2))
	g := m.tab().gists[0]
	if m.preview.commentScroll[g.ID] != 1 {
		t.Errorf("scrolled to %d", m.preview.commentScroll[g.ID])
	}
	click(t, m, 20, l.listY+1)
	if m.tab().cur != 1 {
		t.Errorf("cursor at %d", m.tab().cur)
	}
}

func TestCommentsScroll(t *testing.T) {
	m, _ := commented(t, 30)
	press(t, m, "c")
	if !strings.Contains(screen(m), "┃") {
		t.Errorf("no sign of more:\n%s", screen(m))
	}
	for range 40 {
		press(t, m, "J")
	}
	s := screen(m)
	if !strings.Contains(s, "comment 29") || strings.Contains(s, "comment 0") {
		t.Errorf("got:\n%s", s)
	}
}

func TestAGapInACommentIsNotTheEndOfIt(t *testing.T) {
	m, src := commented(t, 2)
	src.comments["1111111"+strings.Repeat("0", 25)][0].Body = "first\n\nsecond"
	press(t, m, "c")
	lines := strings.Split(screen(m), "\n")
	i := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "│ first") })
	if i < 0 || !strings.Contains(lines[i+1], "│ │ ") || !strings.Contains(lines[i+2], "│ second") {
		t.Fatalf("the gap is not marked as part of the comment:\n%s", strings.Join(lines, "\n"))
	}
	if strings.Contains(lines[i+3], "│ │") {
		t.Errorf("the gap after the comment is marked:\n%s", strings.Join(lines, "\n"))
	}
}

func TestMyCommentsStandOut(t *testing.T) {
	m, src := commented(t, 1)
	src.comments["1111111"+strings.Repeat("0", 25)][0].Author = "alice"
	press(t, m, "c")
	lines := m.commentLines(m.tab().gists[0])
	if lines[0][0].st.fg != m.pal.peach {
		t.Error("my name is not in my color")
	}
}

func TestClickSideTabs(t *testing.T) {
	m, _ := commented(t, 1)
	l := m.layout()
	lines := strings.Split(screen(m), "\n")
	i := strings.Index(lines[l.boxY], "comments (")
	click(t, m, ansi.StringWidth(lines[l.boxY][:i]), l.boxY)
	if m.side != sideComments {
		t.Errorf("side is %d", m.side)
	}
}

func TestStarredReadOnceWhenKept(t *testing.T) {
	src := &source{pages: onePage(manyGists(1)), starred: []model.Gist{gist("7777777", "fresh", time.Hour, "a")}}
	o := options(t, src)
	mine := cache.Open(filepath.Join(o.CacheDir, cache.MineFile), o.Host, "alice")
	if err := mine.Save(cache.List{Gists: manyGists(1)}); err != nil {
		t.Fatal(err)
	}
	kept := cache.Open(filepath.Join(o.CacheDir, cache.StarredFile), o.Host, "alice")
	if err := kept.Save(cache.List{Gists: []model.Gist{gist("8888888", "stale", time.Hour, "a")}}); err != nil {
		t.Fatal(err)
	}
	m := newModel(t, o, 120, 34)
	settle(t, m, m.Init())
	press(t, m, "2")
	if s := screen(m); !strings.Contains(s, "fresh") || strings.Contains(s, "stale") {
		t.Errorf("got:\n%s", s)
	}
}

func TestStarredPreviewNamesTheOwner(t *testing.T) {
	src := &source{pages: onePage(manyGists(1))}
	src.starred = []model.Gist{gist("7777777", "tmux", time.Hour, "tmux.conf")}
	src.starred[0].Owner = "kanata-dev"
	m := newModel(t, options(t, src), 120, 34)
	settle(t, m, m.Init())
	press(t, m, "2")
	s := screen(m)
	if !strings.Contains(s, "owner      kanata-dev") || !strings.Contains(s, "content of tmux.conf") {
		t.Errorf("got:\n%s", s)
	}
}

func TestEscapeSequencesInContentDoNotReachTheTerminal(t *testing.T) {
	bad := "a\x1b[2Jb\x1b]0;title\x07c\n"
	for _, name := range []string{"a.txt", "a.md"} {
		m, _ := preview(t, 120, 34, file(name, bad))
		out := m.render()
		for _, seq := range []string{"\x1b[2J", "\x1b]0;", "\x07"} {
			if strings.Contains(out, seq) {
				t.Errorf("%s: %q reached the screen", name, seq)
			}
		}
	}
}

func TestPreviewFitsEveryWidth(t *testing.T) {
	long := strings.Repeat("日本語の\u2764\ufe0f文章\U0001F468\u200d\U0001F469\u200d\U0001F466 ", 30) + "\n"
	for _, w := range []int{60, 80, 120, 200} {
		for _, f := range []model.File{file("a.md", sample+long), file("a.txt", long)} {
			m, _ := preview(t, w, 30, f, file("b.go", "x\n"))
			for _, keys := range [][]string{nil, {"m"}, {"c"}} {
				press(t, m, keys...)
				for i, l := range strings.Split(m.render(), "\n") {
					if got := ansi.StringWidth(l); got > w {
						t.Errorf("%d %s %v: line %d is %d wide", w, f.Name, keys, i, got)
					}
				}
			}
		}
	}
}

func TestLineEdgesStayInLine(t *testing.T) {
	// The frame's right edge shows a row a cell too wide
	long := strings.Repeat("日本語の文章 ", 40) + "\n"
	for w := 60; w <= 130; w += 7 {
		m, _ := preview(t, w, 30, file("a.md", long))
		l := m.layout()
		for i, row := range strings.Split(screen(m), "\n") {
			if i <= l.boxY || i >= l.boxY+l.boxH-1 {
				continue
			}
			at := strings.LastIndex(row, "│")
			if got := ansi.StringWidth(row[:at]); got != w-1 {
				t.Errorf("width %d: row %d has the edge at %d", w, i, got)
			}
		}
	}
}

func TestGoldenPreview(t *testing.T) {
	code := "package main\n\n// main says hi\nfunc main() {\n\tprintln(\"hi\", 42)\n}\n"
	for _, w := range []int{80, 120} {
		md, _ := preview(t, w, 34, file("a.md", sample), file("main.go", code))
		golden(t, fmt.Sprintf("preview-markdown-%d", w), screen(md))
		press(t, md, "l")
		golden(t, fmt.Sprintf("preview-code-%d", w), screen(md))
	}
	binary, _ := preview(t, 120, 34, model.File{Name: "a.png", Type: "image/png", Size: 2048, Binary: true})
	golden(t, "preview-binary", screen(binary))
	cm, _ := commented(t, 3)
	press(t, cm, "c")
	golden(t, "preview-comments", screen(cm))
	empty, _ := commented(t, 0)
	press(t, empty, "c")
	golden(t, "preview-no-comments", screen(empty))
}

func TestAGistJustChangedIsNotNowAgo(t *testing.T) {
	g := gist("1111111", "x", 0, "a.go")
	m := loaded(t, []model.Gist{g}, 120, 34)
	if s := screen(m); strings.Contains(s, "now ago") {
		t.Errorf("got:\n%s", s)
	}
}

func TestTheTabsShownSitInTheFrame(t *testing.T) {
	m := loaded(t, fixture(), 120, 34)
	backs := map[any]bool{}
	rows := m.paint().rows
	// The files are on the line under the top.
	for _, y := range []int{m.layout().boxY, m.layout().boxY + 1} {
		for _, cl := range rows[y] {
			if cl.style.bg != nil {
				backs[cl.style.bg] = true
			}
		}
	}
	if backs[m.pal.blue] {
		t.Error("a tab is solid blue")
	}
	if !backs[m.pal.surface0] {
		t.Error("the file shown has no tab of its own")
	}
	if !backs[m.pal.tab] {
		t.Error("the side shown has no tab of its own")
	}
}

func commentedOn(t *testing.T, n, w, h int) *Model {
	t.Helper()
	g := gist("1111111", "d", time.Hour, "a.go")
	g.Comments = n
	src := &source{pages: onePage([]model.Gist{g, gist("2222222", "e", 2*time.Hour, "b.go")}),
		comments: map[string][]model.Comment{g.ID: comments(n)}}
	m := newModel(t, options(t, src), w, h)
	settle(t, m, m.Init())
	_, cmd := m.Update(dwellMsg{m.preview.dwellSeq, m.preview.dwellOn})
	settle(t, m, cmd)
	return m
}

func TestCommentsUnderTheInfoOnALargeScreen(t *testing.T) {
	m := commentedOn(t, 2, 200, 60)
	if !m.stacked() {
		t.Fatal("not stacked")
	}
	s := screen(m)
	for _, want := range []string{"visibility", "comments (2)", "bob · 1h ago", "comment 0"} {
		if !strings.Contains(s, want) {
			t.Errorf("no %q in:\n%s", want, s)
		}
	}
	if x := m.paneX(); m.width-3-x > paneMaxWidth || m.width-3-x <= infoWidth {
		t.Errorf("the pane is %d wide", m.width-3-x)
	}
	x := m.paneX()
	press(t, m, "c")
	if m.paneX() != x {
		t.Errorf("c moved the pane from %d to %d", x, m.paneX())
	}
	if m.uiState() != uiListComments {
		t.Errorf("state is %s", m.uiState())
	}
}

// TestOnlyTheSideJAndKMoveHasATab is of a large screen, where the info and the
// comments are both shown: J and K move the content or the comments, and only
// the comments have a tab of the two, while c is on them.
func TestOnlyTheSideJAndKMoveHasATab(t *testing.T) {
	m := commentedOn(t, 2, 200, 60)
	tabbed := func(name string) bool {
		for _, row := range m.paint().rows {
			var line strings.Builder
			for _, cl := range row {
				if cl.style.bg == m.pal.tab {
					line.WriteString(cl.text)
				}
			}
			if strings.Contains(line.String(), name) {
				return true
			}
		}
		return false
	}
	if tabbed("info") || tabbed("comments") {
		t.Errorf("before c: info %v, comments %v", tabbed("info"), tabbed("comments"))
	}
	press(t, m, "c")
	if tabbed("info") || !tabbed("comments") {
		t.Errorf("after c: info %v, comments %v", tabbed("info"), tabbed("comments"))
	}
}

func TestSmallScreenKeepsTheCommentsInPlaceOfTheInfo(t *testing.T) {
	m := commentedOn(t, 2, 200, 30)
	if m.stacked() {
		t.Fatal("stacked in a low preview")
	}
	if s := screen(m); strings.Contains(s, "bob · 1h ago") {
		t.Errorf("comments shown with the info:\n%s", s)
	}
}

func TestWheelOverTheStackedComments(t *testing.T) {
	m := commentedOn(t, 30, 200, 60)
	g := m.tab().gists[0]
	l := m.layout()
	m.Update(wheelDown(m.paneX()+3, l.boxY+2))
	if m.preview.commentScroll[g.ID] != 0 {
		t.Error("the wheel over the info moved the comments")
	}
	m.Update(wheelDown(m.paneX()+3, m.commentsTop(l, g)+1))
	if m.preview.commentScroll[g.ID] != 1 {
		t.Errorf("scroll is %d", m.preview.commentScroll[g.ID])
	}
}
