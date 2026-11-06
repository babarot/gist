package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/config"
	"github.com/babarot/gist/v2/internal/edit"
	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/tui/keymap"
)

// The fields of the form.
const (
	fieldName = iota
	fieldDescription
	fieldVisibility
	fieldContent
	numFields
)

// formState is the form of a new gist.
type formState struct {
	name, description textinput.Model
	public            bool
	content           string
	focus             int
	err               string
	// scratch is the file the editor has.
	scratch *edit.Scratch
}

func newField() textinput.Model {
	box := textinput.New()
	box.Prompt = ""
	box.SetVirtualCursor(false)
	box.SetWidth(50)
	return box
}

func (m *Model) startForm() tea.Cmd {
	m.form = &formState{name: newField(), description: newField(),
		public: m.opt.Config.New.Visibility == config.VisibilityPublic}
	if n := m.opt.NewGist; n != nil {
		m.form.public = n.Public
		m.form.description.SetValue(n.Description)
	}
	return m.form.name.Focus()
}

// openForm opens the form, which makes a gist of the user's own.
func (m *Model) openForm() tea.Cmd {
	if m.cur == tabStarred {
		m.line.Refuse(m.now(), "switch to "+m.keys.Hint(keymap.Mine)+" mine to create a gist")
		return nil
	}
	return m.startForm()
}

// focusField moves to a field, the text boxes taking the keys only when on them.
func (m *Model) focusField(i int) tea.Cmd {
	f := m.form
	f.focus = (i + numFields) % numFields
	f.name.Blur()
	f.description.Blur()
	switch f.focus {
	case fieldName:
		return f.name.Focus()
	case fieldDescription:
		return f.description.Focus()
	}
	return nil
}

// closeForm ends the form. Run alone, that ends the program.
func (m *Model) closeForm() tea.Cmd {
	m.form = nil
	if m.opt.FormOnly {
		return m.quit()
	}
	return nil
}

func (m *Model) formKey(msg tea.KeyPressMsg) tea.Cmd {
	f := m.form
	k := msg.String()
	switch {
	case k == keymap.Back:
		return m.closeForm()
	case k == "tab", k == "down":
		return m.focusField(f.focus + 1)
	case k == "shift+tab", k == "up":
		return m.focusField(f.focus - 1)
	case m.keys.Matches(keymap.Create, k):
		return m.create()
	case m.keys.Matches(keymap.OpenEditor, k):
		return m.openFormEditor()
	}
	switch f.focus {
	case fieldVisibility:
		switch k {
		case "space", " ", "left", "right":
			f.public = !f.public
		case "enter":
			return m.focusField(f.focus + 1)
		}
	case fieldContent:
		if k == "enter" {
			return m.openFormEditor()
		}
	default:
		if k == "enter" {
			return m.focusField(f.focus + 1)
		}
		box := &f.name
		if f.focus == fieldDescription {
			box = &f.description
		}
		var cmd tea.Cmd
		*box, cmd = box.Update(msg)
		f.err = ""
		return cmd
	}
	return nil
}

// openFormEditor writes the content in the editor. A gist that does not exist
// yet has no draft, so what was written stays in the form.
func (m *Model) openFormEditor() tea.Cmd {
	f := m.form
	name := f.name.Value()
	if name == "" {
		name = "untitled"
	}
	s, err := edit.NewScratch(name, f.content)
	if err != nil {
		f.err = err.Error()
		return nil
	}
	f.scratch = s
	m.sending = true
	return m.runEditor(edit.Command(m.opt.Config.Editor, s.Path()))
}

func (m *Model) gotFormEdited(msg editedMsg) {
	m.sending = false
	f := m.form
	if f.scratch == nil {
		return
	}
	content, err := f.scratch.Read()
	f.scratch = nil
	switch {
	case err != nil:
		f.err = err.Error()
	case msg.err != nil && content == f.content:
		f.err = "the editor failed: " + msg.err.Error()
	default:
		f.content, f.err = content, ""
	}
}

type createdMsg struct {
	gist model.Gist
	err  error
}

