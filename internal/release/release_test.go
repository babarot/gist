package release

import (
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		want bool
	}{
		{"2.0.1", "2.0.0", true},
		{"v2.1.0", "2.0.9", true},
		{"10.0.0", "9.9.9", true},
		{"2.0.0", "2.0.0", false},
		{"1.9.9", "2.0.0", false},
		{"nightly", "2.0.0", false},
		{"2.0", "1.0.0", false},
		{"", "1.0.0", false},
	} {
		if got := Newer(tt.a, tt.b); got != tt.want {
			t.Errorf("Newer(%q, %q) = %v", tt.a, tt.b, got)
		}
	}
}

func TestDue(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		checked time.Time
		want    bool
	}{
		{time.Time{}, true},
		{now.Add(-time.Hour), false},
		{now.Add(-24 * time.Hour), true},
		{now.Add(time.Hour), true}, // a clock set back
	} {
		if got := Due(tt.checked, now); got != tt.want {
			t.Errorf("%v: %v", tt.checked, got)
		}
	}
}

func TestHow(t *testing.T) {
	for exe, want := range map[string]string{
		"/nix/store/abc-gist/bin/gist":                  "update it with Nix",
		"/opt/homebrew/Cellar/gist/2.0.0/bin/gist":      "brew upgrade babarot/tap/gist",
		"/home/linuxbrew/.linuxbrew/Cellar/gist/2/gist": "brew upgrade babarot/tap/gist",
		"/usr/local/bin/gist":                           Page,
	} {
		if got := How(exe); got != want {
			t.Errorf("%s: got %q", exe, got)
		}
	}
}

func TestNotes(t *testing.T) {
	body := "<!-- Release notes generated -->\r\n## What's Changed\r\n* Add a thing by @alice in https://github.com/babarot/gist/pull/12\r\n\r\n**Full Changelog**: https://github.com/babarot/gist/compare/a...b\r\n"
	if got, want := Notes(body), "* Add a thing (#12)"; got != want {
		t.Errorf("got %q", got)
	}
}
