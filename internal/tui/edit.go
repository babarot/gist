package tui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/cache"
	"github.com/babarot/gist/v2/internal/diff"
	"github.com/babarot/gist/v2/internal/edit"
	"github.com/babarot/gist/v2/internal/github"
	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/state"
	"github.com/babarot/gist/v2/internal/tui/keymap"
)

// editing is a file being edited, from the key to the push.
type editing struct {
	session *edit.Session
	gist    model.Gist
	file    string
	work    *edit.Workspace
	// diff is what is about to be sent, and scroll how far it is down.
	diff   diff.Diff
	scroll int
	// conflict is the gist as GitHub has it, when it moved on.
	conflict *edit.ChangedError
}

// inputKind is what a text box is changing.
type inputKind int

const (
	inputDescription inputKind = iota
	inputFilename
)

// inputState is a text box and what it is for.
type inputState struct {
	kind inputKind
	gist model.Gist
	file string
	box  textinput.Model
	err  string
	// base is what the change is based on: the description, or the revision.
	base string
	// conflict is the gist as GitHub has it, when it moved on.
	conflict *edit.ChangedError
}

// deleting is the question of deleting a gist.
type deleting struct {
	gist model.Gist
	file string
}

// discarding is the question of throwing a draft away.
type discarding struct {
	gist string
	file string
}

// readOnly says why a gist cannot be changed here.
func (m *Model) readOnly(g model.Gist) bool {
	if m.cur != tabStarred {
		return false
	}
	m.line.Refuse(m.now(), fmt.Sprintf("read only: %s's gist · %s opens it in the browser", g.Owner, m.keys.Hint(keymap.Browse)))
	return true
}

// busyWith says a gist is being written to, and refuses what would go with it.
func (m *Model) busyWith(g model.Gist) bool {
	if m.writing[g.ID] == "" {
		return false
	}
	m.line.Refuse(m.now(), "wait for what is being sent to "+shortID(g.ID))
	return true
}

func shortID(id string) string { return id[:min(7, len(id))] }

type beganMsg struct {
	session *edit.Session
	gist    model.Gist
	file    string
	err     error
}

// startEdit starts editing the file shown, from its draft if it has one.
func (m *Model) startEdit() tea.Cmd {
	g, ok := m.selected()
	if !ok || m.readOnly(g) || m.busyWith(g) {
		return nil
	}
	f, read := m.shownFile(g)
	switch {
	case !read:
		m.line.Refuse(m.now(), "wait for "+f.Name+" to be read")
		return nil
	case f.Binary:
		m.line.Refuse(m.now(), "a binary file cannot be edited here: use gist clone")
		return nil
	}
	editor, ctx := m.opt.Editor, m.ctx
	return func() tea.Msg {
		s, err := editor.Begin(ctx, g.ID, f.Name)
		return beganMsg{s, g, f.Name, err}
	}
}

func (m *Model) gotBegan(msg beganMsg) tea.Cmd {
	if msg.err != nil {
		m.fail(msg.err)
		return nil
	}
	m.editing = &editing{session: msg.session, gist: msg.gist, file: msg.file}
	return m.openEditor()
}

// fail says what went wrong in a way a person can act on.
func (m *Model) fail(err error) {
	switch {
	case errors.Is(err, state.ErrLocked):
		m.line.Refuse(m.now(), err.Error())
	case errors.Is(err, github.ErrEmptyContent):
		m.line.Fail(m.now(), fmt.Sprintf("GitHub removes an empty file: %s then f removes it · the edit is kept as a draft", m.keys.Hint(keymap.Delete)))
	case errors.Is(err, github.ErrRawTooLarge), errors.Is(err, github.ErrRawIncomplete):
		m.line.Fail(m.now(), err.Error())
	default:
		m.line.Fail(m.now(), err.Error())
	}
}

type editedMsg struct{ err error }

