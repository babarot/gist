package diff

import (
	"slices"
	"unicode"
	"unicode/utf8"
)

// Span is a part of a line, by byte offsets.
type Span struct{ Start, End int }

const (
	// maxInlineCells bounds the work of comparing two lines, the product
	// of their numbers of words; longer lines get no words marked.
	maxInlineCells = 200_000
	// minShared is how much of two lines must be the same, as a share of
	// the longer one, for the words that differ to be marked. Lines more
	// different than that are told apart well enough by their color.
	minShared = 0.4
)

// Inline finds the words that differ between a line deleted and the line
// added in its place, as the spans of each to mark; none when the lines
// have too little in common.
//
// It compares words by their longest common subsequence. Should marks
// read poorly, github.com/sergi/go-diff is the one to compare it with:
// its DiffCleanupSemantic moves the edges of what changed to where a
// reader expects them, which this does not do.
func Inline(old, new string) (oldSpans, newSpans []Span) {
	a, b := words(old), words(new)
	// The same words at both ends need no comparing
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre].text == b[pre].text {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf].text == b[len(b)-1-suf].text {
		suf++
	}
	ma, mb := a[pre:len(a)-suf], b[pre:len(b)-suf]
	if len(ma)*len(mb) > maxInlineCells {
		return nil, nil
	}
	keepA, keepB := lcs(ma, mb)

	shared := 0
	for _, w := range a[:pre] {
		shared += len(w.text)
	}
	for _, w := range a[len(a)-suf:] {
		shared += len(w.text)
	}
	for i, keep := range keepA {
		if keep {
			shared += len(ma[i].text)
		}
	}
	if float64(shared) < minShared*float64(max(len(old), len(new))) {
		return nil, nil
	}
	return spans(ma, keepA), spans(mb, keepB)
}

type word struct {
	text  string
	start int
}

// words splits a line into words, runs of spaces, and single other
// characters. A character of a script written without spaces, as
// Japanese is, is a word of its own.
func words(s string) []word {
	var out []word
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		j := i + n
		switch {
		case unspaced(r):
		case isWord(r):
			for j < len(s) {
				r, n := utf8.DecodeRuneInString(s[j:])
				if !isWord(r) || unspaced(r) {
					break
				}
				j += n
			}
		case unicode.IsSpace(r):
			for j < len(s) {
				r, n := utf8.DecodeRuneInString(s[j:])
				if !unicode.IsSpace(r) {
					break
				}
				j += n
			}
		}
		out = append(out, word{s[i:j], i})
		i = j
	}
	return out
}

func isWord(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func unspaced(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul, unicode.Thai)
}

// lcs tells which words of a and of b are in their longest common
// subsequence.
func lcs(a, b []word) (keepA, keepB []bool) {
	keepA, keepB = make([]bool, len(a)), make([]bool, len(b))
	// n[i][j] is the length of the subsequence of a[i:] and b[j:]
	w := len(b) + 1
	n := make([]int, (len(a)+1)*w)
	for i, x := range slices.Backward(a) {
		for j, y := range slices.Backward(b) {
			if x.text == y.text {
				n[i*w+j] = n[(i+1)*w+j+1] + 1
			} else {
				n[i*w+j] = max(n[(i+1)*w+j], n[i*w+j+1])
			}
		}
	}
	for i, j := 0, 0; i < len(a) && j < len(b); {
		switch {
		case a[i].text == b[j].text:
			keepA[i], keepB[j] = true, true
			i++
			j++
		case n[(i+1)*w+j] >= n[i*w+j+1]:
			i++
		default:
			j++
		}
	}
	return keepA, keepB
}

// spans joins the words not kept into spans. Spaces kept between two of
// them are taken in, so that "a b" replaced reads as one change.
func spans(ws []word, keep []bool) []Span {
	var out []Span
	for i := range ws {
		if keep[i] {
			continue
		}
		start, end := ws[i].start, ws[i].start+len(ws[i].text)
		if k := len(out) - 1; k >= 0 && onlySpaces(ws, keep, out[k].End, start) {
			out[k].End = end
		} else {
			out = append(out, Span{start, end})
		}
	}
	return out
}

// onlySpaces tells whether the words from byte from to byte to are all
// spaces.
func onlySpaces(ws []word, keep []bool, from, to int) bool {
	for i, w := range ws {
		if w.start >= from && w.start < to {
			if !keep[i] {
				continue
			}
			for _, r := range w.text {
				if !unicode.IsSpace(r) {
					return false
				}
			}
		}
	}
	return true
}
