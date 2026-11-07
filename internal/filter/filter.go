// Package filter narrows the list of gists: words found in order in a gist's
// description or file names, and key:value terms that look in one thing only:
// file:, desc:, vis:, lang:, owner:, size: and updated:.
package filter

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The keys, with their colon.
const (
	FileKey    = "file:"
	DescKey    = "desc:"
	VisKey     = "vis:"
	LangKey    = "lang:"
	OwnerKey   = "owner:"
	SizeKey    = "size:"
	UpdatedKey = "updated:"
)

// Keys are the keys.
var Keys = []string{FileKey, DescKey, VisKey, LangKey, OwnerKey, SizeKey, UpdatedKey}

// Query is the parsed filter, lower-cased.
type Query struct {
	Words []string
	// File and Desc are parts of a file name or the description, Vis is secret
	// or public, Lang part of a file's language, Owner part of the owner's
	// login. Several of one key match any of them.
	File, Desc, Vis, Lang, Owner []string
	// Size are bounds on the size of a gist's files together and Updated on how
	// long ago it changed, in nanoseconds. All of them must hold, so that two
	// make a range.
	Size, Updated []Bound
}

// Bound is a bound on a number, such as >200kb.
type Bound struct {
	// Op is >, >=, < or <=.
	Op    string
	Value float64
}

func (b Bound) holds(n float64) bool {
	switch b.Op {
	case ">":
		return n > b.Value
	case ">=":
		return n >= b.Value
	case "<":
		return n < b.Value
	}
	return n <= b.Value
}

// boundKey is a key that takes a bound: what its units are worth, and what the
// ghost says at each step of typing one. Without "" among units, a number needs
// a unit.
type boundKey struct {
	units                        map[string]float64
	start, number, unit, invalid string
}

const day = 24 * time.Hour

// boundKeys are the keys that take a bound. Sizes are of 1024 and ages of 30 and
// 365 days, as the list shows them.
var boundKeys = map[string]boundKey{
	SizeKey: {
		units:   map[string]float64{"": 1, "b": 1, "k": 1 << 10, "kb": 1 << 10, "m": 1 << 20, "mb": 1 << 20},
		start:   ">200KB, <1MB, also >= and <=",
		number:  "200KB",
		unit:    " bytes, or KB, MB",
		invalid: "  not a size: try size:>200KB",
	},
	UpdatedKey: {
		units: map[string]float64{
			"m": float64(time.Minute), "h": float64(time.Hour), "d": float64(day), "w": float64(7 * day),
			"mo": float64(30 * day), "y": float64(365 * day),
		},
		start:   "<7d within 7 days, >1y over a year ago",
		number:  "7d",
		unit:    " m, h, d, w, mo or y",
		invalid: "  not an age: try updated:<7d",
	},
}

// boundTerm is a value of a key that takes a bound, lower-cased: an operator, a
// number and a unit.
var boundTerm = regexp.MustCompile(`^(>=|<=|>|<)(\d+\.?\d*|\.\d+)([a-z]*)$`)

// parseBound reads a value of k. typing says it is not whole yet, so it is still
// being typed; with neither ok nor typing, it is wrong.
func (k boundKey) parseBound(v string) (b Bound, ok, typing bool) {
	if v == "" || isOp(v) {
		return Bound{}, false, true
	}
	m := boundTerm.FindStringSubmatch(v)
	if m == nil {
		return Bound{}, false, false
	}
	unit, known := k.units[m[3]]
	if !known {
		return Bound{}, false, m[3] == ""
	}
	n, err := strconv.ParseFloat(m[2], 64)
	if err != nil {
		return Bound{}, false, false
	}
	return Bound{m[1], n * unit}, true, false
}

func isOp(v string) bool { return v == ">" || v == "<" || v == ">=" || v == "<=" }

// BoundHint says how to go on with word, a term with a bound being typed, or why
// it cannot be read; "" for any other word and one with nothing to add. What it
// shows is an example, not a value to fill in: a bound has none to offer.
func BoundHint(word string) string {
	key, v, found := strings.Cut(strings.ToLower(word), ":")
	k, known := boundKeys[key+":"]
	if !found || !known {
		return ""
	}
	_, ok, typing := k.parseBound(v)
	switch {
	case v == "":
		return k.start
	case isOp(v):
		return k.number
	case !ok && !typing:
		return k.invalid
	}
	if c := v[len(v)-1]; c == '.' || c >= '0' && c <= '9' {
		return k.unit
	}
	return ""
}

