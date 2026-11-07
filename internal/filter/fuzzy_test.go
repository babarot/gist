package filter

import (
	"reflect"
	"slices"
	"testing"
)

func TestFuzzy(t *testing.T) {
	cases := []struct {
		pattern, s string
		ok         bool
		hits       []int
	}{
		{"dot", "babarot/dotfiles", true, []int{8, 9, 10}},
		{"bdot", "babarot/dotfiles", true, []int{0, 8, 9, 10}},
		{"INFRA", "10xinc/infrastructure", true, []int{7, 8, 9, 10, 11}},
		{"app", "a/b/app", true, []int{4, 5, 6}},
		{"ss", "10xinc/stailer-server", true, []int{7, 15}}, // word starts over the s right after
		{"xyz", "babarot/dotfiles", false, nil},
		{"", "anything", true, nil},
	}
	for _, c := range cases {
		_, hits, ok := Fuzzy(c.pattern, c.s)
		if ok != c.ok || !reflect.DeepEqual(hits, c.hits) {
			t.Errorf("Fuzzy(%q, %q) = %v %v, want %v %v", c.pattern, c.s, hits, ok, c.hits, c.ok)
		}
	}
	// A run of word starts beats the same letters scattered.
	a, _, _ := Fuzzy("st", "10xinc/stailer")
	b, _, _ := Fuzzy("st", "10xinc/last-thing")
	if a <= b {
		t.Errorf("st: stailer %d should beat last-thing %d", a, b)
	}
}

func TestAHitMovesToAWordStartWhenTheRestStillFits(t *testing.T) {
	for _, tt := range []struct {
		pattern, s string
		want       []int
	}{
		// The b of xb gives way to the one starting a word
		{"abc", "a xb b c", []int{0, 5, 7}},
		// Here the word's b comes after the c, so xb stays
		{"abcd", "a xb c b d", []int{0, 3, 5, 9}},
	} {
		_, hits, ok := Fuzzy(tt.pattern, tt.s)
		if !ok || !slices.Equal(hits, tt.want) {
			t.Errorf("%q in %q: %v, %v", tt.pattern, tt.s, hits, ok)
		}
	}
}
