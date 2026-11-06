package text

import "testing"

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
