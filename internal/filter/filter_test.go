package filter

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	for in, want := range map[string]Query{
		"":                       {},
		"foo Bar":                {Words: []string{"foo", "bar"}},
		"file:a.go desc:x":       {File: []string{"a.go"}, Desc: []string{"x"}},
		"vis:Secret lang:go foo": {Words: []string{"foo"}, Vis: []string{"secret"}, Lang: []string{"go"}},
		"file:":                  {},
		"http://x":               {Words: []string{"http://x"}},
		"nope:x":                 {Words: []string{"nope:x"}},
		"file:a file:b":          {File: []string{"a", "b"}},
	} {
		if got := Parse(in); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %+v", in, got)
		}
	}
}

func TestEmpty(t *testing.T) {
	if !Parse("file:").Empty() || Parse("x").Empty() || Parse("vis:public").Empty() {
		t.Error("wrong")
	}
}

var item = Item{
	Description: "retry helper 日本語の説明",
	Files:       []string{"main.go", "README.md"},
	Languages:   []string{"Go", "Markdown"},
	Public:      false,
}

func TestMatch(t *testing.T) {
	for _, tt := range []struct {
		q    string
		want bool
	}{
		{"", true},
		{"retry", true},
		{"rtry", true},
		{"RETRY", true},
		{"日本語", true},
		{"main", true},
		{"readme", true},
		{"retry main", true},
		{"retry nothing", false},
		{"zzz", false},
		{"file:main", true},
		{"file:retry", false},
		{"desc:retry", true},
		{"desc:main", false},
		{"vis:secret", true},
		{"vis:public", false},
		{"lang:go", true},
		{"lang:markdown", true},
		{"lang:ruby", false},
		{"file:zzz file:main", true},
		{"retry vis:public", false},
		{"nope:x", false},
	} {
		if _, got := Match(Parse(tt.q), item); got != tt.want {
			t.Errorf("%q: got %v", tt.q, got)
		}
	}
}

func TestPublic(t *testing.T) {
	if _, ok := Match(Parse("vis:public"), Item{Public: true}); !ok {
		t.Error("a public gist did not match")
	}
}

func TestHits(t *testing.T) {
	h, ok := Match(Parse("ret mai"), item)
	if !ok {
		t.Fatal("no match")
	}
	if !reflect.DeepEqual(h.Description[:3], []int{0, 1, 2}) {
		t.Errorf("description hits are %v", h.Description)
	}
	if !reflect.DeepEqual(h.File, []int{0, 1, 2}) {
		t.Errorf("file hits are %v", h.File)
	}
}

func TestAWordOnlyInALaterFileHasNoHitToShow(t *testing.T) {
	h, ok := Match(Parse("readme"), Item{Files: []string{"main.go", "README.md"}})
	if !ok || h.File != nil {
		t.Errorf("got %+v, %v", h, ok)
	}
}

func TestKeyRest(t *testing.T) {
	for query, want := range map[string]string{
		"":           "",
		"f":          "", // too little to tell from a word
		"fi":         "le:",
		"FIL":        "e:",
		"file":       ":",
		"file:":      "",
		"go de":      "sc:",
		"la ":        "",
		"日本語":        "",
		"go\u3000la": "ng:",
		"xy":         "",
	} {
		if got := KeyRest(query); got != want {
			t.Errorf("%q: got %q, want %q", query, got, want)
		}
	}
}

func TestValueTerm(t *testing.T) {
	for _, tt := range []struct {
		word, key, value string
		ok               bool
	}{
		{"lang:go", LangKey, "go", true},
		{"VIS:", VisKey, "", true},
		{"file:a:b", FileKey, "a:b", true},
		{"desc:x", "", "", false},
		{"http://a", "", "", false},
		{"plain", "", "", false},
	} {
		key, value, ok := ValueTerm(tt.word)
		if key != tt.key || value != tt.value || ok != tt.ok {
			t.Errorf("%q: got %q %q %v", tt.word, key, value, ok)
		}
	}
}
