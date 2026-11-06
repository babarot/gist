package filter

import (
	"strings"
	"unicode"
)

// Fuzzy reports whether the runes of pattern appear in s in order,
// ignoring case, and scores the match: consecutive runes and runes at the
// start of a word (after / - _ . or a space) count more, gaps count
// against it. hits are the rune indexes in s that matched.
//
// It takes the first place the whole pattern fits, then the latest start
// that still fits before that end, so "app" in "a/b/app" matches the word
// rather than the stray a, moved back to a word start if one has the
// first rune ("bdot" in "babarot/dotfiles" starts at the first b).
func Fuzzy(pattern, s string) (score int, hits []int, ok bool) {
	p := []rune(strings.ToLower(pattern))
	r := []rune(strings.ToLower(s))
	if len(p) == 0 {
		return 0, nil, true
	}
	// Forward: where the first full match ends.
	end, j := -1, 0
	for i, c := range r {
		if c == p[j] {
			if j++; j == len(p) {
				end = i
				break
			}
		}
	}
	if end < 0 {
		return 0, nil, false
	}
	// Backward from that end: the tightest start.
	start, j := end, len(p)-1
	for i := end; i >= 0; i-- {
		if r[i] == p[j] {
			if j--; j < 0 {
				start = i
				break
			}
		}
	}
	// The first rune at the start of a word, when one comes earlier.
	for i := start - 1; i >= 0 && !boundary(r, start); i-- {
		if r[i] == p[0] && boundary(r, i) {
			start = i
		}
	}
	// Forward again from the start, preferring word starts when a later
	// one still leaves room for the rest of the pattern.
	j = 0
	for i := start; i <= end && j < len(p); i++ {
		if r[i] != p[j] {
			continue
		}
		if !boundary(r, i) {
			if k := nextBoundary(r, p[j], i+1, end); k > 0 && fits(r, p[j+1:], k+1, end) {
				i = k
			}
		}
		hits = append(hits, i)
		j++
	}
	score = 16 * len(p)
	for n, i := range hits {
		if boundary(r, i) {
			score += 8
		}
		if n > 0 {
			if gap := i - hits[n-1] - 1; gap == 0 {
				score += 8
			} else {
				score -= min(gap, 8)
			}
		}
	}
	score -= min(hits[0], 8) / 2
	return score, hits, true
}

// boundary reports whether r[i] starts a word.
func boundary(r []rune, i int) bool {
	if i == 0 {
		return true
	}
	prev := r[i-1]
	return prev == '/' || prev == '-' || prev == '_' || prev == '.' || unicode.IsSpace(prev)
}

// nextBoundary finds c at a word start in r[from:to], or -1.
func nextBoundary(r []rune, c rune, from, to int) int {
	for i := from; i <= to && i < len(r); i++ {
		if r[i] == c && boundary(r, i) {
			return i
		}
	}
	return -1
}

// fits reports whether p appears in order in r[from:to].
func fits(r []rune, p []rune, from, to int) bool {
	j := 0
	for i := from; i <= to && i < len(r) && j < len(p); i++ {
		if r[i] == p[j] {
			j++
		}
	}
	return j == len(p)
}
