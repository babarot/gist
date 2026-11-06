package tui

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/cache"
	"github.com/babarot/gist/v2/internal/edit"
	"github.com/babarot/gist/v2/internal/fakegist"
	"github.com/babarot/gist/v2/internal/github"
	"github.com/babarot/gist/v2/internal/logging"
	"github.com/babarot/gist/v2/internal/state"
)

// world is the TUI against the made-up GitHub, with an editor that does what
// the test says.
type world struct {
	t *testing.T
	m *Model
	s *fakegist.Server
	// typed is what the editor leaves in the file.
	typed func(path string) error
	// opened counts the times the editor ran.
	opened int
	// gave is what the editor found in the file.
	gave string
	// editorErr is how the editor ends.
	editorErr error
}

func newWorld(t *testing.T, setup func(*fakegist.Server)) *world {
	t.Helper()
	s := fakegist.New(t)
	setup(s)
	c, err := github.New(logging.Discard(), github.Options{Token: "t", Transport: s.Transport()})
	if err != nil {
		t.Fatal(err)
	}
	o := options(t, c)
	o.Now = time.Now
	o.Editor = &edit.Editor{API: c, State: state.Open(t.TempDir())}
	w := &world{t: t, s: s}
	m := New(o)
	m.clockless = true
	t.Cleanup(m.cancel)
	w.m = m
	m.runEditor = func(args []string) tea.Cmd {
		w.opened++
		path := args[len(args)-1]
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		w.gave = string(b)
		if w.typed != nil {
			if err := w.typed(path); err != nil {
				t.Fatal(err)
			}
		}
		return func() tea.Msg { return editedMsg{w.editorErr} }
	}
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	settle(t, m, cmd)
	settle(t, m, m.Init())
	return w
}

// one is a world with one gist of the files.
func one(t *testing.T, files map[string]string) (*world, *fakegist.Gist) {
	t.Helper()
	var g *fakegist.Gist
	w := newWorld(t, func(s *fakegist.Server) { g = s.Add("the description", false, files) })
	return w, g
}

func (w *world) type_(content string) {
	w.typed = func(path string) error { return os.WriteFile(path, []byte(content), 0o600) }
}

func (w *world) file(id, name string) string { return w.s.Get(id).Revisions[0].Files[name] }

func (w *world) screen() string { return screen(w.m) }

func (w *world) wants(subs ...string) {
	w.t.Helper()
	s := w.screen()
	for _, want := range subs {
		if !strings.Contains(s, want) {
			w.t.Errorf("no %q in:\n%s", want, s)
		}
	}
}

func (w *world) draft(id, file string) (string, bool) {
	d, ok, _ := w.m.opt.Editor.Draft(id, file)
	return string(d.Content), ok
}

func enter(t *testing.T, m *Model) {
	t.Helper()
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	settle(t, m, cmd)
}

func TestEditPushes(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "old\n"})
	w.type_("new\n")
	enter(t, w.m)
	if w.m.uiState() != uiConfirmPush {
		t.Fatalf("state is %s:\n%s", w.m.uiState(), w.screen())
	}
	if w.gave != "old\n" {
		t.Errorf("the editor got %q", w.gave)
	}
	w.wants("push a.txt to "+g.ID[:7], "+1", "−1", "- old", "+ new", "push this change to GitHub?")
	if d, ok := w.draft(g.ID, "a.txt"); !ok || d != "new\n" {
		t.Errorf("no draft before sending: %q", d)
	}
	press(t, w.m, "y")
	if got := w.file(g.ID, "a.txt"); got != "new\n" {
		t.Errorf("got %q", got)
	}
	w.wants("✓ pushed a.txt (+1 −1)", "new")
	if _, ok := w.draft(g.ID, "a.txt"); ok {
		t.Error("the draft stayed")
	}
	if w.m.uiState() != uiList {
		t.Errorf("state is %s", w.m.uiState())
	}
}

