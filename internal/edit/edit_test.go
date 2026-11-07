package edit

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/babarot/gist/v2/internal/fakegist"
	"github.com/babarot/gist/v2/internal/github"
	"github.com/babarot/gist/v2/internal/logging"
	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/state"
)

var ctx = context.Background()

// raw is a file only the raw content keeps as written.
const raw = "col\tumn\r\n\x1b[31mred\x1b[0m\n❤️\n"

func setup(t *testing.T) (*Editor, *fakegist.Server) {
	t.Helper()
	s := fakegist.New(t)
	c, err := github.New(logging.Discard(), github.Options{Token: "t", Transport: s.Transport()})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	stateDirs[t.Name()] = dir
	return &Editor{API: c, State: state.Open(dir)}, s
}

// stateDirs are the state directories of the tests.
var stateDirs = map[string]string{}

func stateDirOf(t *testing.T, _ *Editor) string { return stateDirs[t.Name()] }

func begin(t *testing.T, e *Editor, id string, files ...string) *Session {
	t.Helper()
	s, err := e.Begin(ctx, id, files...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func keep(t *testing.T, s *Session, content string) {
	t.Helper()
	if changed, err := s.Keep(s.Files[0].Name, content); err != nil || !changed {
		t.Fatalf("changed %v, %v", changed, err)
	}
}

func TestEditStartsFromTheRawContent(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": raw})
	es := begin(t, e, g.ID, "a.txt")
	f := es.File("a.txt")
	if f.Content != raw || f.Original != raw || f.FromDraft {
		t.Errorf("got %+v", f)
	}
	if f.Revision != g.Revisions[0].Version {
		t.Errorf("revision is %q", f.Revision)
	}
}

func TestPushSendsTheBytesAndDropsTheDraft(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "old"})
	es := begin(t, e, g.ID, "a.txt")
	keep(t, es, raw)
	res, err := es.Push(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Sent || res.Overwritten != "" {
		t.Errorf("got %+v", res)
	}
	if got := s.Get(g.ID).Revisions[0].Files["a.txt"]; got != raw {
		t.Errorf("sent %q", got)
	}
	if _, ok, _ := e.Draft(g.ID, "a.txt"); ok {
		t.Error("the draft stayed")
	}
}

func TestNoChangeLeavesNoDraft(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "x"})
	es := begin(t, e, g.ID, "a.txt")
	keep(t, es, "y")
	if changed, err := es.Keep("a.txt", "x"); err != nil || changed {
		t.Errorf("changed %v, %v", changed, err)
	}
	if _, ok, _ := e.Draft(g.ID, "a.txt"); ok {
		t.Error("a draft is left for no change")
	}
}

func TestDraftSurvivesAFailedPush(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "x"})
	es := begin(t, e, g.ID, "a.txt")
	keep(t, es, "edited")
	s.Fail("PATCH", http.StatusInternalServerError)
	if _, err := es.Push(ctx, false); err == nil {
		t.Fatal("no error")
	}
	d, ok, err := e.Draft(g.ID, "a.txt")
	if !ok || err != nil || string(d.Content) != "edited" {
		t.Errorf("got %+v, %v, %v", d, ok, err)
	}
}

func TestEmptyContentIsRefusedAndKept(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "x", "b.txt": "y"})
	es := begin(t, e, g.ID, "a.txt")
	keep(t, es, "")
	if _, err := es.Push(ctx, false); !errors.Is(err, github.ErrEmptyContent) {
		t.Fatalf("got %v", err)
	}
	if _, ok := s.Get(g.ID).Revisions[0].Files["a.txt"]; !ok {
		t.Error("the file was removed")
	}
	if _, ok, _ := e.Draft(g.ID, "a.txt"); !ok {
		t.Error("the edit is not in a draft")
	}
}

func TestContinueFromADraft(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "x"})
	first := begin(t, e, g.ID, "a.txt")
	keep(t, first, "edited")
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	s.Edit(g.ID, "a.txt", "moved on")

	es := begin(t, e, g.ID, "a.txt")
	f := es.File("a.txt")
	if !f.FromDraft || f.Content != "edited" || f.Original != "x" || f.Revision != g.Revisions[1].Version {
		t.Fatalf("got %+v; a draft keeps its own base", f)
	}
	// Opened and closed unchanged, it still differs from its base
	if changed, err := es.Keep("a.txt", es.Files[0].Content); err != nil || !changed {
		t.Errorf("changed %v, %v", changed, err)
	}
	var ce *ChangedError
	if _, err := es.Push(ctx, false); !errors.As(err, &ce) {
		t.Errorf("got %v; a draft's base is checked like any other", err)
	}
}

func TestEditingOneGistTwiceIsRefused(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "x", "b.txt": "y"})
	begin(t, e, g.ID, "a.txt")
	if _, err := e.Begin(ctx, g.ID, "b.txt"); !errors.Is(err, state.ErrLocked) {
		t.Errorf("begin: got %v", err)
	}
	if err := e.Discard(g.ID, "a.txt"); !errors.Is(err, state.ErrLocked) {
		t.Errorf("discard: got %v", err)
	}
	_, err := e.Apply(ctx, Request{Gist: g.ID, Build: func(context.Context, model.Gist) (model.Change, error) {
		t.Error("built a change under another's lock")
		return model.Change{}, nil
	}})
	if !errors.Is(err, state.ErrLocked) {
		t.Errorf("apply: got %v", err)
	}
}

