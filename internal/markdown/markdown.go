// Package markdown draws Markdown in lines of a given width, with what each
// piece of text is rather than its color, which is for the theme to say.
//
// Headings lose their #, list items get a bullet or a box, code and tables keep
// their columns, and prose wraps at the edge. An image is never fetched: reading
// it would tell the place it is on that the gist was opened, so it is drawn as
// one line. HTML is turned into Markdown first, in html.go.
package markdown

import (
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/kyokomi/emoji/v2"

	"github.com/babarot/gist/v2/internal/text"
)

// Role is what a piece of text is.
type Role int

// The roles.
const (
	Text Role = iota
	Heading
	Subheading
	Rule
	Quote
	QuoteMark
	Mark
	Done
	Todo
	Finished
	Code
	CodeBlock
	CodeLang
	Indented
	Link
	Image
	Cell
	TableLine
)

// Span is text with one role.
type Span struct {
	Text string
	Role Role
	Bold bool
	// from is the span a wrapped piece came from, so pieces of one span on a
	// line are one.
	from int
}

// Line is a line drawn.
type Line []Span

// token is a word, a run of spaces, or one other character, such as one of a
// Japanese sentence, which may break anywhere.
var token = regexp.MustCompile(`[!-~]+\p{M}*|\s+\p{M}*|` + character)

// character is one character as it shows: with the marks on it, and what a
// zero-width joiner joins it to. Their width is that of the whole.
const character = `.\p{M}*(?:\x{200D}.\p{M}*)*`

var oneCharacter = regexp.MustCompile(character)

// noLineStart are the characters a line may not start with; each goes to the
// next line with what comes before it.
const noLineStart = "、。，．）」』】,.;:)!?！？"

// noLineEnd are the characters a line may not end with; each goes to the next
// line with what comes after it.
const noLineEnd = "（「『【("

type piece struct {
	text string
	from int
}

// units are the tokens of ss, joined where a line may not break between them.
func units(ss []Span) [][]piece {
	var out [][]piece
	word := false
	for i, s := range ss {
		for _, tok := range token.FindAllString(s.Text, -1) {
			space := strings.TrimSpace(tok) == ""
			if k := len(out) - 1; k >= 0 && word && !space && joined(out[k][len(out[k])-1].text, tok) {
				out[k] = append(out[k], piece{tok, i})
			} else {
				out = append(out, []piece{{tok, i}})
			}
			word = !space
		}
	}
	return out
}

func joined(prev, next string) bool {
	r, _ := utf8.DecodeRuneInString(next)
	l, _ := utf8.DecodeLastRuneInString(prev)
	return ascii(prev) && ascii(next) || strings.ContainsRune(noLineStart, r) || strings.ContainsRune(noLineEnd, l)
}

func ascii(s string) bool {
	for i := range len(s) {
		if s[i] < '!' || s[i] > '~' {
			return false
		}
	}
	return true
}

// wrap breaks spans into lines width cells wide. Lines after the first are
// indented by hang cells, under the text after a bullet.
func wrap(ss []Span, width, hang int) []Line {
	hang = min(hang, width/2)
	var out []Line
	var cur Line
	n := 0
	add := func(s string, from Span, i int) {
		if k := len(cur) - 1; k >= 0 && cur[k].from == i {
			cur[k].Text += s
		} else {
			cur = append(cur, Span{Text: s, Role: from.Role, Bold: from.Bold, from: i})
		}
		n += text.Width(s)
	}
	newLine := func() {
		// The space a line broke at is not left at its end
		if k := len(cur) - 1; k >= 0 {
			cur[k].Text = strings.TrimRight(cur[k].Text, " ")
		}
		out = append(out, cur)
		cur, n = nil, 0
		if hang > 0 {
			cur, n = Line{{Text: strings.Repeat(" ", hang), from: -1}}, hang
		}
	}
	for _, u := range units(ss) {
		w := 0
		for _, p := range u {
			w += text.Width(p.text)
		}
		switch {
		case strings.TrimSpace(u[0].text) == "":
			if n+w > width {
				newLine()
				continue
			}
			if n == hang && len(out) > 0 {
				continue
			}
		case n+w > width && w > width-hang:
			// A word longer than a line breaks where it reaches the edge
			for _, p := range u {
				for _, r := range oneCharacter.FindAllString(p.text, -1) {
					if n+text.Width(r) > width && n > hang {
						newLine()
					}
					add(r, ss[p.from], p.from)
				}
			}
			continue
		case n+w > width:
			newLine()
		}
		for _, p := range u {
			add(p.text, ss[p.from], p.from)
		}
	}
	// The widths of tokens added up can fall short of the width of the line, as
	// what joins characters can do so across tokens
	out = append(out, cur)
	for i := range out {
		out[i] = cut(out[i], width)
	}
	return out
}