func TestEditKeepsTheBytes(t *testing.T) {
	const raw = "col\tumn\r\n\x1b[31mred\x1b[0m\n"
	w, g := one(t, map[string]string{"a.txt": raw, "b.txt": "b\n"})
	w.typed = func(path string) error {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(path, append(b, "more\n"...), 0o600)
	}
	enter(t, w.m)
	if w.gave != raw {
		t.Errorf("the editor got %q", w.gave)
	}
	press(t, w.m, "y")
	if got := w.file(g.ID, "a.txt"); got != raw+"more\n" {
		t.Errorf("sent %q", got)
	}
}

func TestEditWithNoChangeSendsNothing(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	enter(t, w.m)
	w.wants("no changes · nothing pushed")
	if len(w.s.Get(g.ID).Revisions) != 1 {
		t.Error("made a revision")
	}
	if w.m.uiState() != uiList {
		t.Errorf("state is %s", w.m.uiState())
	}
}

func TestEscKeepsADraftAndItContinues(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("edited\n")
	enter(t, w.m)
	press(t, w.m, "esc")
	w.wants(draftMark, "draft · enter to continue", "kept as a draft", "X discard draft")
	if d, ok := w.draft(g.ID, "a.txt"); !ok || d != "edited\n" {
		t.Errorf("got %q, %v", d, ok)
	}
	w.typed = nil
	enter(t, w.m)
	if w.gave != "edited\n" {
		t.Errorf("the editor got %q", w.gave)
	}
	// Opened and closed unchanged, it still differs from what it started from
	if w.m.uiState() != uiConfirmPush {
		t.Errorf("state is %s", w.m.uiState())
	}
	press(t, w.m, "y")
	if got := w.file(g.ID, "a.txt"); got != "edited\n" {
		t.Errorf("got %q", got)
	}
}

func TestEditAgainFromTheDiff(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("one\n")
	enter(t, w.m)
	w.type_("two\n")
	press(t, w.m, "e")
	if w.gave != "one\n" {
		t.Errorf("the editor got %q", w.gave)
	}
	w.wants("+ two")
	press(t, w.m, "y")
	if got := w.file(g.ID, "a.txt"); got != "two\n" {
		t.Errorf("got %q", got)
	}
}

func TestEnterDoesNotReopenTheEditorFromTheDiff(t *testing.T) {
	w, _ := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("edited\n")
	enter(t, w.m)
	enter(t, w.m)
	if w.opened != 1 {
		t.Errorf("opened %d times", w.opened)
	}
	w.wants("e edit again")
}

func TestAnEditorThatFailsLosesNothing(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("edited\n")
	w.editorErr = errors.New("the editor exploded")
	enter(t, w.m)
	if w.m.uiState() != uiConfirmPush {
		t.Fatalf("state is %s", w.m.uiState())
	}
	if d, ok := w.draft(g.ID, "a.txt"); !ok || d != "edited\n" {
		t.Errorf("got %q, %v", d, ok)
	}
}

func TestAnEditorThatDidNotRunSaysSo(t *testing.T) {
	w, _ := one(t, map[string]string{"a.txt": "a\n"})
	w.editorErr = errors.New("no such editor")
	enter(t, w.m)
	w.wants("the editor failed: no such editor")
	if s := w.screen(); strings.Contains(s, "no changes") {
		t.Errorf("got:\n%s", s)
	}
}

func TestAFailedPushKeepsTheDraft(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("edited\n")
	enter(t, w.m)
	w.s.Fail("PATCH", http.StatusInternalServerError)
	press(t, w.m, "y")
	w.wants("refused by the test", draftMark)
	if d, ok := w.draft(g.ID, "a.txt"); !ok || d != "edited\n" {
		t.Errorf("got %q, %v", d, ok)
	}
	if w.m.uiState() != uiList {
		t.Errorf("state is %s", w.m.uiState())
	}
}