// openEditor gives the terminal to the editor, on the files of the edit.
func (m *Model) openEditor() tea.Cmd {
	e := m.editing
	work, err := e.session.Workspace()
	if err != nil {
		m.fail(err)
		m.endEdit()
		return nil
	}
	e.work = work
	args := edit.Command(m.opt.Config.Editor, work.Paths()...)
	return m.runEditor(args)
}

// runProgram runs the editor on the terminal.
func runProgram(args []string) tea.Cmd {
	cmd := exec.Command(args[0], args[1:]...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return editedMsg{err} })
}

// gotEdited reads back what the editor left. How the editor ended does not
// matter: the file is there, and it may be all there is of the edit.
func (m *Model) gotEdited(msg editedMsg) tea.Cmd {
	e := m.editing
	if e == nil {
		return nil
	}
	if msg.err != nil {
		m.opt.Log.Warn("the editor ended badly", "err", msg.err.Error())
	}
	changed, err := e.work.ReadBack(e.session)
	m.readDrafts()
	if err != nil {
		m.fail(err)
		m.endEdit()
		return nil
	}
	if !changed {
		m.line.Say(m.now(), "no changes · nothing pushed")
		m.endEdit()
		return nil
	}
	m.showDiff()
	return nil
}

func (m *Model) showDiff() {
	e := m.editing
	f := e.session.File(e.file)
	e.diff, e.scroll, e.conflict = diff.Compare(f.Original, f.Content), 0, nil
}

// endEdit lets the lock go. A draft stays.
func (m *Model) endEdit() {
	if m.editing == nil {
		return
	}
	if err := m.editing.session.Close(); err != nil {
		m.opt.Log.Warn("the lock cannot be let go", "err", err.Error())
	}
	m.editing = nil
	m.readDrafts()
}

// readDrafts learns which gists have drafts.
func (m *Model) readDrafts() {
	if m.opt.Editor == nil {
		return
	}
	gists, err := m.opt.Editor.State.DraftGists()
	if err != nil {
		m.opt.Log.Warn("the drafts cannot be read", "err", err.Error())
		return
	}
	clear(m.drafts)
	for _, id := range gists {
		m.drafts[id] = true
	}
}

type pushedMsg struct {
	result edit.Result
	err    error
}

// push sends the edit. With overwrite it goes over what changed on GitHub.
func (m *Model) push(overwrite bool) tea.Cmd {
	e := m.editing
	m.writing[e.gist.ID] = "push"
	s, ctx := e.session, m.ctx
	return func() tea.Msg {
		res, err := s.Push(ctx, overwrite)
		return pushedMsg{res, err}
	}
}

func (m *Model) gotPushed(msg pushedMsg) tea.Cmd {
	e := m.editing
	if e == nil {
		return nil
	}
	delete(m.writing, e.gist.ID)
	if changed, ok := errors.AsType[*edit.ChangedError](msg.err); ok {
		e.conflict = changed
		return nil
	}
	if msg.err != nil && !msg.result.Sent {
		m.fail(msg.err)
		m.endEdit()
		return nil
	}
	m.showResult(msg.result.Gist)
	m.line.Say(m.now(), fmt.Sprintf("✓ pushed %s (+%d −%d)", e.file, e.diff.Added, e.diff.Removed))
	m.warnOverwritten(msg.result)
	m.endEdit()
	return nil
}

// warnOverwritten says a change made elsewhere was gone over, and where it is.
func (m *Model) warnOverwritten(r edit.Result) {
	if r.Overwritten != "" {
		url := r.Gist.URL + "/" + r.Overwritten
		m.opt.Log.Warn("a change made elsewhere was overwritten", "gist", r.Gist.ID, "revision", r.Overwritten)
		m.line.Fail(m.now(), "overwrote "+url)
	}
}