var (
	fence    = regexp.MustCompile("^\\s*(```|~~~)\\s*(\\S*)")
	heading  = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	quote    = regexp.MustCompile(`^\s*>\s?(.*)`)
	rule     = regexp.MustCompile(`^\s*([-*_])(\s*[-*_]){2,}\s*$`)
	task     = regexp.MustCompile(`^(\s*)[-*+]\s+\[([ xX])\]\s+(.*)`)
	bullet   = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)`)
	ordered  = regexp.MustCompile(`^(\s*)(\d+[.)])\s+(.*)`)
	tableSep = regexp.MustCompile(`^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$`)
	// inlineMark finds, in turn: an image in a link, an image, code, bold, a link
	inlineMark = regexp.MustCompile("\\[!\\[([^\\]]*)\\]\\(([^)\\s]*)[^)]*\\)\\]\\([^)]*\\)|!\\[([^\\]]*)\\]\\(([^)\\s]*)[^)]*\\)|`[^`]+`|\\*\\*[^*]+\\*\\*|\\[[^\\]]+\\]\\([^)]+\\)")
	linkText   = regexp.MustCompile(`^\[([^\]]+)\]`)
	shortcode  = regexp.MustCompile(`:[a-z0-9_+-]+:`)
)

// inline is a line of prose as spans.
func inline(s string, role Role, bold bool) []Span {
	var out []Span
	at := 0
	for _, m := range inlineMark.FindAllStringSubmatchIndex(s, -1) {
		if m[0] > at {
			out = append(out, Span{Text: emojize(s[at:m[0]]), Role: role, Bold: bold})
		}
		t := s[m[0]:m[1]]
		sub := func(i int) string {
			if m[i] < 0 {
				return ""
			}
			return s[m[i]:m[i+1]]
		}
		switch {
		case m[2] >= 0:
			out = append(out, image(sub(2), sub(4)))
		case m[6] >= 0:
			out = append(out, image(sub(6), sub(8)))
		case t[0] == '`':
			out = append(out, Span{Text: t[1 : len(t)-1], Role: Code})
		case t[0] == '*':
			out = append(out, Span{Text: emojize(t[2 : len(t)-2]), Role: role, Bold: true})
		default:
			out = append(out, Span{Text: emojize(linkText.FindStringSubmatch(t)[1]), Role: Link, Bold: bold})
		}
		at = m[1]
	}
	if at < len(s) {
		out = append(out, Span{Text: emojize(s[at:]), Role: role, Bold: bold})
	}
	return out
}

// image is the line an image is drawn as.
func image(alt, src string) Span {
	host := "relative path"
	if u, err := url.Parse(src); err == nil && u.Host != "" {
		host = u.Host
	}
	if alt != "" {
		alt += " · "
	}
	return Span{Text: "[image: " + alt + host + "]", Role: Image}
}

// emojize turns the shortcodes in prose into the emoji they name, as GitHub
// shows them. One with no emoji stays as written.
func emojize(s string) string {
	if !strings.Contains(s, ":") {
		return s
	}
	codes := emoji.CodeMap()
	return shortcode.ReplaceAllStringFunc(s, func(c string) string {
		if e, ok := codes[c]; ok {
			return e
		}
		return c
	})
}