func TestAnEmptiedFileIsNotRemoved(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	w.type_("")
	enter(t, w.m)
	press(t, w.m, "y")
	w.wants("GitHub removes an empty file: D then f removes it")
	if _, ok := w.s.Get(g.ID).Revisions[0].Files["a.txt"]; !ok {
		t.Error("the file was removed")
	}
	if _, ok := w.draft(g.ID, "a.txt"); !ok {
		t.Error("the edit is not in a draft")
	}
}

func TestConflictAsked(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("mine\n")
	enter(t, w.m)
	w.s.Edit(g.ID, "a.txt", "theirs\n")
	press(t, w.m, "y")
	if w.m.uiState() != uiConfirmConflict {
		t.Fatalf("state is %s", w.m.uiState())
	}
	w.wants("GitHub side was updated", "o overwrites", "r reads the new revision")
	if got := w.file(g.ID, "a.txt"); got != "theirs\n" {
		t.Errorf("sent over it: %q", got)
	}
}

func TestConflictOverwrite(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("mine\n")
	enter(t, w.m)
	w.s.Edit(g.ID, "a.txt", "theirs\n")
	press(t, w.m, "y", "o")
	if got := w.file(g.ID, "a.txt"); got != "mine\n" {
		t.Errorf("got %q", got)
	}
	if w.m.uiState() != uiList {
		t.Errorf("state is %s", w.m.uiState())
	}
	if _, ok := w.draft(g.ID, "a.txt"); ok {
		t.Error("the draft stayed")
	}
}

func TestConflictReload(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("mine\n")
	enter(t, w.m)
	w.s.Edit(g.ID, "a.txt", "theirs\n")
	press(t, w.m, "y", "r")
	if w.m.uiState() != uiConfirmPush {
		t.Fatalf("state is %s", w.m.uiState())
	}
	w.wants("- theirs", "+ mine")
	press(t, w.m, "y")
	if got := w.file(g.ID, "a.txt"); got != "mine\n" {
		t.Errorf("got %q", got)
	}
}

func TestConflictReloadOntoTheSameChange(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("same\n")
	enter(t, w.m)
	w.s.Edit(g.ID, "a.txt", "same\n")
	press(t, w.m, "y", "r")
	w.wants("no changes · nothing pushed")
	if _, ok := w.draft(g.ID, "a.txt"); ok {
		t.Error("the draft stayed")
	}
}

func TestConflictKeepAsDraft(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("mine\n")
	enter(t, w.m)
	w.s.Edit(g.ID, "a.txt", "theirs\n")
	press(t, w.m, "y", "esc")
	if d, ok := w.draft(g.ID, "a.txt"); !ok || d != "mine\n" {
		t.Errorf("got %q, %v", d, ok)
	}
}

func TestOverwriteFoundAfterThePush(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("mine\n")
	enter(t, w.m)
	w.s.Before("PATCH /gists/", func() { w.s.Edit(g.ID, "a.txt", "theirs\n") })
	press(t, w.m, "y")
	w.wants("overwrote ", w.s.Get(g.ID).Revisions[1].Version[:12])
}

func TestAPushMovesTheGistToTheTopWithTheCursor(t *testing.T) {
	var older *fakegist.Gist
	w := newWorld(t, func(s *fakegist.Server) {
		older = s.Add("older", false, map[string]string{"a.txt": "a\n"})
		s.Add("newer", false, map[string]string{"b.txt": "b\n"})
	})
	press(t, w.m, "j")
	if g, _ := w.m.selected(); g.ID != older.ID {
		t.Fatalf("on %s", g.Description)
	}
	w.type_("edited\n")
	enter(t, w.m)
	press(t, w.m, "y")
	if g, _ := w.m.selected(); g.ID != older.ID {
		t.Errorf("the cursor stayed behind on %s", g.Description)
	}
	if w.m.tab().cur != 0 {
		t.Errorf("cursor at %d", w.m.tab().cur)
	}
	w.wants("edited")
}

func TestLockTakenElsewhereRefuses(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	l, err := w.m.opt.Editor.State.Lock(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Release() }()
	enter(t, w.m)
	w.wants("being edited in another gist process")
	if w.opened != 0 {
		t.Error("the editor opened")
	}
}

