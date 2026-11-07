// Package release tells whether a newer release of gist is out and how the
// running install is updated. It reads nothing from the network: internal/github
// does.
package release

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Page is where the releases are on the web.
const Page = "https://github.com/babarot/gist/releases"

// CheckEvery is how long a reading of the releases holds.
const CheckEvery = 24 * time.Hour

// Due says the releases are read again at now, after a reading at checked.
func Due(checked, now time.Time) bool {
	return checked.IsZero() || now.Sub(checked) >= CheckEvery || now.Before(checked)
}

// Compare compares two versions written as three numbers, with or without a
// leading v: -1 when a is older than b, 0 when they are the same, 1 when a is
// newer.
func Compare(a, b string) (int, error) {
	x, err := parse(a)
	if err != nil {
		return 0, err
	}
	y, err := parse(b)
	if err != nil {
		return 0, err
	}
	for i := range x {
		if x[i] != y[i] {
			if x[i] < y[i] {
				return -1, nil
			}
			return 1, nil
		}
	}
	return 0, nil
}

// Newer says a is a newer version than b. A version that does not parse is never
// newer, so a tag of another form offers nothing.
func Newer(a, b string) bool {
	c, err := Compare(a, b)
	return err == nil && c > 0
}

func parse(v string) ([3]int, error) {
	var n [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return n, fmt.Errorf("version %q is not three numbers", v)
	}
	for i, p := range parts {
		x, err := strconv.Atoi(p)
		if err != nil || x < 0 {
			return n, fmt.Errorf("version %q is not three numbers", v)
		}
		n[i] = x
	}
	return n, nil
}

// How says how the executable at exe, with its links followed, is updated.
func How(exe string) string {
	switch {
	case strings.HasPrefix(exe, "/nix/store/"):
		return "update it with Nix"
	case strings.Contains(exe, "/Cellar/"):
		return "brew upgrade babarot/tap/gist"
	}
	return Page
}

var (
	notesComment = regexp.MustCompile(`(?m)^<!--.*-->[ \t]*\n?`)
	notesHeading = regexp.MustCompile(`(?m)^## What's Changed[ \t]*\n?`)
	notesFull    = regexp.MustCompile(`(?m)^\*\*Full Changelog\*\*.*$`)
	notesBy      = regexp.MustCompile(` by @[\w-]+(?:\[bot\])? in https://github\.com/[\w.-]+/[\w.-]+/pull/(\d+)`)
)

// Notes is a release's notes as GitHub generated them, cut to what a reader of
// gist wants: the comment, the "What's Changed" heading and the link to the full
// changelog go, and "by @user in <pull request URL>" becomes its number.
func Notes(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = notesComment.ReplaceAllString(body, "")
	body = notesHeading.ReplaceAllString(body, "")
	body = notesFull.ReplaceAllString(body, "")
	body = notesBy.ReplaceAllString(body, " (#$1)")
	return strings.TrimSpace(body)
}
