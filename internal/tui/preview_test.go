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
	if !strings.Contains(line, "…") || !strings.Contains(line, "│ gist") {
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
	m.tabs[tabMine].gists, m.tabs[tabMine].loaded = []model.Gist{g}, true
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
	m.tabs[tabMine].gists, m.tabs[tabMine].loaded = manyGists(20), true
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

func TestScrollStopsAtTheEnds(t *testing.T) {
	m, _ := preview(t, 120, 34, file("a.txt", strings.Repeat("line\n", 40)))
	inner := m.innerRows()
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

func TestCommentsPaneIsWider(t *testing.T) {
	m, _ := commented(t, 1)
	info := m.paneX()
	press(t, m, "c")
	if m.paneX() >= info {
		t.Errorf("the pane went from %d to %d", info, m.paneX())
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
	if !strings.Contains(screen(m), "↓") {
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