func TestTheLockIsLetGoAfterAnEdit(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	enter(t, w.m)
	l, err := w.m.opt.Editor.State.Lock(g.ID)
	if err != nil {
		t.Fatalf("the lock stayed: %v", err)
	}
	_ = l.Release()
}

func TestDraftsShowOnTheNextLaunch(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("edited\n")
	enter(t, w.m)
	press(t, w.m, "esc")
	m := New(w.m.opt)
	m.clockless = true
	t.Cleanup(m.cancel)
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	settle(t, m, cmd)
	settle(t, m, m.Init())
	if s := screen(m); !strings.Contains(s, draftMark) {
		t.Errorf("got:\n%s", s)
	}
	_ = g
}

func TestDiscardADraft(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("edited\n")
	enter(t, w.m)
	press(t, w.m, "esc", "X")
	if w.m.uiState() != uiConfirmDiscard {
		t.Fatalf("state is %s", w.m.uiState())
	}
	w.wants("discard the draft", "a.txt")
	press(t, w.m, "esc")
	if _, ok := w.draft(g.ID, "a.txt"); !ok {
		t.Fatal("esc discarded it")
	}
	press(t, w.m, "X", "y")
	if _, ok := w.draft(g.ID, "a.txt"); ok {
		t.Error("the draft stayed")
	}
	if s := w.screen(); strings.Contains(s, draftMark) {
		t.Errorf("got:\n%s", s)
	}
}

func TestDiscardIsRefusedWhileLockedElsewhere(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("edited\n")
	enter(t, w.m)
	press(t, w.m, "esc", "X")
	l, err := w.m.opt.Editor.State.Lock(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Release() }()
	press(t, w.m, "y")
	if _, ok := w.draft(g.ID, "a.txt"); !ok {
		t.Error("the draft was removed under another's lock")
	}
	w.wants("being edited in another gist process")
}

func TestStarredIsReadOnly(t *testing.T) {
	var star *fakegist.Gist
	w := newWorld(t, func(s *fakegist.Server) {
		s.Add("mine", false, map[string]string{"a.txt": "a\n"})
		star = s.Star("kanata", "theirs", map[string]string{"b.txt": "b\n"})
	})
	press(t, w.m, "2")
	for _, k := range []string{"D", "E", "r"} {
		press(t, w.m, k)
		w.wants("read only: kanata's gist · o opens it in the browser")
		if w.m.uiState() != uiList {
			t.Errorf("%s: state is %s", k, w.m.uiState())
		}
	}
	enter(t, w.m)
	if w.opened != 0 {
		t.Error("the editor opened")
	}
	if s := w.screen(); strings.Contains(s, "enter edit") {
		t.Errorf("got:\n%s", s)
	}
	_ = star
}

func TestDelete(t *testing.T) {
	w := newWorld(t, func(s *fakegist.Server) {
		s.Add("keep", false, map[string]string{"k.txt": "k\n"})
		s.Add("go", false, map[string]string{"a.txt": "a\n"})
	})
	g, _ := w.m.selected()
	press(t, w.m, "D")
	if w.m.uiState() != uiConfirmDelete {
		t.Fatalf("state is %s", w.m.uiState())
	}
	w.wants("delete gist", "This deletes the gist and its 1 revision on GitHub.", "It cannot be undone.")
	press(t, w.m, "esc")
	if w.s.Get(g.ID) == nil {
		t.Fatal("esc deleted it")
	}
	press(t, w.m, "D", "y")
	if w.s.Get(g.ID) != nil {
		t.Error("not deleted")
	}
	w.wants("✓ deleted " + g.ID[:7])
	if s := w.screen(); strings.Contains(s, "\tgo") || !strings.Contains(s, "keep") {
		t.Errorf("got:\n%s", s)
	}
	if got := len(w.m.tab().gists); got != 1 {
		t.Errorf("%d in the list", got)
	}
	s, ok, err := cache.Open(filepath.Join(w.m.opt.CacheDir, cache.MineFile), "github.com", "alice").Load()
	if err != nil || !ok || len(s.Gists) != 1 {
		t.Errorf("the kept list has %d: %v, %v", len(s.Gists), ok, err)
	}
}