func TestChangedSinceTheEditBegan(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "x"})
	es := begin(t, e, g.ID, "a.txt")
	keep(t, es, "mine")
	s.Edit(g.ID, "a.txt", "theirs")

	var ce *ChangedError
	if _, err := es.Push(ctx, false); !errors.As(err, &ce) {
		t.Fatalf("got %v", err)
	}
	if got := s.Get(g.ID).Revisions[0].Files["a.txt"]; got != "theirs" {
		t.Errorf("sent over the change: %q", got)
	}
	if _, ok, _ := e.Draft(g.ID, "a.txt"); !ok {
		t.Error("the edit is not in a draft")
	}
	res, err := es.Push(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Overwritten != "" {
		t.Errorf("overwriting on purpose reported %q", res.Overwritten)
	}
	if got := s.Get(g.ID).Revisions[0].Files["a.txt"]; got != "mine" {
		t.Errorf("got %q", got)
	}
}

func TestChangedBetweenTheCheckAndThePatch(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "x"})
	es := begin(t, e, g.ID, "a.txt")
	keep(t, es, "mine")
	s.Before("PATCH /gists/", func() { s.Edit(g.ID, "a.txt", "theirs") })
	res, err := es.Push(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := s.Get(g.ID).Revisions[1].Version; res.Overwritten != want {
		t.Errorf("reported %q, want %q", res.Overwritten, want)
	}
}

func TestReload(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "x"})
	es := begin(t, e, g.ID, "a.txt")
	keep(t, es, "mine")
	s.Edit(g.ID, "a.txt", "theirs")
	changed, err := es.Reload(ctx)
	if err != nil || !changed {
		t.Fatalf("changed %v, %v", changed, err)
	}
	if f := es.File("a.txt"); f.Original != "theirs" || f.Content != "mine" {
		t.Errorf("got %+v", f)
	}
	d, _, _ := e.Draft(g.ID, "a.txt")
	if string(d.Original) != "theirs" || d.Revision != s.Get(g.ID).Revisions[0].Version {
		t.Errorf("the draft has the old base: %+v", d)
	}
	if _, err := es.Push(ctx, false); err != nil {
		t.Errorf("after a reload: %v", err)
	}
}

func TestReloadOntoTheSameChange(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "x"})
	es := begin(t, e, g.ID, "a.txt")
	keep(t, es, "same")
	s.Edit(g.ID, "a.txt", "same")
	changed, err := es.Reload(ctx)
	if err != nil || changed {
		t.Errorf("changed %v, %v", changed, err)
	}
	if _, ok, _ := e.Draft(g.ID, "a.txt"); ok {
		t.Error("a draft stayed")
	}
}

func TestDiscard(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "x"})
	es := begin(t, e, g.ID, "a.txt")
	keep(t, es, "y")
	if err := es.Close(); err != nil {
		t.Fatal(err)
	}
	if err := e.Discard(g.ID, "a.txt"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := e.Draft(g.ID, "a.txt"); ok {
		t.Error("the draft stayed")
	}
}

func TestBeginOnAFileNotThere(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "x"})
	if _, err := e.Begin(ctx, g.ID, "b.txt"); !errors.Is(err, ErrNoFile) {
		t.Fatalf("got %v", err)
	}
	// The lock was let go
	begin(t, e, g.ID, "a.txt")
}

func description(s string) func(context.Context, model.Gist) (model.Change, error) {
	return func(context.Context, model.Gist) (model.Change, error) {
		return model.Change{Description: &s}, nil
	}
}

func content(name, s string) func(context.Context, model.Gist) (model.Change, error) {
	return func(context.Context, model.Gist) (model.Change, error) {
		return model.Change{Files: map[string]model.FileChange{name: {Content: &s}}}, nil
	}
}

func TestApplyWithABase(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "x"})
	base := g.Revisions[0].Version
	if _, err := e.Apply(ctx, Request{Gist: g.ID, Base: base, Build: content("a.txt", "y")}); err != nil {
		t.Fatal(err)
	}
	// base is now old
	var ce *ChangedError
	_, err := e.Apply(ctx, Request{Gist: g.ID, Base: base, Build: content("a.txt", "z")})
	if !errors.As(err, &ce) || ce.What != "revision" {
		t.Fatalf("got %v", err)
	}
	if got := s.Get(g.ID).Revisions[0].Files["a.txt"]; got != "y" {
		t.Errorf("got %q", got)
	}
}

func TestApplyWithoutABaseGoesOver(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "x"})
	s.Edit(g.ID, "a.txt", "theirs")
	res, err := e.Apply(ctx, Request{Gist: g.ID, Build: content("a.txt", "mine")})
	if err != nil || !res.Sent || res.Overwritten != "" {
		t.Errorf("got %+v, %v", res, err)
	}
}

