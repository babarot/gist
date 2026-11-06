package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/babarot/gist/v2/internal/filter"
	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/text"
	"github.com/babarot/gist/v2/internal/tui/keymap"
)

// row is a gist the filter left, with what matched in it.
type row struct {
	gist model.Gist
	hits filter.Hits
}

// rowsOf are the gists a query leaves, in their order.
func rowsOf(gists []model.Gist, query string) []row {
	q := filter.Parse(query)
	rows := make([]row, 0, len(gists))
	for _, g := range gists {
		if q.Empty() {
			rows = append(rows, row{gist: g})
			continue
		}
		it := filter.Item{Description: text.Line(g.Description), Public: g.Public}
		for _, f := range g.Files {
			it.Files = append(it.Files, text.Line(f.Name))
			if f.Language != "" {
				it.Languages = append(it.Languages, f.Language)
			}
		}
		if h, ok := filter.Match(q, it); ok {
			rows = append(rows, row{g, h})
		}
	}
	return rows
}

// filterState is the filter being typed.
type filterState struct {
	box textinput.Model
}

func (m *Model) openFilter() tea.Cmd {
	box := textinput.New()
	box.Prompt = ""
	box.SetVirtualCursor(false)
	box.SetWidth(max(10, m.width-4))
	box.SetValue(m.tab().query)
	box.CursorEnd()
	m.filtering = &filterState{box}
	return m.filtering.box.Focus()
}

// filterRow is the line of the filter, under the header.
const filterRow = 1

// filterShown says that line is there: while typing, and for as long as the
// filter narrows the list, so what it is narrowed by can always be seen.
func (m *Model) filterShown() bool { return m.filtering != nil || m.tab().query != "" }

// drawFilter writes the filter as it was typed, the key of a term set apart.
func (m *Model) drawFilter(c *canvas) {
	p := m.pal
	query := m.tab().query
	x := c.put(filterRow, 0, "/ ", style{fg: p.yellow, bold: true})
	for _, word := range strings.SplitAfter(query, " ") {
		key, rest, ok := strings.Cut(word, ":")
		if ok && slices.Contains(filter.Keys, key+":") {
			x = c.put(filterRow, x, key+":", style{fg: p.blue})
			word = rest
		}
		x = c.put(filterRow, x, word, style{fg: p.yellow})
	}
}

// setFilter narrows the tab to query.
func (m *Model) setFilter(query string) {
	t := m.tab()
	t.query = query
	m.refilter(t)
}

func (m *Model) filterKey(msg tea.KeyPressMsg) tea.Cmd {
	t := m.tab()
	switch k := msg.String(); k {
	case keymap.Back:
		m.filtering = nil
		m.setFilter("")
		return nil
	case "enter":
		m.filtering = nil
		return nil
	case "up":
		m.moveTo(t.cur - 1)
		return nil
	case "down":
		m.moveTo(t.cur + 1)
		return nil
	}
	var cmd tea.Cmd
	m.filtering.box, cmd = m.filtering.box.Update(msg)
	if v := m.filtering.box.Value(); v != t.query {
		m.setFilter(v)
	}
	return cmd
}

// marked splits s into what matched, in the color of a match, and the rest.
func (m *Model) marked(s string, hits []int, st style) []seg {
	if len(hits) == 0 {
		return []seg{{s, st}}
	}
	hit := style{fg: m.pal.yellow, bg: st.bg, bold: true}
	marks := map[int]bool{}
	for _, h := range hits {
		marks[h] = true
	}
	var out []seg
	start, on := 0, marks[0]
	runes := []rune(text.Line(s))
	for i := 1; i <= len(runes); i++ {
		if i == len(runes) || marks[i] != on {
			cur := st
			if on {
				cur = hit
			}
			out = append(out, seg{string(runes[start:i]), cur})
			start, on = i, marks[i]
		}
	}
	return out
}
