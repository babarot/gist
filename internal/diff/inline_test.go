package diff

import "testing"

func TestInline(t *testing.T) {
	marked := func(s string, spans []Span) []string {
		var out []string
		for _, sp := range spans {
			out = append(out, s[sp.Start:sp.End])
		}
		return out
	}
	for _, c := range []struct {
		old, new           string
		oldMarks, newMarks []string
	}{
		// One word
		{"return a + b", "return a - b", []string{"+"}, []string{"-"}},
		// Two apart: only they, not what is between
		{"the quick brown fox jumps", "the slow brown fox runs", []string{"quick", "jumps"}, []string{"slow", "runs"}},
		// Words replaced together, with the space between
		{"see the old page now", "see a new one now", []string{"the old page"}, []string{"a new one"}},
		// Added only
		{"func main() {", "func main(args []string) {", nil, []string{"args []string"}},
		// Japanese, without spaces, by its characters
		{"ファイルを開く", "ファイルを閉じる", []string{"開く"}, []string{"閉じる"}},
		{"設定を保存する", "設定を読み込む", []string{"保存する"}, []string{"読み込む"}},
		// Too different to mark
		{"package main", "import \"fmt\"", nil, nil},
		// The same
		{"x := 1", "x := 1", nil, nil},
	} {
		o, n := Inline(c.old, c.new)
		if got := marked(c.old, o); !equal(got, c.oldMarks) {
			t.Errorf("%q -> %q: old marks %q, want %q", c.old, c.new, got, c.oldMarks)
		}
		if got := marked(c.new, n); !equal(got, c.newMarks) {
			t.Errorf("%q -> %q: new marks %q, want %q", c.old, c.new, got, c.newMarks)
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
