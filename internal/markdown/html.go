package markdown

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// HTML in a file or a comment is turned into the Markdown that Render draws
// before it is drawn: headings, tables, lists, bold, code, images and line
// breaks become their Markdown, other tags go and their text stays. The text
// between tags is Markdown and is left as written, and so are fenced code
// blocks and code spans, whose < is not a tag.

var (
	codeSpan    = regexp.MustCompile("`[^`\n]+`")
	codeHolder  = regexp.MustCompile(`\x{E000}(\d+)\x{E001}`)
	spaceRun    = regexp.MustCompile(`\s+`)
	autolinkRaw = regexp.MustCompile(`^<([a-zA-Z][a-zA-Z0-9+.-]*:[^\s<>]*|[^\s<>@]+@[^\s<>@]+)>$`)
)

// htmlToMarkdown turns the HTML in body into Markdown.
func htmlToMarkdown(body string) string {
	if !strings.ContainsAny(body, "<&") {
		return body
	}
	var out []string
	var text []string
	flush := func() {
		if len(text) > 0 {
			out = append(out, convertHTML(strings.Join(text, "\n")))
			text = nil
		}
	}
	lines := strings.Split(body, "\n")
	for i := 0; i < len(lines); i++ {
		f := fence.FindStringSubmatch(lines[i])
		if f == nil {
			text = append(text, lines[i])
			continue
		}
		flush()
		start := i
		for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), f[1]); i++ {
		}
		out = append(out, strings.Join(lines[start:min(i+1, len(lines))], "\n"))
	}
	flush()
	return strings.Join(out, "\n")
}

// convertHTML turns the tags of s, which holds no fenced code, into
// Markdown.
func convertHTML(s string) string {
	var spans []string
	s = codeSpan.ReplaceAllStringFunc(s, func(c string) string {
		spans = append(spans, c)
		return fmt.Sprintf("%d", len(spans)-1)
	})
	c := &htmlConv{spans: spans}
	z := html.NewTokenizer(strings.NewReader(s))
	for {
		switch z.Next() {
		case html.ErrorToken:
			c.endTable()
			out := c.out.String()
			if !strings.HasSuffix(s, "\n") {
				out = strings.TrimRight(out, "\n")
			}
			return out
		case html.TextToken:
			c.text(string(z.Raw()))
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			c.start(string(name), string(z.Raw()), z)
		case html.EndTagToken:
			name, _ := z.TagName()
			c.end(string(name))
		}
	}
}

// maxListIndent is how deep a list is indented. Past this a list goes no further
// right, so that a small file of nested lists cannot make an enormous one.
const maxListIndent = 8

type htmlList struct {
	ordered bool
	n       int
}

// htmlConv is the state of convertHTML.
type htmlConv struct {
	out strings.Builder
	// spans are the code spans taken out of the text before it was read.
	spans []string
	// cell is the cell being read, while inside a td or th.
	cell *strings.Builder
	rows [][]string
	row  []string
	// tables is how deep in tables the text is; a table inside a table
	// is read as text of the outer one's cell.
	tables int
	lists  []htmlList
	// inline counts the open tags whose text is one line: headings,
	// list items and summaries.
	inline int
	pre    int
	skip   int
	// tagLine is set when a tag has ended the line, so the line break
	// after the tag in the source does not leave a blank line.
	tagLine bool
}

func (c *htmlConv) write(s string) {
	if c.cell != nil {
		c.cell.WriteString(s)
		return
	}
	c.out.WriteString(s)
}

// newline starts a line unless one has just started.
func (c *htmlConv) newline() {
	if c.cell != nil {
		c.cell.WriteString(" ")
		return
	}
	s := c.out.String()
	if t := strings.TrimRight(s, " "); t != s {
		c.out.Reset()
		c.out.WriteString(t)
		s = t
	}
	if s != "" && !strings.HasSuffix(s, "\n") {
		c.out.WriteString("\n")
	}
	c.tagLine = true
}

// blank leaves a blank line unless there is one.
func (c *htmlConv) blank() {
	c.newline()
	if s := c.out.String(); c.cell == nil && s != "" && !strings.HasSuffix(s, "\n\n") {
		c.out.WriteString("\n")
	}
}

func (c *htmlConv) text(raw string) {
	if c.skip > 0 {
		return
	}
	s := c.unescape(raw)
	if c.pre > 0 {
		c.write(s)
		return
	}
	if c.tables > 0 && c.cell == nil {
		return
	}
	if c.cell == nil {
		atLine := c.out.Len() == 0 || strings.HasSuffix(c.out.String(), "\n")
		if strings.TrimSpace(s) == "" && atLine {
			// Spaces between tags at the start of a line go.
			s = strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), "\t", "")
		}
		if c.tagLine && atLine {
			s = strings.TrimPrefix(s, "\n")
		}
		if s == "" {
			return
		}
		c.tagLine = false
	}
	if c.cell != nil || c.inline > 0 || len(c.lists) > 0 {
		s = spaceRun.ReplaceAllString(s, " ")
		cur := c.out.String()
		if c.cell != nil {
			cur = c.cell.String()
		}
		if cur == "" || strings.HasSuffix(cur, "\n") || strings.HasSuffix(cur, " ") {
			s = strings.TrimLeft(s, " ")
		}
	}
	c.write(s)
}