// Render draws body in lines width cells wide.
func Render(body string, width int) []Line {
	width = max(width, 1)
	var out []Line
	add := func(ss []Span, hang int) { out = append(out, wrap(ss, width, hang)...) }
	src := strings.Split(htmlToMarkdown(text.Clean(body)), "\n")
	for i := 0; i < len(src); i++ {
		l := src[i]
		if f := fence.FindStringSubmatch(l); f != nil {
			var code []string
			for i++; i < len(src) && !strings.HasPrefix(strings.TrimSpace(src[i]), f[1]); i++ {
				code = append(code, src[i])
			}
			out = append(out, codeBlock(code, f[2], width)...)
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(l), "|") {
			var rows []string
			for ; i < len(src) && strings.HasPrefix(strings.TrimSpace(src[i]), "|"); i++ {
				rows = append(rows, src[i])
			}
			i--
			out = append(out, table(rows, width)...)
			continue
		}
		if g := heading.FindStringSubmatch(l); g != nil {
			role := Subheading
			if len(g[1]) <= 2 {
				role = Heading
			}
			add(inline(g[2], role, true), 0)
			continue
		}
		if rule.MatchString(l) {
			out = append(out, Line{{Text: strings.Repeat("─", width), Role: Rule}})
			continue
		}
		if g := quote.FindStringSubmatch(l); g != nil {
			for _, w := range wrap(inline(g[1], Quote, false), max(width-2, 1), 0) {
				out = append(out, append(Line{{Text: "▎ ", Role: QuoteMark}}, w...))
			}
			continue
		}
		if g := task.FindStringSubmatch(l); g != nil {
			box, role := Span{Text: "[ ] ", Role: Todo}, Text
			if g[2] != " " {
				box, role = Span{Text: "[✓] ", Role: Done}, Finished
			}
			in := indent(g[1])
			lead := []Span{{Text: in}, {Text: "• ", Role: Mark, Bold: true}, box}
			add(append(lead, inline(g[3], role, false)...), len(in)+6)
			continue
		}
		if g := bullet.FindStringSubmatch(l); g != nil {
			in := indent(g[1])
			add(append([]Span{{Text: in}, {Text: "• ", Role: Mark, Bold: true}}, inline(g[2], Text, false)...), len(in)+2)
			continue
		}
		if g := ordered.FindStringSubmatch(l); g != nil {
			in, mark := indent(g[1]), g[2]+" "
			add(append([]Span{{Text: in}, {Text: mark, Role: Mark, Bold: true}}, inline(g[3], Text, false)...), len(in)+len(mark))
			continue
		}
		if strings.HasPrefix(l, "    ") {
			out = append(out, Line{{Text: fit(l, width), Role: Indented}})
			continue
		}
		if strings.TrimSpace(l) == "" {
			if len(out) > 0 && len(out[len(out)-1]) > 0 {
				out = append(out, nil)
			}
			continue
		}
		add(inline(l, Text, false), 0)
	}
	for len(out) > 0 && len(out[len(out)-1]) == 0 {
		out = out[:len(out)-1]
	}
	return out
}

// maxIndent is how far a list goes right.
const maxIndent = 8

func indent(s string) string { return s[:min(len(s), maxIndent)] }

// fit pads s to width cells, or cuts it with "…".
func fit(s string, width int) string {
	var b strings.Builder
	for _, sp := range cut(Line{{Text: s}}, width) {
		b.WriteString(sp.Text)
	}
	s = b.String()
	return s + strings.Repeat(" ", max(0, width-text.Width(s)))
}

// codeBlock draws a fenced block on its own background, cut at the edge rather
// than wrapped, with its language at the right of the first line.
func codeBlock(code []string, lang string, width int) []Line {
	w := 0
	for _, c := range code {
		w = max(w, text.Width(c))
	}
	tag := ""
	if lang != "" {
		tag = " " + lang + " "
	}
	w = min(w+2+text.Width(tag), width)
	out := make([]Line, 0, len(code))
	for i, c := range code {
		if i == 0 && tag != "" && text.Width(" "+c)+text.Width(tag) <= w {
			out = append(out, Line{{Text: fit(" "+c, w-text.Width(tag)), Role: CodeBlock}, {Text: tag, Role: CodeLang}})
			continue
		}
		out = append(out, Line{{Text: fit(" "+c, w), Role: CodeBlock}})
	}
	return out
}