func (m *Model) create() tea.Cmd {
	f := m.form
	name := strings.TrimSpace(f.name.Value())
	switch {
	case name == "":
		f.err = "a filename is required"
		return m.focusField(fieldName)
	case strings.ContainsAny(name, `/\`):
		f.err = "a filename cannot have / or \\ in it"
		return m.focusField(fieldName)
	case f.content == "":
		f.err = fmt.Sprintf("GitHub does not take an empty file: %s to write it", m.keys.Hint(keymap.OpenEditor))
		return m.focusField(fieldContent)
	}
	n := model.NewGist{Description: f.description.Value(), Public: f.public, Files: map[string]string{name: f.content}}
	m.sending = true
	src, ctx := m.opt.Source, m.ctx
	return func() tea.Msg {
		g, err := src.Create(ctx, n)
		return createdMsg{g, err}
	}
}

func (m *Model) gotCreated(msg createdMsg) tea.Cmd {
	m.sending = false
	if msg.err != nil {
		if m.form != nil {
			m.form.err = msg.err.Error()
		}
		return nil
	}
	if m.opt.FormOnly {
		m.created = &msg.gist
		return m.closeForm()
	}
	m.form = nil
	g := msg.gist
	t := &m.tabs[tabMine]
	// A new gist would not show if the filter leaves it out
	t.query = ""
	gists := append(slices.Clone(t.gists), g)
	m.cur = tabMine
	m.afterChange(t, gists)
	if i := slices.IndexFunc(t.rows, func(r row) bool { return r.gist.ID == g.ID }); i >= 0 {
		t.cur = i
		m.keepCursorShown()
	}
	m.showResult(g)
	vis := "secret"
	if g.Public {
		vis = "public"
	}
	m.line.Say(m.now(), fmt.Sprintf("✓ created gist %s (%s) · %s copies its URL", shortID(g.ID), vis, m.keys.Hint(keymap.CopyURL)))
	return nil
}

const (
	formX    = 17 // where the values start, in the box
	formName = 2
	formDesc = 3
)

func (m *Model) formBox() (x, y, w, h int) {
	w, h = min(78, m.width-8), min(15, m.height)
	x, y = m.boxAt(w, h, 4)
	return x, y, w, h
}

func (m *Model) drawForm(c *canvas) {
	p := m.pal
	f := m.form
	x, y, w, h := m.formBox()
	c.box(x, y, w, h, p.blue, nil)
	title(c, x, y, "new gist", style{fg: p.subtext1, bold: true})
	lx, vx, fw := x+3, x+formX, w-formX-4
	label := func(i, yy int, s string) {
		st := style{fg: p.subtext0}
		if f.focus == i {
			st = style{fg: p.blue, bold: true}
		}
		c.put(yy, lx, s, st)
	}
	field := func(i, yy int, name string, box textinput.Model, hint string) {
		label(i, yy, name)
		bar := p.surface1
		if f.focus == i {
			bar = p.blue
		}
		c.put(yy, vx-2, "┃", style{fg: bar})
		if v := box.Value(); v != "" {
			pos := 0
			if f.focus == i {
				pos = box.Position()
			}
			shown, _ := inputView(v, pos, fw)
			c.putCut(yy, vx, []seg{{shown, style{fg: p.text}}}, fw, style{fg: p.overlay})
		} else if f.focus != i {
			c.put(yy, vx, hint, style{fg: p.dim})
		}
	}
	field(fieldName, y+formName, "filename", f.name, "hello.py")
	field(fieldDescription, y+formDesc, "description", f.description, "(optional)")

	label(fieldVisibility, y+5, "visibility")
	dot := func(on bool, color style) (string, style) {
		if on {
			return "●", color
		}
		return "○", style{fg: p.overlay}
	}
	name := func(on bool) style {
		if on {
			return style{fg: p.text, bold: true}
		}
		return style{fg: p.overlay}
	}
	s, st := dot(!f.public, style{fg: p.peach})
	vx2 := c.put(y+5, vx, s, st)
	vx2 = c.put(y+5, vx2, " secret   ", name(!f.public))
	s, st = dot(f.public, style{fg: p.blue})
	vx2 = c.put(y+5, vx2, s, st)
	vx2 = c.put(y+5, vx2, " public", name(f.public))
	if f.focus == fieldVisibility {
		c.put(y+5, vx2+4, "space switches", style{fg: p.dim})
	}
	note := "only people with the URL can see it"
	if f.public {
		note = "listed on your profile and searchable"
	}
	c.put(y+6, vx, note, style{fg: p.dim})

	label(fieldContent, y+8, "content")
	if f.content != "" {
		n := lineCount(f.content)
		cx := c.put(y+8, vx, fmt.Sprintf("%d %s", n, plural(n, "line", "lines")), style{fg: p.text})
		c.put(y+8, cx, " · "+size(int64(len(f.content))), style{fg: p.overlay})
		first, _, _ := strings.Cut(f.content, "\n")
		c.putCut(y+9, vx, []seg{{"│ " + first, style{fg: p.dim}}}, fw, style{fg: p.overlay})
	} else {
		c.put(y+8, vx, "empty", style{fg: p.dim})
	}
	opens := m.keys.Hint(keymap.OpenEditor) + " opens $EDITOR"
	if f.focus == fieldContent {
		opens = "enter or " + opens
	}
	c.put(y+8, vx+20, opens, style{fg: p.dim})
	if f.err != "" {
		c.putCut(y+11, lx, []seg{{"✗ " + f.err, style{fg: p.red}}}, w-6, style{fg: p.overlay})
	}
	m.keyLine(c, y+h-2, lx, []hint{{"tab", "next"}, {m.keys.Hint(keymap.OpenEditor), "$EDITOR"}, {m.keys.Hint(keymap.Create), "create"}, {keymap.Back, "cancel"}})
}

// formCursor is where the terminal's cursor goes in the form.
func (m *Model) formCursor() *tea.Cursor {
	f := m.form
	x, y, w, _ := m.formBox()
	width := w - formX - 4
	switch f.focus {
	case fieldName:
		return boxCursor(f.name, x+formX, y+formName, width)
	case fieldDescription:
		return boxCursor(f.description, x+formX, y+formDesc, width)
	}
	return nil
}
