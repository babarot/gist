// Package filter narrows the list of gists: words found in order in a gist's
// description or file names, and key:value terms that look in one thing only:
// file:, desc:, vis: and lang:.
package filter

import (
	"slices"
	"strings"
)

// The keys, with their colon.
const (
	FileKey = "file:"
	DescKey = "desc:"
	VisKey  = "vis:"
	LangKey = "lang:"
)

// Query is the parsed filter, lower-cased.
type Query struct {
	Words []string
	// File and Desc are parts of a file name or the description, Vis is secret
	// or public, Lang part of a file's language. Several of one key match any
	// of them.
	File, Desc, Vis, Lang []string
}

// Parse reads a filter. A word with an unknown key, such as a URL, is a plain
// word; a known key with nothing after it is nothing.
func Parse(s string) Query {
	var q Query
	fields := map[string]*[]string{FileKey: &q.File, DescKey: &q.Desc, VisKey: &q.Vis, LangKey: &q.Lang}
	for w := range strings.FieldsSeq(strings.ToLower(s)) {
		if key, v, ok := strings.Cut(w, ":"); ok {
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

// Empty says the query narrows nothing.
func (q Query) Empty() bool {
	return len(q.Words)+len(q.File)+len(q.Desc)+len(q.Vis)+len(q.Lang) == 0
}

// Item is what is looked in.
type Item struct {
	Description string
	Files       []string
	Languages   []string
	Public      bool
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
	vis := "secret"
	if it.Public {
		vis = "public"
	}
	if len(q.Vis) > 0 && !slices.Contains(q.Vis, vis) {
		return Hits{}, false
	}
	return h, true
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
