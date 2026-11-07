package tui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

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
	// base is the filter as typed before tab filled in the first value, and at
	// the value it is on now, while tab goes round them.
	cycling bool
	base    string
	at      int
}

func (m *Model) openFilter() tea.Cmd {
	box := textinput.New()
	box.Prompt = ""
	box.SetVirtualCursor(false)
	box.SetWidth(max(10, m.width-4))
	box.SetValue(m.tab().query)
	box.CursorEnd()
	m.filtering = &filterState{box: box}
	return m.filtering.box.Focus()
}

const (
	filterRow  = 1 // the line of the filter, under the header
	filterText = 2 // where the text starts on it
)

// filterShown says that line is there: while typing, and for as long as the
// filter narrows the list, so what it is narrowed by can always be seen.
func (m *Model) filterShown() bool { return m.filtering != nil || m.tab().query != "" }

// drawFilter writes the filter as it was typed, the key of a term set apart.
func (m *Model) drawFilter(c *canvas) {
	p := m.pal
	query := m.tab().query
	if m.filtering != nil {
		query, _ = inputView(query, m.filtering.box.Position(), c.w-filterText)
	}
	x := c.put(filterRow, 0, "/ ", style{fg: p.yellow, bold: true})
	for _, word := range strings.SplitAfter(query, " ") {
		key, rest, ok := strings.Cut(word, ":")
		if ok && slices.Contains(filter.Keys, key+":") {
			x = c.put(filterRow, x, key+":", style{fg: p.blue})
			word = rest
		}
		x = c.put(filterRow, x, word, style{fg: p.yellow})
	}
	c.put(filterRow, x, m.filterGhost(), style{fg: p.overlay})
}

// atEnd says the cursor is after the whole filter: what is completed is what it
// ends with, so nothing is offered from the middle of it.
func (f *filterState) atEnd() bool {
	return f.box.Position() == utf8.RuneCountInString(f.box.Value())
}

// valueCandidates are the values the last word of query may be completed to,
// from the gists the rest of the query leaves, so that a value never leaves no
// rows. head is query up to the value; what most gists have comes first.
func (m *Model) valueCandidates(query string) (head string, cands []string) {
	word := filter.LastWord(query)
	key, value, ok := filter.ValueTerm(word)
	if !ok {
		return "", nil
	}
	low := strings.ToLower(value)
	seen := map[string]int{}
	for _, r := range rowsOf(m.tab().gists, query[:len(query)-len(word)]) {
		for _, v := range valuesOf(key, r.gist) {
			if l := strings.ToLower(v); l != low && strings.HasPrefix(l, low) {
				seen[v]++
			}
		}
	}
	for v := range seen {
		cands = append(cands, v)
	}
	slices.SortFunc(cands, func(a, b string) int {
		return cmp.Or(seen[b]-seen[a], cmp.Compare(strings.ToLower(a), strings.ToLower(b)))
	})
	return query[:len(query)-len(value)], cands
}

// valuesOf are what key can be completed to from g. A term ends at a space, and
// its value is looked for inside the name, so a name with spaces gives its first
// word: Vim Script is found by lang:vim.
func valuesOf(key string, g model.Gist) []string {
	if key == filter.VisKey {
		if g.Public {
			return []string{"public"}
		}
		return []string{"secret"}
	}
	var out []string
	for _, f := range g.Files {
		name := f.Language
		if key == filter.FileKey {
			name = text.Line(f.Name)
		}
		if w := strings.Fields(name); len(w) > 0 {
			out = append(out, w[0])
		}
	}
	return out
}

// cycled are the values tab is going round, while it still is: the filter is
// still what tab left, and that value is still among them.
func (m *Model) cycled() (head string, cands []string, ok bool) {
	f := m.filtering
	if !f.cycling {
		return "", nil, false
	}
	head, cands = m.valueCandidates(f.base)
	if f.at >= len(cands) || f.box.Value() != head+cands[f.at] {
		return "", nil, false
	}
	return head, cands, true
}

// completeFilter fills in what the ghost offers: the rest of a key, a value,
// and on the next tab the next value.
func (m *Model) completeFilter() {
	f := m.filtering
	if !f.atEnd() {
		return
	}
	set := func(s string) {
		f.box.SetValue(s)
		f.box.CursorEnd()
		m.setFilter(s)
	}
	value := f.box.Value()
	if rest := filter.KeyRest(value); rest != "" {
		set(value + rest)
		return
	}
	if head, cands, ok := m.cycled(); ok {
		f.at = (f.at + 1) % len(cands)
		set(head + cands[f.at])
		return
	}
	f.cycling = false
	if head, cands := m.valueCandidates(value); len(cands) > 0 {
		f.cycling, f.base, f.at = true, value, 0
		set(head + cands[0])
	}
}

// filterGhost is what shows faint after the cursor: the keys while the filter is
// empty, then what tab would fill in, and which of the values it is on.
func (m *Model) filterGhost() string {
	f := m.filtering
	if f == nil || !f.atEnd() {
		return ""
	}
	value := f.box.Value()
	if value == "" {
		return strings.Join(filter.Keys, " ")
	}
	if rest := filter.KeyRest(value); rest != "" {
		return rest
	}
	count := func(i, n int) string {
		if n < 2 {
			return ""
		}
		return fmt.Sprintf(" (%d/%d)", i+1, n)
	}
	if _, cands, ok := m.cycled(); ok {
		return count(f.at, len(cands))
	}
	_, cands := m.valueCandidates(value)
	if len(cands) == 0 {
		return ""
	}
	_, typed, _ := filter.ValueTerm(filter.LastWord(value))
	first := []rune(cands[0])
	return string(first[min(utf8.RuneCountInString(typed), len(first)):]) + count(0, len(cands))
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
	case "tab":
		m.completeFilter()
		return nil
	case "up":
		m.moveTo(t.cur - 1)
		return nil
	case "down":
		m.moveTo(t.cur + 1)
		return nil
	case "pgup":
		m.moveTo(t.cur - m.layout().listRows)
		return nil
	case "pgdown":
		m.moveTo(t.cur + m.layout().listRows)
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
