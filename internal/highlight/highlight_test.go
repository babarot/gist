package highlight

import (
	"slices"
	"strings"
	"testing"
)

func text(row []Token) string {
	var b strings.Builder
	for _, t := range row {
		b.WriteString(t.Text)
	}
	return b.String()
}

func TestLinesKeepTheText(t *testing.T) {
	src := "package main\n\n// a comment\nfunc main() {\n\tprintln(\"hi\", 1)\n}\n"
	lines := Lines("main.go", src)
	want := strings.Split(strings.TrimSuffix(src, "\n"), "\n")
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i, l := range lines {
		if got := text(l); got != want[i] {
			t.Errorf("line %d: got %q, want %q", i, got, want[i])
		}
	}
}

func has(lines [][]Token, want Token) bool {
	for _, l := range lines {
		if slices.Contains(l, want) {
			return true
		}
	}
	return false
}

func TestKinds(t *testing.T) {
	lines := Lines("main.go", "package main\n// note\nvar s = \"hi\"\nvar n = 42\n")
	for _, want := range []Token{{"package", Keyword}, {"// note", Comment}, {`"hi"`, String}, {"42", Number}} {
		if !has(lines, want) {
			t.Errorf("no %+v in %+v", want, lines)
		}
	}
}

func TestLanguage(t *testing.T) {
	for _, tt := range []struct{ name, content, want string }{
		{"main.go", "", "go"},
		{"README.md", "", "markdown"},
		{"deadlink", "#!/bin/bash\necho hi\n", "bash"},
		{"notes", "just words here", "text"},
	} {
		if got := Language(tt.name, tt.content); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestPlainFile(t *testing.T) {
	lines := Lines("notes", "one\ntwo\n")
	if len(lines) != 2 || text(lines[1]) != "two" {
		t.Errorf("got %+v", lines)
	}
}

func TestEmptyFile(t *testing.T) {
	if lines := Lines("a.go", ""); len(lines) != 0 {
		t.Errorf("got %+v", lines)
	}
}

func TestNoFinalNewline(t *testing.T) {
	if lines := Lines("notes", "one"); len(lines) != 1 || text(lines[0]) != "one" {
		t.Errorf("got %+v", lines)
	}
}