// cut shortens a line to width cells, ending in "…". It measures the line as a
// whole, as that is how it will show: what joins characters can do so across the
// pieces, so their widths added up can fall short.
func cut(l Line, width int) Line {
	if lineWidth(l) <= width {
		return l
	}
	l = firstOfLine(l, width*maxCellBytes)
	for len(l) > 0 && lineWidth(l) > width-1 {
		last := &l[len(l)-1]
		_, n := utf8.DecodeLastRuneInString(last.Text)
		last.Text = last.Text[:len(last.Text)-n]
		if last.Text == "" {
			l = l[:len(l)-1]
		}
	}
	if len(l) == 0 {
		return Line{{Text: "…"}}
	}
	end := l[len(l)-1]
	end.Text = "…"
	return append(l, end)
}

// firstOfLine is the start of l, up to n bytes. Cutting measures after each
// character it takes off, so what it starts from has to be short.
func firstOfLine(l Line, n int) Line {
	var out Line
	for _, s := range l {
		if n <= 0 {
			break
		}
		s.Text = firstBytes(s.Text, n)
		n -= len(s.Text)
		out = append(out, s)
	}
	return out
}

func lineWidth(l Line) int {
	var b strings.Builder
	for _, s := range l {
		b.WriteString(s.Text)
	}
	return text.Width(b.String())
}

// minColumn is how narrow a table's column may be made to fit.
const minColumn = 10

// maxCellBytes is the most bytes read of a cell for each cell it can show, which
// leaves room for what its Markdown takes.
const maxCellBytes = 8

// firstBytes is s up to n bytes, not ending inside a character.
func firstBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// minColumnStep is the least a column can take: a cell and what is between
// two.
const minColumnStep = 4

// table draws a table with its columns lined up. While it is wider than width,
// its widest column is narrowed, down to minColumn, so that every column shows.
// A cell wider than its column is cut with "…", and so is a table still too wide.
func table(rows []string, width int) []Line {
	// Columns past the edge cannot show, and no cell shows wider than the edge:
	// reading more would take as long as a table can be made to take
	columns := width/minColumnStep + 1
	var cells [][]string
	for _, r := range rows {
		if tableSep.MatchString(r) {
			continue
		}
		r = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(r), "|"), "|")
		var cs []string
		for c := range strings.SplitSeq(r, "|") {
			if len(cs) == columns {
				break
			}
			var b strings.Builder
			for _, s := range inline(firstBytes(strings.TrimSpace(c), width*maxCellBytes), Cell, false) {
				b.WriteString(s.Text)
			}
			cs = append(cs, b.String())
		}
		cells = append(cells, cs)
	}
	var ws []int
	for _, cs := range cells {
		for j, c := range cs {
			if j == len(ws) {
				ws = append(ws, 0)
			}
			ws[j] = max(ws[j], min(text.Width(c), width))
		}
	}
	total := 3 * (len(ws) - 1)
	for _, w := range ws {
		total += w
	}
	for total > width {
		k := 0
		for j, w := range ws {
			if w > ws[k] {
				k = j
			}
		}
		if ws[k] <= minColumn {
			break
		}
		ws[k]--
		total--
	}
	var out []Line
	for i, cs := range cells {
		var row Line
		for j, w := range ws {
			if j > 0 {
				row = append(row, Span{Text: " │ ", Role: TableLine})
			}
			c := ""
			if j < len(cs) {
				c = cs[j]
			}
			row = append(row, Span{Text: fit(c, w), Role: Cell, Bold: i == 0})
		}
		out = append(out, cut(row, width))
		if i == 0 && len(cells) > 1 {
			sep := make([]string, len(ws))
			for j, w := range ws {
				sep[j] = strings.Repeat("─", w)
			}
			out = append(out, cut(Line{{Text: strings.Join(sep, "─┼─"), Role: TableLine}}, width))
		}
	}
	return out
}
