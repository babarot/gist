package cache

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/babarot/gist/v2/internal/model"
)

var (
	t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 = t0.Add(time.Hour)
	t2 = t1.Add(time.Hour)
)

func gist(id string, updated time.Time) model.Gist {
	return model.Gist{ID: id, Updated: updated, Files: []model.File{{Name: "a.txt", Content: "content of " + id}}}
}

func ids(gists []model.Gist) string {
	s := ""
	for _, g := range gists {
		s += g.ID
	}
	return s
}

func TestMergeFirstRead(t *testing.T) {
	got := Merge(List{}, []model.ListPage{
		{Number: 1, ETag: "e1", Gists: []model.Gist{gist("a", t0), gist("b", t2)}},
		{Number: 2, ETag: "e2", Gists: []model.Gist{gist("c", t1)}},
	}, t2)
	if ids(got.Gists) != "bca" {
		t.Errorf("order is %s, want the newest update first", ids(got.Gists))
	}
	if len(got.Pages) != 2 || got.Pages[0].ETag != "e1" || len(got.Pages[0].IDs) != 2 {
		t.Errorf("pages are %+v", got.Pages)
	}
	if !got.Read.Equal(t2) {
		t.Errorf("read at %v", got.Read)
	}
	if e := got.ETags(); len(e) != 2 || e[1] != "e2" {
		t.Errorf("etags are %v", e)
	}
}

func TestMergeKeepsPagesNotModified(t *testing.T) {
	old := Merge(List{}, []model.ListPage{
		{Number: 1, ETag: "e1", Gists: []model.Gist{gist("a", t2), gist("b", t1)}},
		{Number: 2, ETag: "e2", Gists: []model.Gist{gist("c", t0)}},
	}, t0)
	got := Merge(old, []model.ListPage{
		{Number: 1, ETag: "e1", NotModified: true},
		{Number: 2, ETag: "new", Gists: []model.Gist{gist("d", t0)}},
	}, t1)
	if ids(got.Gists) != "abd" {
		t.Errorf("got %s; a page not modified keeps its gists, another replaces them", ids(got.Gists))
	}
	if got.Pages[0].ETag != "e1" || got.Pages[1].ETag != "new" || got.Pages[0].IDs[1] != "b" {
		t.Errorf("pages are %+v", got.Pages)
	}
}

func TestMergeFindsAPageNotModifiedByItsNumber(t *testing.T) {
	old := Merge(List{}, []model.ListPage{
		{Number: 1, ETag: "e1", Gists: []model.Gist{gist("a", t0)}},
		{Number: 2, ETag: "e2", Gists: []model.Gist{gist("b", t0)}},
	}, t0)
	got := Merge(old, []model.ListPage{{Number: 2, ETag: "e2", NotModified: true}}, t1)
	if ids(got.Gists) != "b" {
		t.Errorf("got %s", ids(got.Gists))
	}
}

func TestMergeTakesAGistMovedBetweenPagesOnce(t *testing.T) {
	got := Merge(List{}, []model.ListPage{
		{Number: 1, Gists: []model.Gist{gist("a", t0)}},
		{Number: 2, Gists: []model.Gist{gist("a", t0), gist("b", t0)}},
	}, t0)
	if ids(got.Gists) != "ab" {
		t.Errorf("got %s", ids(got.Gists))
	}
}

func TestStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "gists.json")
	s := Open(path, "github.com", "alice")
	if _, ok, err := s.Load(); ok || err != nil {
		t.Fatalf("no file: %v, %v", ok, err)
	}
	l := Merge(List{}, []model.ListPage{{ETag: "e", Gists: []model.Gist{gist("a", t0)}}}, t1)
	if err := s.Save(l); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.Load()
	if !ok || err != nil {
		t.Fatalf("got %v, %v", ok, err)
	}
	if ids(got.Gists) != "a" || got.Pages[0].ETag != "e" || !got.Read.Equal(t1) {
		t.Errorf("got %+v", got)
	}
	if c := got.Gists[0].Files[0].Content; c != "" {
		t.Errorf("the list file holds content: %q", c)
	}
	if l.Gists[0].Files[0].Content == "" {
		t.Error("Save changed the caller's list")
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("mode is %o", fi.Mode().Perm())
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("left behind %v", files)
	}
}

func TestStoreIsForOneAccount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gists.json")
	if err := Open(path, "github.com", "alice").Save(List{Read: t0}); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := Open(path, "github.com", "bob").Load(); ok {
		t.Error("read a list of another user")
	}
	if _, ok, _ := Open(path, "ghe.example.com", "alice").Load(); ok {
		t.Error("read a list of another host")
	}
}

func TestStoreOfAnotherVersionIsNotUsed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gists.json")
	if err := os.WriteFile(path, []byte(`{"version":999,"host":"h","viewer":"v"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := Open(path, "h", "v").Load(); ok || err != nil {
		t.Errorf("got %v, %v", ok, err)
	}
}

func TestBrokenStoreIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gists.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := Open(path, "h", "v").Load(); ok || err == nil {
		t.Errorf("got %v, %v", ok, err)
	}
}

func TestContents(t *testing.T) {
	c := OpenContents(filepath.Join(t.TempDir(), "contents"))
	if _, ok := c.Load("a", t0); ok {
		t.Error("found what was not kept")
	}
	if err := c.Save(gist("a", t0)); err != nil {
		t.Fatal(err)
	}
	g, ok := c.Load("a", t0)
	if !ok || g.Files[0].Content != "content of a" {
		t.Errorf("got %+v, %v", g, ok)
	}
	if _, ok := c.Load("a", t1); ok {
		t.Error("used what is older than the list says")
	}
	if err := c.Remove("a"); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Load("a", t0); ok {
		t.Error("still there after Remove")
	}
	if err := c.Remove("a"); err != nil {
		t.Errorf("removing what is not there: %v", err)
	}
}