func TestDeleteFailureSaysSo(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.s.Fail("DELETE", http.StatusInternalServerError)
	press(t, w.m, "D", "y")
	w.wants("refused by the test")
	if len(w.m.tab().gists) != 1 || w.s.Get(g.ID) == nil {
		t.Error("the gist went")
	}
}

func TestDeleteOneFile(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n", "c.txt": "c\n"})
	press(t, w.m, "l", "D")
	w.wants("f removes only b.txt. The other 2 files stay.", "Older revisions keep this file.")
	press(t, w.m, "f")
	files := w.s.Get(g.ID).Revisions[0].Files
	if _, ok := files["b.txt"]; ok || len(files) != 2 {
		t.Errorf("files are %q", files)
	}
	w.wants("✓ removed b.txt")
}

func TestOnlyFileKeyIsNotForALoneFile(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	press(t, w.m, "D", "f")
	if w.s.Get(g.ID) == nil || w.m.uiState() != uiConfirmDelete {
		t.Errorf("state is %s", w.m.uiState())
	}
}

func typeText(t *testing.T, m *Model, s string) {
	t.Helper()
	for _, r := range s {
		_, cmd := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		settle(t, m, cmd)
	}
}

func clearInput(t *testing.T, m *Model) {
	t.Helper()
	for range len([]rune(m.input.box.Value())) {
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
		settle(t, m, cmd)
	}
}

func TestChangeTheDescription(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	press(t, w.m, "E")
	if w.m.uiState() != uiInput {
		t.Fatalf("state is %s", w.m.uiState())
	}
	w.wants("description of "+g.ID[:7], "the description")
	clearInput(t, w.m)
	typeText(t, w.m, "日本語の説明")
	enter(t, w.m)
	if got := w.s.Get(g.ID).Description; got != "日本語の説明" {
		t.Errorf("got %q", got)
	}
	w.wants("✓ description changed", "日本語の説明")
}

func TestTheInputUsesTheTerminalsCursor(t *testing.T) {
	w, _ := one(t, map[string]string{"a.txt": "a\n"})
	press(t, w.m, "E")
	cur := w.m.cursor()
	if cur == nil {
		t.Fatal("no cursor")
	}
	x, y, _, _ := w.m.inputBox()
	if cur.Y != y+inputRow || cur.X != x+5+len("the description") {
		t.Errorf("at %v", cur.Position)
	}
	press(t, w.m, "esc")
	if w.m.cursor() != nil {
		t.Error("a cursor with no box")
	}
}

func TestSameDescriptionSendsNothing(t *testing.T) {
	w, _ := one(t, map[string]string{"a.txt": "a\n"})
	press(t, w.m, "E")
	enter(t, w.m)
	if w.m.uiState() != uiList {
		t.Errorf("state is %s", w.m.uiState())
	}
	for _, r := range w.s.Requests() {
		if strings.HasPrefix(r, "PATCH") {
			t.Errorf("sent %s", r)
		}
	}
}

func TestDescriptionConflict(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	press(t, w.m, "E")
	typeText(t, w.m, " mine")
	w.s.SetDescription(g.ID, "theirs")
	enter(t, w.m)
	if w.m.uiState() != uiInputConflict {
		t.Fatalf("state is %s:\n%s", w.m.uiState(), w.screen())
	}
	w.wants("the description on GitHub is now:", "theirs", "overwriting loses this one")
	if got := w.s.Get(g.ID).Description; got != "theirs" {
		t.Errorf("got %q", got)
	}
	press(t, w.m, "o")
	if got := w.s.Get(g.ID).Description; got != "the description mine" {
		t.Errorf("got %q", got)
	}
}

