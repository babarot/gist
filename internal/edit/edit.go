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
	"cmp"
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
	Delete(ctx context.Context, id string) error
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

// ErrDraftsLeft says a gist was deleted but its drafts could not be.
var ErrDraftsLeft = errors.New("the drafts of the deleted gist cannot be removed")

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

// Delete deletes a gist under its lock, and its drafts with it once GitHub has
// deleted it. Any answer but that keeps them, not found among them: a token
// without the permission gets that for a gist that is still there, and a draft
// can be the only copy of an edit.
func (e *Editor) Delete(ctx context.Context, gist string) (err error) {
	l, err := e.State.Lock(gist)
	if err != nil {
		return err
	}
	defer func() { err = cmp.Or(err, l.Release()) }()
	if err := e.API.Delete(ctx, gist); err != nil {
		return err
	}
	if err := l.DeleteDrafts(); err != nil {
		return fmt.Errorf("%w: %w", ErrDraftsLeft, err)
	}
	return nil
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

// FileEdit is a file being edited.
type FileEdit struct {
	// Name is the file's name.
	Name string
	// Revision and Original are the base: what the edit started from. A
	// refresh of the list does not change them.
	Revision string
	Original string
	// Content is what to edit: the original, or the draft of an earlier edit.
	Content string
	// FromDraft says Content came from a draft.
	FromDraft bool

	created time.Time
}

// Changed says the edit differs from what it started from.
func (f *FileEdit) Changed() bool { return f.Content != f.Original }

// Session is the editing of a gist's files: from taking the lock until they
// are sent or given up. Close lets the lock go.
type Session struct {
	// Files are the files being edited.
	Files []*FileEdit

	gist   string
	editor *Editor
	lock   *state.Lock
}

// File is the file being edited with this name, or nil.
func (s *Session) File(name string) *FileEdit {
	if i := slices.IndexFunc(s.Files, func(f *FileEdit) bool { return f.Name == name }); i >= 0 {
		return s.Files[i]
	}
	return nil
}

// Begin starts editing files, each from its draft if it has one. The gist's
// lock is held until Close: ErrLocked says another process has it.
func (e *Editor) Begin(ctx context.Context, gist string, files ...string) (*Session, error) {
	l, err := e.State.Lock(gist)
	if err != nil {
		return nil, err
	}
	s := &Session{gist: gist, editor: e, lock: l}
	if err := s.start(ctx, files); err != nil {
		_ = l.Release()
		return nil, err
	}
	return s, nil
}

func (s *Session) start(ctx context.Context, files []string) error {
	var fresh []*FileEdit
	for _, name := range files {
		f := &FileEdit{Name: name, created: s.editor.now()}
		d, ok, err := s.editor.State.Draft(s.gist, name)
		if err != nil {
			return err
		}
		if ok {
			f.Revision, f.Original, f.Content, f.FromDraft = d.Revision, string(d.Original), string(d.Content), true
			f.created = d.Created
		} else {
			fresh = append(fresh, f)
		}
		s.Files = append(s.Files, f)
	}
	if len(fresh) == 0 {
		return nil
	}
	if err := s.readBases(ctx, fresh); err != nil {
		return err
	}
	for _, f := range fresh {
		f.Content = f.Original
	}
	return nil
}

// readBases takes the gist as it is now as the base of files.
func (s *Session) readBases(ctx context.Context, files []*FileEdit) error {
	g, err := s.editor.API.Get(ctx, s.gist)
	if err != nil {
		return err
	}
	for _, f := range files {
		i := slices.IndexFunc(g.Files, func(gf model.File) bool { return gf.Name == f.Name })
		if i < 0 {
			return fmt.Errorf("%w: %s", ErrNoFile, f.Name)
		}
		original, err := s.editor.API.Raw(ctx, g.Files[i])
		if err != nil {
			return err
		}
		f.Revision, f.Original = g.Revision(), original
	}
	return nil
}

// DropDrafts throws away the drafts the session started from, and starts those
// files again from GitHub.
func (s *Session) DropDrafts(ctx context.Context) error {
	var drafted []*FileEdit
	for _, f := range s.Files {
		if f.FromDraft {
			drafted = append(drafted, f)
		}
	}
	if err := s.readBases(ctx, drafted); err != nil {
		return err
	}
	for _, f := range drafted {
		f.Content, f.FromDraft = f.Original, false
		if err := s.lock.DeleteDraft(f.Name); err != nil {
			return err
		}
	}
	return nil
}

// Close lets the lock go. Drafts stay.
func (s *Session) Close() error { return s.lock.Release() }

// Keep records content as the draft of a file, if it differs from the base, and
// returns whether it does. When it does not, there is nothing to send of it and
// no draft is left. Comparing with the base rather than the draft means a draft
// opened and closed unchanged still goes on to be sent.
func (s *Session) Keep(name, content string) (bool, error) {
	f := s.File(name)
	if f == nil {
		return false, fmt.Errorf("%w: %s", ErrNoFile, name)
	}
	f.Content = content
	if !f.Changed() {
		return false, s.lock.DeleteDraft(f.Name)
	}
	return true, s.lock.SaveDraft(state.Draft{
		Gist:     s.gist,
		File:     f.Name,
		Revision: f.Revision,
		Original: []byte(f.Original),
		Content:  []byte(content),
		Created:  f.created,
	})
}

// Changed are the files that differ from what they started from.
func (s *Session) Changed() []*FileEdit {
	var out []*FileEdit
	for _, f := range s.Files {
		if f.Changed() {
			out = append(out, f)
		}
	}
	return out
}

// Push sends the content kept of the files changed, in one change, unless the
// gist has moved on from what one of them is based on, which ChangedError says.
// With overwrite it goes over what is there. Their drafts go once it is sent.
func (s *Session) Push(ctx context.Context, overwrite bool) (Result, error) {
	g, err := s.editor.API.Get(ctx, s.gist)
	if err != nil {
		return Result{}, err
	}
	changed := s.Changed()
	files := make(map[string]model.FileChange, len(changed))
	for _, f := range changed {
		if !overwrite && g.Revision() != f.Revision {
			return Result{}, &ChangedError{Current: g, What: "revision"}
		}
		files[f.Name] = model.FileChange{Content: &f.Content}
	}
	updated, err := s.editor.API.Update(ctx, s.gist, model.Change{Files: files})
	if err != nil {
		return Result{}, err
	}
	res := Result{Gist: updated, Sent: true, Overwritten: overwritten(updated, g.Revision())}
	for _, f := range changed {
		if derr := s.lock.DeleteDraft(f.Name); derr != nil {
			err = derr
		}
	}
	return res, err
}

// Reload takes the gist as it is now as the new base of every file, keeping the
// edits, and returns whether any still differs from it. It may not: the same
// change may have been sent from elsewhere.
func (s *Session) Reload(ctx context.Context) (bool, error) {
	if err := s.readBases(ctx, s.Files); err != nil {
		return false, err
	}
	any := false
	for _, f := range s.Files {
		changed, err := s.Keep(f.Name, f.Content)
		if err != nil {
			return any, err
		}
		any = any || changed
	}
	return any, nil
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