// showResult puts a gist as GitHub returned it in the list, which moves it to
// the top with the cursor, and keeps the list.
func (m *Model) showResult(g model.Gist) {
	m.preview.contents[g.ID] = g
	if err := m.preview.disk.Save(g); err != nil {
		m.opt.Log.Warn("the content cannot be kept", "gist", g.ID, "err", err.Error())
	}
	row := g
	row.History = nil
	row.Files = slices.Clone(g.Files)
	for i := range row.Files {
		row.Files[i].Content = ""
	}
	t := m.tab()
	gists := slices.Clone(t.gists)
	if i := slices.IndexFunc(gists, func(o model.Gist) bool { return o.ID == g.ID }); i >= 0 {
		gists[i] = row
	} else {
		gists = append(gists, row)
	}
	m.afterChange(t, gists)
}

// afterChange shows and keeps a list after a change made here.
func (m *Model) afterChange(t *tab, gists []model.Gist) {
	cache.Sort(gists)
	m.replace(t, gists)
	t.list.Gists = gists
	m.keep(t, t.list)
}

// conflictKey answers the question of what to do about a change on GitHub.
func (m *Model) conflictKey(k string) tea.Cmd {
	e := m.editing
	switch {
	case k == keymap.Back:
		m.line.Say(m.now(), "kept as a draft")
		m.endEdit()
	case m.keys.Matches(keymap.Overwrite, k):
		e.conflict = nil
		return m.push(true)
	case m.keys.Matches(keymap.Reload, k):
		s, ctx := e.session, m.ctx
		return func() tea.Msg {
			changed, err := s.Reload(ctx)
			return reloadedMsg{changed, err}
		}
	}
	return nil
}

type reloadedMsg struct {
	changed bool
	err     error
}

// gotReloaded shows the edit against the new base, or ends if that makes it the
// same: the same change was sent from elsewhere.
func (m *Model) gotReloaded(msg reloadedMsg) {
	if m.editing == nil {
		return
	}
	switch {
	case msg.err != nil:
		m.fail(msg.err)
		m.endEdit()
	case !msg.changed:
		m.line.Say(m.now(), "no changes · nothing pushed")
		m.endEdit()
	default:
		m.readDrafts()
		m.showDiff()
	}
}

// diffKey answers the question of sending what the diff shows.
func (m *Model) diffKey(k string) tea.Cmd {
	switch {
	case k == keymap.Back:
		m.line.Say(m.now(), "kept as a draft")
		m.endEdit()
	case m.keys.Matches(keymap.Confirm, k):
		return m.push(false)
	case m.keys.Matches(keymap.Edit, k):
		return m.openEditor()
	case m.keys.Matches(keymap.ScrollDown, k), m.keys.Matches(keymap.Down, k):
		m.scrollDiff(1)
	case m.keys.Matches(keymap.ScrollUp, k), m.keys.Matches(keymap.Up, k):
		m.scrollDiff(-1)
	}
	return nil
}

func (m *Model) scrollDiff(by int) {
	e := m.editing
	e.scroll = max(0, min(e.scroll+by, max(0, len(e.diff.Lines)-m.diffRows())))
}

// diffRows is how many lines of a diff show.
func (m *Model) diffRows() int { return max(1, m.height-5-4) }

// openInput opens a text box on the gist under the cursor.
func (m *Model) openInput(kind inputKind) tea.Cmd {
	g, ok := m.selected()
	if !ok || m.readOnly(g) || m.busyWith(g) {
		return nil
	}
	box := textinput.New()
	box.Prompt = ""
	box.SetVirtualCursor(false)
	box.SetWidth(max(10, min(80, m.width-8)-8))
	in := &inputState{kind: kind, gist: g, box: box}
	switch kind {
	case inputDescription:
		in.base = g.Description
		in.box.SetValue(g.Description)
	case inputFilename:
		f, read := m.shownFile(g)
		c, ok := m.content(g)
		if !read || !ok {
			m.line.Refuse(m.now(), "wait for "+f.Name+" to be read")
			return nil
		}
		in.file, in.base = f.Name, c.Revision()
		in.box.SetValue(f.Name)
	}
	in.box.CursorEnd()
	m.input = in
	return in.box.Focus()
}

