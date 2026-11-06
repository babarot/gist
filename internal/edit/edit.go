// Package edit is how a gist is changed, shared by the CLI and the TUI: it
// holds the gist's lock, knows the revision a change was based on, keeps an
// edit as a draft until it is sent, and checks the gist did not move underneath
// it.
//
// What can be promised is limited by the API, which takes no condition on a
// PATCH. A change made elsewhere between the start of an edit and the check just
// before sending is always found. One made between that check and the PATCH is
// not prevented, but it is found afterwards and reported, and the revision it
// made stays on GitHub. Changing just the description makes no revision, so it
// is compared by its text, and one overwritten in that last moment is lost.
package edit

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/state"
)

// API is what editing asks of GitHub.
type API interface {
	Get(ctx context.Context, id string) (model.Gist, error)
	Raw(ctx context.Context, f model.File) (string, error)
	Update(ctx context.Context, id string, ch model.Change) (model.Gist, error)
}

// ChangedError says the gist on GitHub is not what the change was based on.
type ChangedError struct {
	// Current is the gist as it is now.
	Current model.Gist
	// What names what moved: "revision" or "description".
	What string
}

func (e *ChangedError) Error() string {
	return fmt.Sprintf("the %s of gist %s changed on GitHub", e.What, e.Current.ID)
}

// ErrNoFile says the gist has no such file.
var ErrNoFile = errors.New("no such file in the gist")

// Editor changes gists.
type Editor struct {
	API   API
	State *state.Store
	// Now is the time, for tests; nil uses the clock.
	Now func() time.Time
}

func (e *Editor) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// Result is what a change did.
type Result struct {
	// Gist is the gist after the change.
	Gist model.Gist
	// Sent says something was sent; not when there was nothing to change.
	Sent bool
	// Overwritten is the revision of a change made elsewhere that this one
	// went over, found only after sending, or "".
	Overwritten string
}

// Request is a change made without an editor.
type Request struct {
	Gist string
	// Base, when not "", is the revision the change must be based on.
	Base string
	// BaseDescription, when not nil, is the description it must be based on.
	BaseDescription *string
	// Build makes the change from the gist as it is. An empty one sends
	// nothing.
	Build func(ctx context.Context, g model.Gist) (model.Change, error)
}

// Apply makes a change under the gist's lock.
func (e *Editor) Apply(ctx context.Context, r Request) (Result, error) {
	l, err := e.State.Lock(r.Gist)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = l.Release() }()
	g, err := e.API.Get(ctx, r.Gist)
	if err != nil {
		return Result{}, err
	}
	if r.Base != "" && g.Revision() != r.Base {
		return Result{}, &ChangedError{Current: g, What: "revision"}
	}
	if r.BaseDescription != nil && g.Description != *r.BaseDescription {
		return Result{}, &ChangedError{Current: g, What: "description"}
	}
	ch, err := r.Build(ctx, g)
	if err != nil {
		return Result{}, err
	}
	if ch.Empty() {
		return Result{Gist: g}, nil
	}
	updated, err := e.API.Update(ctx, r.Gist, ch)
	if err != nil {
		return Result{}, err
	}
	res := Result{Gist: updated, Sent: true}
	if r.Base != "" && len(ch.Files) > 0 {
		res.Overwritten = overwritten(updated, r.Base)
	}
	return res, nil
}

// overwritten is the revision that a change, checked against base, turned out
// to go over, or "". A change of files makes one revision, so the one before
// it is base unless another came in between.
func overwritten(updated model.Gist, base string) string {
	if len(updated.History) < 2 || updated.History[1] == base {
		return ""
	}
	return updated.History[1]
}

// Session is the editing of one file: from taking the lock until it is sent
// or given up. Close lets the lock go.
type Session struct {
	// File is the file's name.
	File string
	// Revision and Original are the base: what the edit started from.
	// A refresh of the list does not change them.
	Revision string
	Original string
	// Content is what to edit: the original, or the draft of an earlier edit.
	Content string
	// FromDraft says Content came from a draft.
	FromDraft bool

	gist    string
	editor  *Editor
	lock    *state.Lock
	created time.Time
}