func (c *htmlConv) start(name, raw string, z *html.Tokenizer) {
	if c.tables > 1 && name != "table" {
		return
	}
	switch name {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		c.newline()
		c.write(strings.Repeat("#", int(name[1]-'0')) + " ")
		c.inline++
	case "p":
		if c.cell == nil && len(c.lists) == 0 {
			c.blank()
		}
	case "div", "details", "blockquote":
		c.newline()
	case "summary":
		c.newline()
		c.write("▸ ")
		c.inline++
	case "br":
		if c.cell != nil || c.inline > 0 {
			c.write(" ")
		} else {
			c.write("\n")
		}
	case "hr":
		c.newline()
		c.write("---")
		c.newline()
	case "strong", "b":
		c.write("**")
	case "code":
		if c.pre == 0 {
			c.write("`")
		}
	case "pre":
		c.newline()
		c.write("```\n")
		c.pre++
	case "ul", "ol":
		c.newline()
		c.lists = append(c.lists, htmlList{ordered: name == "ol", n: olStart(z)})
	case "li":
		if c.cell != nil {
			c.write(" ")
			return
		}
		c.newline()
		mark := "- "
		if k := len(c.lists) - 1; k >= 0 {
			c.write(strings.Repeat("  ", min(k, maxListIndent)))
			if c.lists[k].ordered {
				mark = strconv.Itoa(c.lists[k].n) + ". "
				c.lists[k].n++
			}
		}
		c.write(mark)
		c.inline++
	case "table":
		c.tables++
		if c.tables == 1 {
			c.rows, c.row = nil, nil
		}
	case "tr":
		c.endRow()
	case "th", "td":
		c.endCell()
		if c.tables == 1 {
			c.cell = &strings.Builder{}
		}
	case "img":
		c.write("![" + attr(raw, imgAlt) + "](" + attr(raw, imgSrc) + ")")
	case "script", "style":
		c.skip++
	default:
		// An autolink, <https://...>, is read as a tag; its address stays.
		if g := autolinkRaw.FindStringSubmatch(raw); g != nil {
			c.write(g[1])
		}
	}
}

var (
	imgSrc = regexp.MustCompile(`(?i)\bsrc=["']([^"']*)`)
	imgAlt = regexp.MustCompile(`(?i)\balt=["']([^"']*)`)
)

// attr is what re finds in a tag, or "".
func attr(raw string, re *regexp.Regexp) string {
	if m := re.FindStringSubmatch(raw); m != nil {
		return m[1]
	}
	return ""
}

func (c *htmlConv) end(name string) {
	if c.tables > 1 && name != "table" {
		return
	}
	switch name {
	case "h1", "h2", "h3", "h4", "h5", "h6", "summary", "li":
		c.inline = max(c.inline-1, 0)
		if name != "li" {
			c.newline()
		}
	case "p":
		if c.cell == nil && len(c.lists) == 0 {
			c.blank()
		}
	case "div", "details", "blockquote":
		c.newline()
	case "strong", "b":
		c.write("**")
	case "code":
		if c.pre == 0 {
			c.write("`")
		}
	case "pre":
		if c.pre > 0 {
			c.pre--
			c.newline()
			c.write("```")
			c.newline()
		}
	case "ul", "ol":
		if len(c.lists) > 0 {
			c.lists = c.lists[:len(c.lists)-1]
		}
		c.newline()
	case "th", "td":
		c.endCell()
	case "tr":
		c.endRow()
	case "table":
		if c.tables == 1 {
			c.endTable()
		}
		c.tables = max(c.tables-1, 0)
	case "script", "style":
		c.skip = max(c.skip-1, 0)
	}
}

// olStart is where an ol's numbers start.
func olStart(z *html.Tokenizer) int {
	for {
		k, v, more := z.TagAttr()
		if string(k) == "start" {
			if n, err := strconv.Atoi(string(v)); err == nil {
				return n
			}
		}
		if !more {
			return 1
		}
	}
}

func (c *htmlConv) endCell() {
	if c.cell == nil {
		return
	}
	s := strings.TrimSpace(spaceRun.ReplaceAllString(c.cell.String(), " "))
	c.row = append(c.row, strings.ReplaceAll(s, "|", "¦"))
	c.cell = nil
}

func (c *htmlConv) endRow() {
	c.endCell()
	if len(c.row) > 0 {
		c.rows = append(c.rows, c.row)
	}
	c.row = nil
}

// endTable writes the rows read as a Markdown table, the first row as
// its header.
func (c *htmlConv) endTable() {
	c.endRow()
	if len(c.rows) == 0 {
		return
	}
	c.blank()
	for i, r := range c.rows {
		c.out.WriteString("| " + strings.Join(r, " | ") + " |\n")
		if i == 0 {
			c.out.WriteString("|" + strings.Repeat("---|", len(r)) + "\n")
		}
	}
	c.out.WriteString("\n")
	c.rows = nil
	c.tagLine = true
}

// unescape is raw text with its entities decoded and its code spans put
// back as written. The spans go back before anything is decoded, so an
// entity in the body cannot be taken for one.
func (c *htmlConv) unescape(raw string) string {
	var b strings.Builder
	at := 0
	for _, loc := range codeHolder.FindAllStringSubmatchIndex(raw, -1) {
		n, err := strconv.Atoi(raw[loc[2]:loc[3]])
		if err != nil || n >= len(c.spans) {
			continue
		}
		b.WriteString(html.UnescapeString(raw[at:loc[0]]))
		b.WriteString(c.spans[n])
		at = loc[1]
	}
	b.WriteString(html.UnescapeString(raw[at:]))
	return b.String()
}
