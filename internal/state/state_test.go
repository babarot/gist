package state

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLockKeepsOthersOut(t *testing.T) {
	s := Open(t.TempDir())
	l, err := s.Lock("a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Lock("a"); !errors.Is(err, ErrLocked) {
		t.Errorf("a second lock: %v", err)
	}
	other, err := s.Lock("b")
	if err != nil {
		t.Fatalf("another gist's lock: %v", err)
	}
	if err := other.Release(); err != nil {
		t.Fatal(err)
	}
	if err := l.Release(); err != nil {
		t.Fatal(err)
	}
	again, err := s.Lock("a")
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	if err := again.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestDraftKeepsEveryByte(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	l, err := s.Lock("a")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Release() }()
	content := []byte("tab\t cr\r esc\x1b invalid\xff ❤️\n")
	d := Draft{Gist: "a", File: "dir/odd name.md", Revision: "r1", Original: []byte("old\xfe"), Content: content,
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	if err := l.SaveDraft(d); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.Draft("a", d.File)
	if !ok || err != nil {
		t.Fatalf("got %v, %v", ok, err)
	}
	if !bytes.Equal(got.Content, content) || !bytes.Equal(got.Original, d.Original) || got.Revision != "r1" {
		t.Errorf("got %+v", got)
	}
	drafts, err := s.Drafts("a")
	if err != nil || len(drafts) != 1 {
		t.Fatalf("got %v, %v", drafts, err)
	}
	if err := l.DeleteDraft(d.File); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.Draft("a", d.File); ok {
		t.Error("the draft is still there")
	}
	if err := l.DeleteDraft(d.File); err != nil {
		t.Errorf("deleting what is not there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "drafts", "a")); !os.IsNotExist(err) {
		t.Errorf("the gist's directory stayed: %v", err)
	}
}

func TestDraftIsOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	l, err := s.Lock("a")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Release() }()
	if err := l.SaveDraft(Draft{Gist: "a", File: "f"}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, "drafts", "a", "f.json"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("mode is %o", fi.Mode().Perm())
	}
}

func TestLockIsForItsOwnGist(t *testing.T) {
	s := Open(t.TempDir())
	l, err := s.Lock("a")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Release() }()
	if err := l.SaveDraft(Draft{Gist: "b", File: "f"}); err == nil {
		t.Error("saved a draft of another gist")
	}
}

func TestNoDrafts(t *testing.T) {
	s := Open(t.TempDir())
	if d, err := s.Drafts("a"); d != nil || err != nil {
		t.Errorf("got %v, %v", d, err)
	}
	if _, ok, err := s.Draft("a", "f"); ok || err != nil {
		t.Errorf("got %v, %v", ok, err)
	}
}

func TestPreviewHeight(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	s := Open(dir)
	if got := s.PreviewHeight(); got != 0 {
		t.Errorf("nothing saved: %d", got)
	}
	if err := s.SavePreviewHeight(12); err != nil {
		t.Fatal(err)
	}
	if got := Open(dir).PreviewHeight(); got != 12 {
		t.Errorf("got %d", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := s.PreviewHeight(); got != 0 {
		t.Errorf("a broken file: %d", got)
	}
}

func TestDraftGists(t *testing.T) {
	s := Open(t.TempDir())
	if got, err := s.DraftGists(); got != nil || err != nil {
		t.Errorf("got %v, %v", got, err)
	}
	l, err := s.Lock("a")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Release() }()
	if err := l.SaveDraft(Draft{Gist: "a", File: "f"}); err != nil {
		t.Fatal(err)
	}
	if got, err := s.DraftGists(); err != nil || len(got) != 1 || got[0] != "a" {
		t.Errorf("got %v, %v", got, err)
	}
	if err := l.DeleteDraft("f"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.DraftGists(); err != nil || len(got) != 0 {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestDraftsOfNamesThatDifferOnlyInCase(t *testing.T) {
	s := Open(t.TempDir())
	l, err := s.Lock("a")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Release() }()
	for _, f := range []string{"A.go", "a.go", "%41.go"} {
		if err := l.SaveDraft(Draft{Gist: "a", File: f, Content: []byte(f)}); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"A.go", "a.go", "%41.go"} {
		if d, ok, err := s.Draft("a", f); err != nil || !ok || string(d.Content) != f {
			t.Errorf("%s: %+v, %v, %v", f, d, ok, err)
		}
	}
	if err := l.DeleteDraft("a.go"); err != nil {
		t.Fatal(err)
	}
	if !s.HasDraft("a", "A.go") || s.HasDraft("a", "a.go") {
		t.Error("the other one went")
	}
}

func TestDraftsKeptUnderTheOldNamesAreFound(t *testing.T) {
	dir := t.TempDir()
	draftDir := filepath.Join(dir, "drafts", "a")
	if err := os.MkdirAll(draftDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(draftDir, "Main.go.json"), []byte(`{"gist":"a","file":"Main.go","content":"a2VwdA=="}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(draftDir, "broken.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := Open(dir)
	if d, ok, err := s.Draft("a", "Main.go"); err != nil || !ok || string(d.Content) != "kept" {
		t.Errorf("got %+v, %v, %v", d, ok, err)
	}
	if _, err := os.Stat(filepath.Join(draftDir, "broken.json")); err != nil {
		t.Errorf("a draft that cannot be read went: %v", err)
	}
}