// Parse reads a filter. A word with an unknown key, such as a URL, is a plain
// word; a known key with nothing after it is nothing. A bound that cannot be
// read is a plain word too, which leaves no rows rather than all of them.
func Parse(s string) Query {
	var q Query
	fields := map[string]*[]string{FileKey: &q.File, DescKey: &q.Desc, VisKey: &q.Vis, LangKey: &q.Lang, OwnerKey: &q.Owner}
	bounds := map[string]*[]Bound{SizeKey: &q.Size, UpdatedKey: &q.Updated}
	for w := range strings.FieldsSeq(strings.ToLower(s)) {
		if key, v, ok := strings.Cut(w, ":"); ok {
			if k, known := boundKeys[key+":"]; known {
				b, ok, typing := k.parseBound(v)
				if ok {
					*bounds[key+":"] = append(*bounds[key+":"], b)
				}
				if ok || typing {
					continue
				}
			}
			if dst, known := fields[key+":"]; known {
				if v != "" {
					*dst = append(*dst, v)
				}
				continue
			}
		}
		q.Words = append(q.Words, w)
	}
	return q
}

// LastWord is the word query ends with, "" after a space.
func LastWord(query string) string {
	i := strings.LastIndexFunc(query, unicode.IsSpace)
	if i < 0 {
		return query
	}
	_, n := utf8.DecodeRuneInString(query[i:])
	return query[i+n:]
}

// KeyRest is what is left to type of the key the last word of query starts,
// with its colon, once that word has two letters and fits one key only. A word
// of a description often starts like a key, so one letter is too little.
func KeyRest(query string) string {
	w := strings.ToLower(LastWord(query))
	if utf8.RuneCountInString(w) < 2 || strings.Contains(w, ":") {
		return ""
	}
	rest, found := "", 0
	for _, k := range Keys {
		if strings.HasPrefix(k, w) {
			rest = k[len(w):]
			found++
		}
	}
	if found != 1 {
		return ""
	}
	return rest
}

// ValueTerm says word is a key whose values can be listed, with what it has of
// a value so far. A description has none: it is whatever was written.
func ValueTerm(word string) (key, value string, ok bool) {
	k, v, found := strings.Cut(word, ":")
	key = strings.ToLower(k) + ":"
	if !found || !slices.Contains([]string{FileKey, VisKey, LangKey, OwnerKey}, key) {
		return "", "", false
	}
	return key, v, true
}

// Empty says the query narrows nothing.
func (q Query) Empty() bool {
	return len(q.Words)+len(q.File)+len(q.Desc)+len(q.Vis)+len(q.Lang)+len(q.Owner)+len(q.Size)+len(q.Updated) == 0
}

// Item is what is looked in.
type Item struct {
	Description string
	Files       []string
	Languages   []string
	Public      bool
	Owner       string
	// Size is the size of the files together, and Age how long ago it changed.
	Size int64
	Age  time.Duration
}

// Hits are the runes that matched, to show. Each is sorted.
type Hits struct {
	Description []int
	// File is of the first file name.
	File []int
}

// Match says whether it matches and what did.
func Match(q Query, it Item) (Hits, bool) {
	var h Hits
	lowDesc := strings.ToLower(it.Description)
	for _, w := range q.Words {
		ok := false
		if _, hits, m := Fuzzy(w, it.Description); m && it.Description != "" {
			h.Description, ok = merge(h.Description, hits), true
		}
		for i, f := range it.Files {
			if _, hits, m := Fuzzy(w, f); m {
				ok = true
				if i == 0 {
					h.File = merge(h.File, hits)
				}
			}
		}
		if !ok {
			return Hits{}, false
		}
	}
	if len(q.Desc) > 0 && !slices.ContainsFunc(q.Desc, func(d string) bool { return strings.Contains(lowDesc, d) }) {
		return Hits{}, false
	}
	if len(q.File) > 0 && !anyContains(it.Files, q.File) {
		return Hits{}, false
	}
	if len(q.Lang) > 0 && !anyContains(it.Languages, q.Lang) {
		return Hits{}, false
	}
	if len(q.Owner) > 0 && !anyContains([]string{it.Owner}, q.Owner) {
		return Hits{}, false
	}
	vis := "secret"
	if it.Public {
		vis = "public"
	}
	if len(q.Vis) > 0 && !slices.Contains(q.Vis, vis) {
		return Hits{}, false
	}
	if !allHold(q.Size, float64(it.Size)) || !allHold(q.Updated, float64(it.Age)) {
		return Hits{}, false
	}
	return h, true
}

func allHold(bounds []Bound, n float64) bool {
	return !slices.ContainsFunc(bounds, func(b Bound) bool { return !b.holds(n) })
}

// anyContains says one of values has one of parts in it.
func anyContains(values, parts []string) bool {
	return slices.ContainsFunc(values, func(v string) bool {
		v = strings.ToLower(v)
		return slices.ContainsFunc(parts, func(p string) bool { return strings.Contains(v, p) })
	})
}

func merge(a, b []int) []int {
	out := append(slices.Clone(a), b...)
	slices.Sort(out)
	return slices.Compact(out)
}
