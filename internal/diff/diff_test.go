package diff

import (
	"fmt"
	"strings"
	"testing"
)

func TestCompare(t *testing.T) {
	d := Compare("a\nb\nc\n", "a\nx\nc\nd\n")
	if d.Added != 2 || d.Removed != 1 {
		t.Errorf("got +%d -%d", d.Added, d.Removed)
	}
	if d.Lines[0].Kind != Hunk || !strings.HasPrefix(d.Lines[0].Text, "@@") {
		t.Errorf("got %+v", d.Lines[0])
	}
	var got []string
	for _, l := range d.Lines[1:] {
		got = append(got, string(" +-"[l.Kind])+l.Text)
	}
	if want := " a\n-b\n+x\n c\n+d"; strings.Join(got, "\n") != want {
		t.Errorf("got %q", strings.Join(got, "\n"))
	}
}

func TestNoChange(t *testing.T) {
	if d := Compare("a\n", "a\n"); len(d.Lines) != 0 || d.Added != 0 {
		t.Errorf("got %+v", d)
	}
}

func TestContextIsThreeLines(t *testing.T) {
	var lines []string
	for i := range 20 {
		lines = append(lines, fmt.Sprint(i))
	}
	before := strings.Join(lines, "\n") + "\n"
	lines[10] = "changed"
	after := strings.Join(lines, "\n") + "\n"
	if d := Compare(before, after); len(d.Lines) != 1+3+2+3 {
		t.Errorf("%d lines", len(d.Lines))
	}
}

func TestEmptyFile(t *testing.T) {
	if d := Compare("", "a\n"); d.Added != 1 {
		t.Errorf("got %+v", d)
	}
	if d := Compare("a\nb\n", ""); d.Removed != 2 {
		t.Errorf("got %+v", d)
	}
}

func TestNoFinalNewline(t *testing.T) {
	if d := Compare("a", "a\n"); d.Added+d.Removed == 0 {
		t.Error("the newline was not seen")
	}
}
