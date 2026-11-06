// Package highlight splits a file into lines of tokens, with what each token is
// rather than its color, which is for the theme to say.
package highlight

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Kind is what a token is.
type Kind int

// The kinds.
const (
	Plain Kind = iota
	Keyword
	String
	Number
	Comment
	Function
	Tag
)

// Token is text of one kind.
type Token struct {
	Text string
	Kind Kind
}

func lexer(name, content string) chroma.Lexer {
	l := lexers.Match(name)
	if l == nil {
		l = lexers.Analyse(content) //nolint:misspell // the name in chroma
	}
	if l == nil {
		l = lexers.Fallback
	}
	return chroma.Coalesce(l)
}

// Language is the language of a file, by its name and then by its content such
// as a #! line, or "text".
func Language(name, content string) string {
	l := lexer(name, content)
	if n := l.Config().Name; n != "fallback" {
		return strings.ToLower(n)
	}
	return "text"
}

// Lines are the lines of content as tokens. A file that cannot be read as its
// language comes back as plain lines.
func Lines(name, content string) [][]Token {
	plain := func() [][]Token {
		src := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
		out := make([][]Token, len(src))
		for i, l := range src {
			out[i] = []Token{{Text: l}}
		}
		return out
	}
	it, err := lexer(name, content).Tokenise(nil, content)
	if err != nil {
		return plain()
	}
	lines := chroma.SplitTokensIntoLines(it.Tokens())
	out := make([][]Token, 0, len(lines))
	for _, l := range lines {
		row := make([]Token, 0, len(l))
		for _, t := range l {
			if s := strings.TrimRight(t.Value, "\n"); s != "" {
				row = append(row, Token{s, kind(t.Type)})
			}
		}
		out = append(out, row)
	}
	// What follows the last newline is not a line
	if n := len(out); n > 0 && len(out[n-1]) == 0 {
		out = out[:n-1]
	}
	return out
}

func kind(t chroma.TokenType) Kind {
	switch {
	case t.InCategory(chroma.Comment):
		return Comment
	case t.InCategory(chroma.Keyword):
		return Keyword
	case t.InSubCategory(chroma.LiteralString):
		return String
	case t.InSubCategory(chroma.LiteralNumber):
		return Number
	case t == chroma.NameFunction, t == chroma.NameClass, t == chroma.NameBuiltin:
		return Function
	case t == chroma.NameTag, t == chroma.NameAttribute:
		return Tag
	}
	return Plain
}