func TestDescriptionConflictReload(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	press(t, w.m, "E")
	typeText(t, w.m, " mine")
	w.s.SetDescription(g.ID, "theirs")
	enter(t, w.m)
	press(t, w.m, "r")
	if w.m.uiState() != uiInput || w.m.input.box.Value() != "the description mine" {
		t.Fatalf("state is %s, value %q", w.m.uiState(), w.m.input.box.Value())
	}
	typeText(t, w.m, " too")
	enter(t, w.m)
	if got := w.s.Get(g.ID).Description; got != "the description mine too" {
		t.Errorf("got %q", got)
	}
}

func TestRename(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	press(t, w.m, "r")
	w.wants("rename a.txt", "a rename makes a new revision")
	clearInput(t, w.m)
	typeText(t, w.m, "c.txt")
	enter(t, w.m)
	files := w.s.Get(g.ID).Revisions[0].Files
	if _, ok := files["c.txt"]; !ok || len(files) != 2 {
		t.Errorf("files are %q", files)
	}
	w.wants("✓ renamed to c.txt", " c.txt ")
}

func TestRenameOntoATakenName(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	press(t, w.m, "r")
	clearInput(t, w.m)
	typeText(t, w.m, "b.txt")
	enter(t, w.m)
	w.wants("this gist already has b.txt")
	if len(w.s.Get(g.ID).Revisions) != 1 {
		t.Error("sent it")
	}
}

func TestRenameConflict(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	press(t, w.m, "r")
	clearInput(t, w.m)
	typeText(t, w.m, "c.txt")
	w.s.Edit(g.ID, "a.txt", "theirs\n")
	enter(t, w.m)
	if w.m.uiState() != uiInputConflict {
		t.Fatalf("state is %s", w.m.uiState())
	}
	press(t, w.m, "o")
	if _, ok := w.s.Get(g.ID).Revisions[0].Files["c.txt"]; !ok {
		t.Error("not renamed")
	}
}

func TestKeysDoNotReachTheListFromABox(t *testing.T) {
	var more *fakegist.Gist
	w := newWorld(t, func(s *fakegist.Server) {
		more = s.Add("first", false, map[string]string{"a.txt": "a\n"})
		s.Add("second", false, map[string]string{"b.txt": "b\n"})
	})
	press(t, w.m, "D", "j", "2", "E")
	if w.m.tab().cur != 0 || w.m.cur != tabMine || w.m.uiState() != uiConfirmDelete {
		t.Errorf("cursor %d, tab %d, %s", w.m.tab().cur, w.m.cur, w.m.uiState())
	}
	_ = more
}

func TestQAndSecondKeysAreTextInTheInput(t *testing.T) {
	w, _ := one(t, map[string]string{"a.txt": "a\n"})
	press(t, w.m, "E")
	press(t, w.m, "q", "j", "?")
	if w.m.uiState() != uiInput || !strings.HasSuffix(w.m.input.box.Value(), "qj?") {
		t.Errorf("state %s, value %q", w.m.uiState(), w.m.input.box.Value())
	}
}

func TestDiffScrolls(t *testing.T) {
	var lines []string
	for range 200 {
		lines = append(lines, "x")
	}
	w, _ := one(t, map[string]string{"a.txt": "a\n"})
	w.type_(strings.Join(lines, "\n") + "\n")
	enter(t, w.m)
	for range 300 {
		press(t, w.m, "J")
	}
	if got, limit := w.m.editing.scroll, len(w.m.editing.diff.Lines)-w.m.diffRows(); got != limit {
		t.Errorf("scrolled to %d, want %d", got, limit)
	}
}

func TestDiffPages(t *testing.T) {
	w, _ := one(t, map[string]string{"a.txt": "a\n"})
	w.type_(strings.Repeat("x\n", 200))
	enter(t, w.m)
	page(t, w.m, "ctrl+d")
	if got := w.m.editing.scroll; got != w.m.diffRows() {
		t.Errorf("at %d", got)
	}
	page(t, w.m, "pgup")
	if got := w.m.editing.scroll; got != 0 {
		t.Errorf("at %d", got)
	}
}

