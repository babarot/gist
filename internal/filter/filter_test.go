package filter

import (
	"reflect"
	"testing"
	"time"
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

func TestSize(t *testing.T) {
	const kb = 1 << 10
	tests := []struct {
		query string
		size  int64
		want  bool
	}{
		{"size:>200kb", 200*kb + 1, true},
		{"size:>200KB", 200 * kb, false},
		{"size:>=200kb", 200 * kb, true},
		{"size:<1k", kb - 1, true},
		{"size:<1k", kb, false},
		{"size:<=1k", kb, true},
		{"size:>1.5m", 2 * 1 << 20, true},
		{"size:>1.5mb", 1 << 20, false},
		{"size:>100", 101, true},
		{"size:>100b", 100, false},
		{"size:>1k size:<2k", 1500, true},
		{"size:>1k size:<2k", 3 * kb, false},
		{"retry size:>1k", 2 * kb, true},
		{"nothing size:>1k", 2 * kb, false},
		// Still being typed: everything stays
		{"size:", 1, true},
		{"size:>", 1, true},
		{"size:>=", 1, true},
		// Wrong: nothing does
		{"size:big", 1 << 30, false},
		{"size:200kb", 200 * kb, false},
		{"size:>2gb", 1 << 40, false},
		{"size:>inf", 1 << 40, false},
	}
	for _, tt := range tests {
		it := Item{Description: "retry", Files: []string{"a.go"}, Size: tt.size}
		if _, got := Match(Parse(tt.query), it); got != tt.want {
			t.Errorf("%q on %d: %v", tt.query, tt.size, got)
		}
	}
}

func TestASizeKeepsTheQueryFromBeingEmpty(t *testing.T) {
	if Parse("size:>1k").Empty() {
		t.Error("a bound narrows nothing")
	}
	if !Parse("size:>").Empty() {
		t.Error("a bound being typed narrows something")
	}
}

func TestSizeCompletesItsKeyOnly(t *testing.T) {
	if got := KeyRest("si"); got != "ze:" {
		t.Errorf("KeyRest is %q", got)
	}
	if _, _, ok := ValueTerm("size:>1"); ok {
		t.Error("a size has values to list")
	}
}

func TestBoundHint(t *testing.T) {
	for word, want := range map[string]string{
		"size:":       ">200KB, <1MB, also >= and <=",
		"SIZE:":       ">200KB, <1MB, also >= and <=",
		"size:>":      "200KB",
		"size:<=":     "200KB",
		"size:>200":   " bytes, or KB, MB",
		"size:>1.":    " bytes, or KB, MB",
		"size:>200k":  "",
		"size:>200KB": "",
		"size:200kb":  "  not a size: try size:>200KB",
		"size:>2gb":   "  not a size: try size:>200KB",
		"updated:":    "<7d within 7 days, >1y over a year ago",
		"updated:<":   "7d",
		"updated:<7":  " m, h, d, w, mo or y",
		"updated:<7m": "",
		"updated:7d":  "  not an age: try updated:<7d",
		"updated:<7x": "  not an age: try updated:<7d",
		"file:a":      "",
		"retry":       "",
		"":            "",
	} {
		if got := BoundHint(word); got != want {
			t.Errorf("%q: %q", word, got)
		}
	}
}

func TestOwner(t *testing.T) {
	it := Item{Files: []string{"a.go"}, Owner: "babarot"}
	for query, want := range map[string]bool{
		"owner:babarot":        true,
		"owner:BABA":           true,
		"owner:alice":          false,
		"owner:alice owner:ba": true,
		"owner:":               true,
	} {
		if _, got := Match(Parse(query), it); got != want {
			t.Errorf("%q: %v", query, got)
		}
	}
	if _, got := Match(Parse("owner:x"), Item{Files: []string{"a.go"}}); got {
		t.Error("a gist with no owner matched")
	}
	if key, value, ok := ValueTerm("owner:ba"); key != OwnerKey || value != "ba" || !ok {
		t.Errorf("got %q %q %v", key, value, ok)
	}
	if got := KeyRest("ow"); got != "ner:" {
		t.Errorf("KeyRest is %q", got)
	}
}

func TestUpdated(t *testing.T) {
	const day = 24 * time.Hour
	for _, tt := range []struct {
		query string
		age   time.Duration
		want  bool
	}{
		{"updated:<7d", 6 * day, true},
		{"updated:<7d", 7 * day, false},
		{"updated:<=7d", 7 * day, true},
		{"updated:>1y", 366 * day, true},
		{"updated:>1y", 364 * day, false},
		{"updated:<30m", 29 * time.Minute, true},
		{"updated:<2h", 3 * time.Hour, false},
		{"updated:<1w", 6 * day, true},
		{"updated:<2mo", 59 * day, true},
		{"updated:<2mo", 61 * day, false},
		{"updated:>1d updated:<1w", 3 * day, true},
		{"updated:>1d updated:<1w", 8 * day, false},
		// Still being typed: everything stays
		{"updated:", day, true},
		{"updated:<", day, true},
		{"updated:<7", day * 100, true},
		// Wrong: nothing does
		{"updated:7d", 7 * day, false},
		{"updated:<7x", day, false},
	} {
		it := Item{Files: []string{"a.go"}, Age: tt.age}
		if _, got := Match(Parse(tt.query), it); got != tt.want {
			t.Errorf("%q at %v: %v", tt.query, tt.age, got)
		}
	}
	if Parse("updated:<7d").Empty() {
		t.Error("a bound narrows nothing")
	}
	if got := KeyRest("up"); got != "dated:" {
		t.Errorf("KeyRest is %q", got)
	}
}