// Begin starts editing a file, from its draft if it has one. The gist's lock
// is held until Close: ErrLocked says another process has it.
func (e *Editor) Begin(ctx context.Context, gist, file string) (*Session, error) {
	l, err := e.State.Lock(gist)
	if err != nil {
		return nil, err
	}
	s := &Session{File: file, gist: gist, editor: e, lock: l, created: e.now()}
	d, ok, err := e.State.Draft(gist, file)
	if err != nil {
		_ = l.Release()
		return nil, err
	}
	if ok {
		s.Revision, s.Original, s.Content, s.FromDraft = d.Revision, string(d.Original), string(d.Content), true
		s.created = d.Created
		return s, nil
	}
	if err := s.readBase(ctx); err != nil {
		_ = l.Release()
		return nil, err
	}
	s.Content = s.Original
	return s, nil
}

// readBase takes the gist as it is now as the base.
func (s *Session) readBase(ctx context.Context) error {
	g, err := s.editor.API.Get(ctx, s.gist)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(g.Files, func(f model.File) bool { return f.Name == s.File })
	if i < 0 {
		return fmt.Errorf("%w: %s", ErrNoFile, s.File)
	}
	original, err := s.editor.API.Raw(ctx, g.Files[i])
	if err != nil {
		return err
	}
	s.Revision, s.Original = g.Revision(), original
	return nil
}

// Close lets the lock go. A draft stays.
func (s *Session) Close() error { return s.lock.Release() }

// Keep records content as the draft, if it differs from the base, and returns
// whether it does. When it does not, there is nothing to send and no draft
// is left. Comparing with the base rather than the draft means a draft opened
// and closed unchanged still goes on to be sent.
func (s *Session) Keep(content string) (bool, error) {
	s.Content = content
	if content == s.Original {
		return false, s.lock.DeleteDraft(s.File)
	}
	return true, s.lock.SaveDraft(state.Draft{
		Gist:     s.gist,
		File:     s.File,
		Revision: s.Revision,
		Original: []byte(s.Original),
		Content:  []byte(content),
		Created:  s.created,
	})
}

// Push sends the content kept, unless the gist has moved on from the base,
// which ChangedError says. With overwrite it goes over what is there. The draft
// goes once the content is sent.
func (s *Session) Push(ctx context.Context, overwrite bool) (Result, error) {
	g, err := s.editor.API.Get(ctx, s.gist)
	if err != nil {
		return Result{}, err
	}
	if !overwrite && g.Revision() != s.Revision {
		return Result{}, &ChangedError{Current: g, What: "revision"}
	}
	content := s.Content
	updated, err := s.editor.API.Update(ctx, s.gist, model.Change{
		Files: map[string]model.FileChange{s.File: {Content: &content}},
	})
	if err != nil {
		return Result{}, err
	}
	res := Result{Gist: updated, Sent: true, Overwritten: overwritten(updated, g.Revision())}
	return res, s.lock.DeleteDraft(s.File)
}

// Reload takes the gist as it is now as the new base, keeping the edit, and
// returns whether the edit still differs from it. It may not: the same change
// may have been sent from elsewhere.
func (s *Session) Reload(ctx context.Context) (bool, error) {
	if err := s.readBase(ctx); err != nil {
		return false, err
	}
	return s.Keep(s.Content)
}

// Discard removes the draft of a file, under the gist's lock: ErrLocked says
// another process is editing the gist.
func (e *Editor) Discard(gist, file string) error {
	l, err := e.State.Lock(gist)
	if err != nil {
		return err
	}
	defer func() { _ = l.Release() }()
	return l.DeleteDraft(file)
}

// Draft reads the draft kept for a file.
func (e *Editor) Draft(gist, file string) (state.Draft, bool, error) {
	return e.State.Draft(gist, file)
}