func TestDoubleClickEdits(t *testing.T) {
	w, _ := one(t, map[string]string{"a.txt": "a\n"})
	y := w.m.layout().listY
	click(t, w.m, 20, y)
	click(t, w.m, 20, y)
	if w.opened != 1 {
		t.Errorf("opened %d times", w.opened)
	}
}

// hold presses a key and keeps what it starts from running, for keys that come
// while it is on its way.
func hold(m *Model, k tea.KeyPressMsg) tea.Cmd {
	_, cmd := m.Update(k)
	return cmd
}

func TestKeysWaitForAnEditToBegin(t *testing.T) {
	var a *fakegist.Gist
	w := newWorld(t, func(s *fakegist.Server) {
		a = s.Add("a", false, map[string]string{"a.txt": "a\n"})
		s.Add("b", false, map[string]string{"b.txt": "b\n"})
	})
	began := hold(w.m, tea.KeyPressMsg{Code: tea.KeyEnter})
	press(t, w.m, "j")
	if second := hold(w.m, tea.KeyPressMsg{Code: tea.KeyEnter}); second != nil {
		t.Error("a second edit began")
	}
	settle(t, w.m, began)
	if w.opened != 1 {
		t.Errorf("opened %d times", w.opened)
	}
	if l, err := w.m.opt.Editor.State.Lock(a.ID); err == nil {
		_ = l.Release()
	} else {
		t.Errorf("a lock stayed: %v", err)
	}
}

