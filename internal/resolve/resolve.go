// Package resolve turns what the user calls a gist into its ID.
package resolve

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/babarot/gist/v2/internal/model"
)

// minPrefix is the shortest start of an ID taken for the ID.
const minPrefix = 4

var (
	fullID  = regexp.MustCompile(`^[0-9a-f]{32}$`)
	gistURL = regexp.MustCompile(`^https?://gist\.github\.com/(?:[^/]+/)?([0-9a-f]+)(?:\.git)?(?:[/#?].*)?$`)
)

// Direct gives the ID when arg says it by itself, as a whole ID or a URL,
// with no need to look in a list.
func Direct(arg string) (string, bool) {
	if fullID.MatchString(arg) {
		return arg, true
	}
	if m := gistURL.FindStringSubmatch(arg); m != nil {
		return m[1], true
	}
	return "", false
}

// NotFoundError says no gist matches.
type NotFoundError struct {
	Arg string
}

func (e *NotFoundError) Error() string { return fmt.Sprintf("no gist matches %q", e.Arg) }

// AmbiguousError says several gists match, which it names.
type AmbiguousError struct {
	Arg        string
	Candidates []model.Gist
}

func (e *AmbiguousError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%q matches %d gists:", e.Arg, len(e.Candidates))
	for _, g := range e.Candidates {
		fmt.Fprintf(&b, "\n  %s  %s", g.ID, oneLine(g.Description))
	}
	return b.String()
}

// oneLine keeps a description from ending the line it is printed on, or from
// reaching the terminal as an escape sequence.
func oneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return ' '
		}
		return r
	}, s)
}

// Find looks for arg in gists: an ID, the start of an ID, then a file name.
// The first of these that names one gist decides.
func Find(arg string, gists []model.Gist) (string, error) {
	if id, ok := Direct(arg); ok {
		return id, nil
	}
	var byPrefix, byFile []model.Gist
	for _, g := range gists {
		if g.ID == arg {
			return g.ID, nil
		}
		if len(arg) >= minPrefix && strings.HasPrefix(g.ID, arg) {
			byPrefix = append(byPrefix, g)
		}
		for _, f := range g.Files {
			if f.Name == arg {
				byFile = append(byFile, g)
				break
			}
		}
	}
	for _, found := range [][]model.Gist{byPrefix, byFile} {
		switch len(found) {
		case 0:
		case 1:
			return found[0].ID, nil
		default:
			return "", &AmbiguousError{Arg: arg, Candidates: found}
		}
	}
	return "", &NotFoundError{Arg: arg}
}
