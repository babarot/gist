package config

import (
	"cmp"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2/unstable"
)

// A mistake in the config file is shown where it is, as linters show theirs:
// the file, line and column, what is wrong, and the line with the mistake
// marked.
//
//	~/.config/gist/config.toml:3:9: theme must be one of auto, default, latte, got "lattee"; did you mean "latte"?
//	  |
//	3 | theme = "lattee"
//	  |         ^~~~~~~~

// Problem is a mistake at a setting of the config file.
type Problem struct {
	// Key is the setting, as its table and name: new, visibility.
	Key []string
	// Index is which element of a list the mistake is in, or -1 for the whole
	// value.
	Index int
	// AtKey marks the name rather than the value, as for a name that is no
	// setting.
	AtKey bool
	// Line and Col place the mistake when the file says where it is itself, as
	// for a file that does not parse. They win over Key.
	Line, Col int
	Message   string
}

// Problems are every mistake found in the config file. Its Error lists them
// without where they are; Report shows them in the file.
type Problems []Problem

func (ps Problems) Error() string {
	msgs := make([]string, len(ps))
	for i, p := range ps {
		msgs[i] = p.Message
	}
	return strings.Join(msgs, "\n")
}

// Report shows ps in the file at path, each with the line it is on, top to
// bottom. One the file has no place for goes first.
func Report(path string, ps Problems) string {
	src, _ := os.ReadFile(path)
	shown := TildePath(path)
	lines := strings.Split(string(src), "\n")
	type placed struct {
		msg              string
		line, col, width int
	}
	all := make([]placed, len(ps))
	for i, p := range ps {
		all[i] = placed{msg: p.Message, line: p.Line, col: p.Col, width: 1}
		if p.Line == 0 {
			all[i].line, all[i].col, all[i].width = locate(src, p)
		}
	}
	slices.SortStableFunc(all, func(a, b placed) int { return cmp.Or(a.line-b.line, a.col-b.col) })
	var b strings.Builder
	for i, pl := range all {
		if i > 0 {
			b.WriteString("\n")
		}
		if pl.line == 0 {
			fmt.Fprintf(&b, "%s: %s\n", shown, pl.msg)
			continue
		}
		fmt.Fprintf(&b, "%s:%d:%d: %s\n", shown, pl.line, pl.col, pl.msg)
		if pl.line > len(lines) {
			continue
		}
		num := fmt.Sprint(pl.line)
		gutter := strings.Repeat(" ", len(num))
		line := strings.ReplaceAll(lines[pl.line-1], "\t", " ")
		mark := "^" + strings.Repeat("~", max(0, pl.width-1))
		fmt.Fprintf(&b, "%s |\n%s | %s\n%s | %s%s\n", gutter, num, line, gutter, strings.Repeat(" ", max(0, pl.col-1)), mark)
	}
	return strings.TrimRight(b.String(), "\n")
}

// TildePath writes a path under the home directory with ~.
func TildePath(p string) string {
	h, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(p, h+string(os.PathSeparator)) {
		return "~" + p[len(h):]
	}
	return p
}

// locate finds where p is in src: its line, column and width, marking the value
// (or one element of it) or the name. 0 when it is not in the file.
func locate(src []byte, p Problem) (line, col, width int) {
	var parser unstable.Parser
	parser.Reset(src)
	var table []string
	// The parser reuses its nodes for the next expression, so a node is read
	// where it is found, never kept
	at := func(n *unstable.Node) (int, int, int) {
		s := parser.Shape(n.Raw)
		return s.Start.Line, s.Start.Column, max(1, s.End.Column-s.Start.Column)
	}
	keyOf := func(n *unstable.Node) (parts []string, last *unstable.Node) {
		for it := n.Key(); it.Next(); {
			last = it.Node()
			parts = append(parts, string(last.Data))
		}
		return parts, last
	}
	for parser.NextExpression() {
		e := parser.Expression()
		switch e.Kind {
		case unstable.Table, unstable.ArrayTable:
			var last *unstable.Node
			table, last = keyOf(e)
			if p.AtKey && slices.Equal(table, p.Key) {
				return at(last)
			}
		case unstable.KeyValue:
			parts, name := keyOf(e)
			if !slices.Equal(append(slices.Clone(table), parts...), p.Key) {
				continue
			}
			if p.AtKey {
				return at(name)
			}
			v := e.Value()
			if v.Kind == unstable.Array && p.Index >= 0 {
				i := 0
				for it := v.Children(); it.Next(); i++ {
					if i == p.Index && it.Node().Raw.Length > 0 {
						return at(it.Node())
					}
				}
			}
			if v.Raw.Length > 0 {
				return at(v)
			}
			return at(name)
		}
	}
	return 0, 0, 0
}