func TestKeysWaitForTheEditorsFilesToBeRead(t *testing.T) {
	for _, k := range []string{"esc", "e", "y", "q"} {
		t.Run(k, func(t *testing.T) {
			w, g := one(t, map[string]string{"a.txt": "a\n"})
			w.type_("edited\n")
			// The editor has closed, and word of that has not reached the model
			w.m.runEditor = func(args []string) tea.Cmd {
				w.opened++
				if err := os.WriteFile(args[len(args)-1], []byte("edited\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				return nil
			}
			enter(t, w.m)
			_, cmd := w.m.Update(keyMsg(k))
			if isQuit(cmd) {
				t.Fatal("quit with the edit not read back")
			}
			settle(t, w.m, cmd)
			_, cmd = w.m.Update(editedMsg{})
			settle(t, w.m, cmd)
			if d, ok := w.draft(g.ID, "a.txt"); !ok || d != "edited\n" {
				t.Errorf("the edit is not in a draft: %q, %v", d, ok)
			}
			if w.opened != 1 {
				t.Errorf("opened %d times", w.opened)
			}
			if w.m.uiState() != uiConfirmPush {
				t.Errorf("state is %s", w.m.uiState())
			}
		})
	}
}

func TestQuittingBeforeTheFilesAreReadKeepsTheEdit(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.m.runEditor = func(args []string) tea.Cmd {
		if err := os.WriteFile(args[len(args)-1], []byte("edited\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return nil
	}
	enter(t, w.m)
	_, cmd := w.m.Update(keyMsg("ctrl+c"))
	if !isQuit(cmd) {
		t.Fatal("did not quit")
	}
	if d, ok := w.draft(g.ID, "a.txt"); !ok || d != "edited\n" {
		t.Errorf("the edit is not in a draft: %q, %v", d, ok)
	}
	if w.m.quitErr != nil {
		t.Errorf("got %v", w.m.quitErr)
	}
}

func TestKeysWaitForAPush(t *testing.T) {
	for _, k := range []string{"esc", "e", "y", "J"} {
		t.Run(k, func(t *testing.T) {
			w, g := one(t, map[string]string{"a.txt": "a\n"})
			w.type_("edited\n")
			enter(t, w.m)
			pushing := hold(w.m, keyMsg("y"))
			press(t, w.m, k)
			settle(t, w.m, pushing)
			if got := len(w.s.Get(g.ID).Revisions); got != 2 {
				t.Errorf("%d revisions", got)
			}
			if w.opened != 1 {
				t.Errorf("opened %d times", w.opened)
			}
			if len(w.m.writing) != 0 {
				t.Errorf("still writing: %v", w.m.writing)
			}
			if w.m.uiState() != uiList {
				t.Errorf("state is %s", w.m.uiState())
			}
			enter(t, w.m)
			if w.opened != 2 {
				t.Error("the gist cannot be edited again")
			}
		})
	}
}

func TestKeysWaitForAnInputToBeSent(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	press(t, w.m, "r")
	clearInput(t, w.m)
	typeText(t, w.m, "c.txt")
	sending := hold(w.m, tea.KeyPressMsg{Code: tea.KeyEnter})
	press(t, w.m, "esc", "x")
	settle(t, w.m, sending)
	if _, ok := w.s.Get(g.ID).Revisions[0].Files["c.txt"]; !ok {
		t.Error("not renamed")
	}
	if len(w.m.writing) != 0 {
		t.Errorf("still writing: %v", w.m.writing)
	}
	_ = screen(w.m)
}

func TestAResultGoesToTheListsThatHaveTheGist(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	press(t, w.m, "D")
	removing := hold(w.m, keyMsg("f"))
	press(t, w.m, "2")
	settle(t, w.m, removing)
	if got := len(w.m.tabs[tabStarred].gists); got != 0 {
		t.Errorf("my gist went into the starred list: %d", got)
	}
	if got := len(w.m.tabs[tabMine].gists[0].Files); got != 1 {
		t.Errorf("%d files in my list", got)
	}
	_ = g
}

func TestDeletingAGistTakesItsDrafts(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("edited\n")
	enter(t, w.m)
	press(t, w.m, "esc", "D", "y")
	if _, ok := w.draft(g.ID, "a.txt"); ok {
		t.Error("the draft stayed")
	}
	if len(w.m.drafts) != 0 {
		t.Errorf("drafts are %v", w.m.drafts)
	}
}

func TestDraftsOfAGistGoneAreToldOf(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("edited\n")
	enter(t, w.m)
	press(t, w.m, "esc")
	w.s.Remove(g.ID)
	m := New(w.m.opt)
	m.clockless = true
	t.Cleanup(m.cancel)
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	settle(t, m, cmd)
	settle(t, m, m.Init())
	if s := screen(m); !strings.Contains(s, "drafts of 1 gist no longer on GitHub are kept in") {
		t.Errorf("got:\n%s", s)
	}
	if _, ok := w.draft(g.ID, "a.txt"); !ok {
		t.Error("the draft went")
	}
}

func TestQuittingAtTheDiffKeepsTheDraft(t *testing.T) {
	w, g := one(t, map[string]string{"a.txt": "a\n"})
	w.type_("edited\n")
	enter(t, w.m)
	_, cmd := w.m.Update(keyMsg("ctrl+c"))
	if !isQuit(cmd) {
		t.Fatal("did not quit")
	}
	if d, ok := w.draft(g.ID, "a.txt"); !ok || d != "edited\n" {
		t.Errorf("got %q, %v", d, ok)
	}
}

func TestEveryBoxFitsTheTerminal(t *testing.T) {
	boxes := map[string][]string{"diff": nil, "delete": {"D"}, "description": {"E"}}
	for name, keys := range boxes {
		w, _ := one(t, map[string]string{"a.txt": "日本語\n", "b.txt": "b\n"})
		for _, size := range [][2]int{{60, 24}, {80, 24}, {120, 34}, {200, 50}, {30, 10}, {10, 4}} {
			w.m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			if name == "diff" && w.m.editing == nil {
				w.type_("編集した\n")
				enter(t, w.m)
			}
			press(t, w.m, keys...)
			keys = nil
			lines := strings.Split(screen(w.m), "\n")
			if len(lines) != size[1] {
				t.Errorf("%s %v: %d lines", name, size, len(lines))
			}
		}
	}
}