func TestApplyFindsWhatItWentOver(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "x"})
	s.Before("PATCH /gists/", func() { s.Edit(g.ID, "a.txt", "theirs") })
	res, err := e.Apply(ctx, Request{Gist: g.ID, Base: g.Revisions[0].Version, Build: content("a.txt", "mine")})
	if err != nil {
		t.Fatal(err)
	}
	if res.Overwritten == "" {
		t.Error("not reported")
	}
}

func TestApplyWithABaseDescription(t *testing.T) {
	e, s := setup(t)
	g := s.Add("old", false, map[string]string{"a.txt": "x"})
	want := "old"
	s.SetDescription(g.ID, "theirs")
	var ce *ChangedError
	_, err := e.Apply(ctx, Request{Gist: g.ID, BaseDescription: &want, Build: description("mine")})
	if !errors.As(err, &ce) || ce.What != "description" {
		t.Fatalf("got %v", err)
	}
	if got := s.Get(g.ID).Description; got != "theirs" {
		t.Errorf("got %q", got)
	}
	empty := ""
	g2 := s.Add("", false, map[string]string{"a.txt": "x"})
	if _, err := e.Apply(ctx, Request{Gist: g2.ID, BaseDescription: &empty, Build: description("set")}); err != nil {
		t.Errorf("an empty base: %v", err)
	}
}

func TestDescriptionMakesNoOverwriteReport(t *testing.T) {
	e, s := setup(t)
	g := s.Add("old", false, map[string]string{"a.txt": "x"})
	s.Edit(g.ID, "a.txt", "y")
	s.Edit(g.ID, "a.txt", "z")
	res, err := e.Apply(ctx, Request{Gist: g.ID, Base: g.Revisions[0].Version, Build: description("new")})
	if err != nil || res.Overwritten != "" {
		t.Errorf("got %+v, %v", res, err)
	}
}

func TestApplySendsNothingForNoChange(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "x"})
	res, err := e.Apply(ctx, Request{Gist: g.ID, Build: func(context.Context, model.Gist) (model.Change, error) {
		return model.Change{}, nil
	}})
	if err != nil || res.Sent {
		t.Errorf("got %+v, %v", res, err)
	}
	for _, r := range s.Requests() {
		if strings.HasPrefix(r, "PATCH") {
			t.Errorf("sent %s", r)
		}
	}
}

func TestLockIsLetGoAfterApply(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "x"})
	s.Fail("PATCH", http.StatusInternalServerError)
	if _, err := e.Apply(ctx, Request{Gist: g.ID, Build: content("a.txt", "y")}); err == nil {
		t.Fatal("no error")
	}
	begin(t, e, g.ID, "a.txt")
}

func TestManyFilesGoInOnePatch(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "a", "b.txt": "b", "c.txt": "c"})
	es := begin(t, e, g.ID, "a.txt", "b.txt", "c.txt")
	if _, err := es.Keep("a.txt", "A"); err != nil {
		t.Fatal(err)
	}
	if _, err := es.Keep("b.txt", "B"); err != nil {
		t.Fatal(err)
	}
	if changed, err := es.Keep("c.txt", "c"); err != nil || changed {
		t.Fatalf("changed %v, %v", changed, err)
	}
	if got := es.Changed(); len(got) != 2 {
		t.Fatalf("%d changed", len(got))
	}
	if _, err := es.Push(ctx, false); err != nil {
		t.Fatal(err)
	}
	got := s.Get(g.ID)
	if len(got.Revisions) != 2 {
		t.Errorf("%d revisions; it should be one change", len(got.Revisions))
	}
	files := got.Revisions[0].Files
	if files["a.txt"] != "A" || files["b.txt"] != "B" || files["c.txt"] != "c" {
		t.Errorf("files are %q", files)
	}
	for _, f := range []string{"a.txt", "b.txt"} {
		if _, ok, _ := e.Draft(g.ID, f); ok {
			t.Errorf("the draft of %s stayed", f)
		}
	}
}

func TestADraftOfOneFileAmongMany(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "a", "b.txt": "b"})
	first := begin(t, e, g.ID, "a.txt")
	keep(t, first, "drafted")
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	es := begin(t, e, g.ID, "a.txt", "b.txt")
	if f := es.File("a.txt"); !f.FromDraft || f.Content != "drafted" {
		t.Errorf("got %+v", f)
	}
	if f := es.File("b.txt"); f.FromDraft || f.Content != "b" {
		t.Errorf("got %+v", f)
	}
}

func TestADraftBehindTheGistStopsThePushOfTheOthers(t *testing.T) {
	e, s := setup(t)
	g := s.Add("", false, map[string]string{"a.txt": "a", "b.txt": "b"})
	first := begin(t, e, g.ID, "a.txt")
	keep(t, first, "drafted")
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	s.Edit(g.ID, "b.txt", "moved on")
	es := begin(t, e, g.ID, "a.txt", "b.txt")
	var ce *ChangedError
	if _, err := es.Push(ctx, false); !errors.As(err, &ce) {
		t.Errorf("got %v", err)
	}
}