func (m *Model) inputKey(msg tea.KeyPressMsg) tea.Cmd {
	in := m.input
	if in.conflict != nil {
		return m.inputConflictKey(msg.String())
	}
	switch msg.String() {
	case keymap.Back:
		m.input = nil
		return nil
	case "enter":
		return m.sendInput(true)
	}
	var cmd tea.Cmd
	in.box, cmd = in.box.Update(msg)
	in.err = ""
	return cmd
}

func (m *Model) inputConflictKey(k string) tea.Cmd {
	in := m.input
	switch {
	case k == keymap.Back:
		m.input = nil
	case m.keys.Matches(keymap.Overwrite, k):
		in.conflict = nil
		return m.sendInput(false)
	case m.keys.Matches(keymap.Reload, k):
		// Start again from what GitHub has
		g := in.conflict.Current
		if in.kind == inputDescription {
			in.base = g.Description
			in.box.SetValue(g.Description)
		} else {
			in.base = g.Revision()
		}
		in.conflict = nil
	}
	return nil
}

// always is a change that is the same whatever the gist is.
func always(ch model.Change) func(context.Context, model.Gist) (model.Change, error) {
	return func(context.Context, model.Gist) (model.Change, error) { return ch, nil }
}

type inputSentMsg struct {
	result edit.Result
	err    error
}

// sendInput sends what was typed. With checked it stops if GitHub moved on.
func (m *Model) sendInput(checked bool) tea.Cmd {
	in := m.input
	value := in.box.Value()
	req := edit.Request{Gist: in.gist.ID}
	switch in.kind {
	case inputDescription:
		if value == in.base {
			m.input = nil
			return nil
		}
		if checked {
			base := in.base
			req.BaseDescription = &base
		}
		req.Build = always(model.Change{Description: &value})
	case inputFilename:
		if value == in.file {
			m.input = nil
			return nil
		}
		if value == "" || strings.ContainsAny(value, `/\\`) {
			in.err = "not a file name"
			return nil
		}
		if slices.ContainsFunc(in.gist.Files, func(f model.File) bool { return f.Name == value }) {
			in.err = "this gist already has " + value
			return nil
		}
		if checked {
			req.Base = in.base
		}
		from := in.file
		req.Build = always(model.Change{Files: map[string]model.FileChange{from: {Name: &value}}})
	}
	m.writing[in.gist.ID] = "change"
	editor, ctx := m.opt.Editor, m.ctx
	return func() tea.Msg {
		res, err := editor.Apply(ctx, req)
		return inputSentMsg{res, err}
	}
}

func (m *Model) gotInputSent(msg inputSentMsg) {
	in := m.input
	if in == nil {
		return
	}
	delete(m.writing, in.gist.ID)
	if changed, ok := errors.AsType[*edit.ChangedError](msg.err); ok {
		in.conflict = changed
		return
	}
	m.input = nil
	if msg.err != nil {
		m.fail(msg.err)
		return
	}
	m.showResult(msg.result.Gist)
	if in.kind == inputFilename {
		m.fileTab[in.gist.ID] = slices.IndexFunc(msg.result.Gist.Files, func(f model.File) bool { return f.Name == in.box.Value() })
		m.line.Say(m.now(), "✓ renamed to "+in.box.Value())
	} else {
		m.line.Say(m.now(), "✓ description changed")
	}
	m.warnOverwritten(msg.result)
}

// openDelete asks about deleting the gist under the cursor.
func (m *Model) openDelete() tea.Cmd {
	g, ok := m.selected()
	if !ok || m.readOnly(g) || m.busyWith(g) {
		return nil
	}
	d := &deleting{gist: g}
	if len(g.Files) > 0 {
		d.file = g.Files[m.selectedFile(g)].Name
	}
	m.deleting = d
	return nil
}

