package markdown

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/babarot/gist/v2/internal/text"
)

// plain is what is drawn, a string to a line.
func plain(lines []Line) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		var b strings.Builder
		for _, s := range l {
			b.WriteString(s.Text)
		}
		out[i] = strings.TrimRight(b.String(), " ")
	}
	return out
}

func TestRender(t *testing.T) {
	for _, tt := range []struct {
		name  string
		src   string
		width int
		want  []string
	}{
		{"heading", "# Title #\n\nbody", 20, []string{"Title", "", "body"}},
		{"bullets", "- one\n  - two\n* three", 20, []string{"• one", "  • two", "• three"}},
		{"a list goes only so far right", strings.Repeat(" ", 30) + "- x", 40, []string{"        • x"}},
		{"bullet wraps under its text", "- aaaa bbbb cccc", 8, []string{"• aaaa", "  bbbb", "  cccc"}},
		{"ordered", "1. one\n2) two", 20, []string{"1. one", "2) two"}},
		{"tasks", "- [ ] todo\n- [x] done", 20, []string{"• [ ] todo", "• [✓] done"}},
		{"quote", "> quoted", 20, []string{"▎ quoted"}},
		{"rule", "---", 5, []string{"─────"}},
		{"code span", "use `go test` now", 30, []string{"use go test now"}},
		{"bold", "a **bold** word", 30, []string{"a bold word"}},
		{"link", "see [the docs](https://example.com) here", 30, []string{"see the docs here"}},
		{"emoji", "ship it :rocket:", 30, []string{"ship it 🚀"}},
		{"no such emoji", "ship it :nothing_like_it:", 30, []string{"ship it :nothing_like_it:"}},
		{"no emoji in code", "`:rocket:`", 30, []string{":rocket:"}},
		{"blank lines collapse", "a\n\n\n\nb\n\n", 30, []string{"a", "", "b"}},
		{"indented code", "    code  here", 30, []string{"    code  here"}},
		{"fence", "```go\nx := 1\n```", 20, []string{" x := 1  go"}},
		{"fence is cut, not wrapped", "```\n" + strings.Repeat("x", 30) + "\n```", 10, []string{" xxxxxxxx…"}},
		{"a fence never closed", "```\nabc", 20, []string{" abc"}},
		{"a long word breaks", strings.Repeat("x", 12), 5, []string{"xxxxx", "xxxxx", "xx"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := plain(Render(tt.src, tt.width)); !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJapaneseWraps(t *testing.T) {
	got := plain(Render("これはとても長い日本語の文章です。", 10))
	want := []string{"これはとて", "も長い日本", "語の文章で", "す。"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCharactersOfManyPartsAreCountedWhole(t *testing.T) {
	for _, src := range []string{
		strings.Repeat("\u24c2\ufe0f", 30),
		strings.Repeat("\U0001F468\u200d\U0001F469\u200d\U0001F466", 30),
		strings.Repeat("e\u0301", 30),
		strings.Repeat("1\ufe0f\u20e3", 30),
	} {
		for _, l := range plain(Render(src, 11)) {
			if w := text.Width(l); w > 11 {
				t.Errorf("%d wide: %q", w, l)
			}
		}
	}
}

func TestLineBreakRules(t *testing.T) {
	for _, tt := range []struct {
		src   string
		width int
		want  []string
	}{
		{"あいう、えお", 6, []string{"あい", "う、え", "お"}},
		{"あいう（えお", 8, []string{"あいう", "（えお"}},
	} {
		if got := plain(Render(tt.src, tt.width)); !slices.Equal(got, tt.want) {
			t.Errorf("%s: got %q, want %q", tt.src, got, tt.want)
		}
	}
}

func TestTable(t *testing.T) {
	src := "| a | b |\n|---|---|\n| 1 | **2** |"
	want := []string{"a │ b", "──┼──", "1 │ 2"}
	got := plain(Render(src, 30))
	for i := range got {
		got[i] = strings.TrimSpace(got[i])
	}
	if strings.ReplaceAll(strings.Join(got, "\n"), " ", "") != strings.ReplaceAll(strings.Join(want, "\n"), " ", "") {
		t.Errorf("got %q", got)
	}
}

func TestTableNarrowsItsWidestColumn(t *testing.T) {
	src := "| short | " + strings.Repeat("長", 40) + " |\n|---|---|\n| x | y |"
	for _, l := range plain(Render(src, 40)) {
		if w := text.Width(l); w > 40 {
			t.Errorf("%d wide: %q", w, l)
		}
	}
	if got := plain(Render(src, 40))[0]; !strings.Contains(got, "…") || !strings.Contains(got, "short") {
		t.Errorf("got %q", got)
	}
}

// TestATableOfManyColumnsDoesNotTakeLong measures work rather than time: a slow
// machine under the race detector must not fail it, and a cost that grows with
// the table must.
func TestATableOfManyColumnsDoesNotTakeLong(t *testing.T) {
	small := "|" + strings.Repeat("x", 500) + strings.Repeat("|x", 40) + "|"
	large := "|" + strings.Repeat("x", 500000) + strings.Repeat("|x", 40000) + "|"
	cost := func(src string) float64 {
		return testing.AllocsPerRun(1, func() { Render(src, 80) })
	}
	if a, b := cost(small), cost(large); b > a*2 {
		t.Errorf("%.0f allocations for a table a thousand times the size of one taking %.0f", b, a)
	}
	if w := text.Width(plain(Render(large, 80))[0]); w > 80 {
		t.Errorf("%d wide", w)
	}
}

// TestALongLineDoesNotTakeLong gives each a minute, far more than any machine
// needs: without the bound on cutting a line, these take hours.
func TestALongLineDoesNotTakeLong(t *testing.T) {
	line := strings.Repeat("日本語 x", 200000)
	for name, src := range map[string]string{
		"code block":  "```\n" + line + "\n```",
		"indented":    "    " + line,
		"prose":       line,
		"table":       "|" + line + "|",
		"a long word": strings.ReplaceAll(line, " ", ""),
	} {
		t.Run(name, func(t *testing.T) {
			done := make(chan struct{})
			go func() {
				Render(src, 80)
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(time.Minute):
				t.Fatal("it took too long")
			}
		})
	}
}

func TestImages(t *testing.T) {
	for src, want := range map[string]string{
		"![a cat](https://img.example.com/cat.png)":                             "[image: a cat · img.example.com]",
		"![](https://img.example.com/cat.png)":                                  "[image: img.example.com]",
		"![local](./cat.png)":                                                   "[image: local · relative path]",
		"[![badge](https://shields.example/b.svg)](https://example.com)":        "[image: badge · shields.example]",
		`<img src="https://img.example.com/cat.png" alt="a cat" width="100">`:   "[image: a cat · img.example.com]",
		`<a href="https://example.com"><img src="https://x.example/y.png"></a>`: "[image: x.example]",
	} {
		lines := Render(src, 60)
		if got := plain(lines); len(got) != 1 || got[0] != want {
			t.Errorf("%s: got %q, want %q", src, got, want)
		}
		if lines[0][0].Role != Image {
			t.Errorf("%s: the role is %v", src, lines[0][0].Role)
		}
	}
}

func TestHTML(t *testing.T) {
	got := plain(Render("<h2>Title</h2>\n<p>a <b>bold</b> word</p>\n<ul><li>one</li></ul>", 30))
	if !slices.Contains(got, "Title") || !slices.Contains(got, "a bold word") || !slices.Contains(got, "• one") {
		t.Errorf("got %q", got)
	}
}

func TestDeepListsStayInProportion(t *testing.T) {
	src := strings.Repeat("<ul>", 10000) + strings.Repeat("<li>x</li>", 50000)
	if got := len(htmlToMarkdown(src)); got > len(src)*4 {
		t.Errorf("%d bytes became %d", len(src), got)
	}
}

func TestEscapeSequencesGo(t *testing.T) {
	for _, l := range plain(Render("a \x1b[2Jb\n```\n\x1b]0;x\x07c\n```", 30)) {
		if strings.ContainsAny(l, "\x1b\x07") {
			t.Errorf("got %q", l)
		}
	}
}

func FuzzRenderFits(f *testing.F) {
	for _, s := range []string{"# a", "- [x] b", "| c | d |\n|---|---|\n| 日本語 | e |", "```go\nf\n```", "> 日本語（かっこ）。", "![a](b)", "**", "`", "<img"} {
		f.Add(s, 20)
	}
	f.Fuzz(func(t *testing.T, s string, w int) {
		if w < 12 || w > 200 {
			return
		}
		for _, l := range plain(Render(s, w)) {
			if got := text.Width(l); got > w {
				t.Errorf("width %d: %d wide: %q", w, got, l)
			}
		}
	})
}
