package text

import (
	"slices"
	"testing"
)

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"plain":                     "plain",
		"a\tb":                      "a    b",
		"one\ntwo":                  "one\ntwo",
		"\x1b[31mred\x1b[0m":        "red",
		"\x1b]0;title\x07after":     "after",
		"bell\x07 del\x7f c1\u0085": "bell del c1",
		"carriage\rreturn":          "carriagereturn",
		"日本語":                       "日本語",
	} {
		if got := Clean(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestLine(t *testing.T) {
	if got := Line("one\ntwo\tthree"); got != "one two    three" {
		t.Errorf("got %q", got)
	}
}

func TestWidth(t *testing.T) {
	for in, want := range map[string]int{
		"abc": 3,
		"日本語": 6,
		"❤️":  2,
		"é":  1,
		"\U0001F468\u200d\U0001F469\u200d\U0001F466": 2,
		"": 0,
	} {
		if got := Width(in); got != want {
			t.Errorf("%q: got %d, want %d", in, got, want)
		}
	}
}

func TestBreaks(t *testing.T) {
	rows := func(s string, width int) []string {
		starts := Breaks(s, width)
		var out []string
		for i, from := range starts {
			to := len(s)
			if i+1 < len(starts) {
				to = starts[i+1]
			}
			out = append(out, s[from:to])
		}
		return out
	}
	for _, tt := range []struct {
		s     string
		width int
		want  []string
	}{
		{"", 10, []string{""}},
		{"short", 10, []string{"short"}},
		{"exactly10!", 10, []string{"exactly10!"}},
		{"one two three", 8, []string{"one two ", "three"}},
		{"日本語のテキスト", 6, []string{"日本語", "のテキ", "スト"}},
		{"日本語", 5, []string{"日本", "語"}},
		{"aあいう", 4, []string{"aあ", "いう"}},
		{"abcdefgh", 3, []string{"abc", "def", "gh"}},
		{"a b日本", 4, []string{"a ", "b日", "本"}},
		{"日本語", 1, []string{"日", "本", "語"}}, // nothing fits: one each, not forever
		{"abc", 0, []string{"a", "b", "c"}},
	} {
		got := rows(tt.s, tt.width)
		if !slices.Equal(got, tt.want) {
			t.Errorf("%q in %d: got %q, want %q", tt.s, tt.width, got, tt.want)
		}
	}
}
