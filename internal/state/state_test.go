package state

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
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
	if fi.Mode().Perm() != 0o600 {
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