func (m *Model) deleteKey(k string) tea.Cmd {
	d := m.deleting
	switch {
	case k == keymap.Back:
		m.deleting = nil
	case m.keys.Matches(keymap.Confirm, k):
		m.deleting = nil
		m.writing[d.gist.ID] = "delete"
		return m.deleteGist(d.gist)
	case m.keys.Matches(keymap.OnlyFile, k) && len(d.gist.Files) > 1:
		m.deleting = nil
		m.writing[d.gist.ID] = "delete"
		return m.removeFile(d.gist, d.file)
	}
	return nil
}

type deletedMsg struct {
	gist string
	err  error
}

func (m *Model) deleteGist(g model.Gist) tea.Cmd {
	editor, src, ctx := m.opt.Editor, m.opt.Source, m.ctx
	return func() tea.Msg {
		l, err := editor.State.Lock(g.ID)
		if err != nil {
			return deletedMsg{g.ID, err}
		}
		defer func() { _ = l.Release() }()
		return deletedMsg{g.ID, src.Delete(ctx, g.ID)}
	}
}

func (m *Model) gotDeleted(msg deletedMsg) {
	delete(m.writing, msg.gist)
	if msg.err != nil && !github.IsNotFound(msg.err) {
		m.fail(msg.err)
		return
	}
	for i := range m.tabs {
		t := &m.tabs[i]
		if i := slices.IndexFunc(t.gists, func(g model.Gist) bool { return g.ID == msg.gist }); i >= 0 {
			m.afterChange(t, slices.Delete(slices.Clone(t.gists), i, i+1))
		}
	}
	m.preview.forget(msg.gist)
	m.removeKept(msg.gist)
	m.line.Say(m.now(), "✓ deleted "+shortID(msg.gist))
}

type fileRemovedMsg struct {
	gist   string
	file   string
	result edit.Result
	err    error
}

func (m *Model) removeFile(g model.Gist, file string) tea.Cmd {
	editor, ctx := m.opt.Editor, m.ctx
	return func() tea.Msg {
		res, err := editor.Apply(ctx, edit.Request{Gist: g.ID, Build: always(model.Change{
			Files: map[string]model.FileChange{file: {Remove: true}},
		})})
		return fileRemovedMsg{g.ID, file, res, err}
	}
}

func (m *Model) gotFileRemoved(msg fileRemovedMsg) {
	delete(m.writing, msg.gist)
	if msg.err != nil {
		m.fail(msg.err)
		return
	}
	m.showResult(msg.result.Gist)
	m.line.Say(m.now(), "✓ removed "+msg.file)
}

// openDiscard asks about throwing away the draft of the gist under the cursor:
// of the file shown, or if that has none, its first.
func (m *Model) openDiscard() tea.Cmd {
	g, ok := m.selected()
	if !ok || !m.drafts[g.ID] {
		return nil
	}
	drafts, err := m.opt.Editor.State.Drafts(g.ID)
	if err != nil {
		m.fail(err)
		return nil
	}
	if len(drafts) == 0 {
		return nil
	}
	file := drafts[0].File
	if f, _ := m.shownFile(g); slices.ContainsFunc(drafts, func(d state.Draft) bool { return d.File == f.Name }) {
		file = f.Name
	}
	m.discarding = &discarding{g.ID, file}
	return nil
}

func (m *Model) discardKey(k string) {
	d := m.discarding
	switch {
	case k == keymap.Back:
		m.discarding = nil
	case m.keys.Matches(keymap.Confirm, k):
		m.discarding = nil
		if err := m.opt.Editor.Discard(d.gist, d.file); err != nil {
			m.fail(err)
			return
		}
		m.readDrafts()
		m.line.Say(m.now(), "draft of "+d.file+" discarded")
	}
}
