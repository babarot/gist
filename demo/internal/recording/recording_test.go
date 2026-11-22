package recording

import (
	"context"
	"testing"
	"time"

	"github.com/babarot/gist/v2/internal/fakegist"
	"github.com/babarot/gist/v2/internal/github"
	"github.com/babarot/gist/v2/internal/logging"
	"github.com/babarot/gist/v2/internal/release"
	"github.com/babarot/gist/v2/internal/version"
)

func TestTheMadeUpGitHubAnswers(t *testing.T) {
	rel, err := ReadChangelog("../../../CHANGELOG.md", version.Version)
	if err != nil {
		t.Fatal(err)
	}
	srv := fakegist.New(t)
	now := time.Now()
	Fill(srv, now, rel)
	c, err := github.New(logging.Discard(), github.Options{Token: "demo", Transport: srv.Transport()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	login, err := c.Viewer(ctx)
	if err != nil || login != Login {
		t.Fatalf("%q, %v", login, err)
	}
	starred, err := c.Starred(ctx)
	if err != nil || len(starred) != 3 {
		t.Errorf("%d starred, %v", len(starred), err)
	}
	got, err := c.Releases(ctx)
	if err != nil || len(got) != shown || got[0].Version != version.Version {
		t.Errorf("%+v, %v", got, err)
	}
}

func TestTheChangelogBecomesReleases(t *testing.T) {
	text := "# Changelog\n\n" +
		"## [v2.2.0](https://github.com/babarot/gist/compare/v2.1.0...v2.2.0) - 2026-11-14\n" +
		"### New Features\n" +
		"- Select gists by @babarot in https://github.com/babarot/gist/pull/56\n\n" +
		"## [v2.1.0](https://github.com/babarot/gist/compare/v2.0.0...v2.1.0) - 2026-11-07\n" +
		"### Bug fixes\n" +
		"- Bring back --version by @babarot in https://github.com/babarot/gist/pull/47\n\n" +
		"## [v2.0.0](https://github.com/babarot/gist/compare/v1.2.8...v2.0.0) - 2026-11-07\n" +
		"- Not shown\n"
	rel, err := ParseChangelog(text, "2.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(rel) != 2 || rel[0].TagName != "v2.1.0" || rel[1].TagName != "v2.0.0" {
		t.Fatalf("%+v", rel)
	}
	rel, err = ParseChangelog(text, "2.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(rel) != 2 || rel[0].TagName != "v2.2.0" || rel[1].TagName != "v2.1.0" {
		t.Fatalf("%+v", rel)
	}
	if want := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC); !rel[0].PublishedAt.Equal(want) {
		t.Errorf("published %v, want %v", rel[0].PublishedAt, want)
	}
	if got, want := release.Notes(rel[0].Body), "### New Features\n* Select gists (#56)"; got != want {
		t.Errorf("notes %q, want %q", got, want)
	}
	if got, want := release.Notes(rel[1].Body), "### Bug fixes\n* Bring back --version (#47)"; got != want {
		t.Errorf("notes %q, want %q", got, want)
	}
}

func TestTheChangelogMustHaveTheVersionAndOneBefore(t *testing.T) {
	text := "## [v2.2.0](https://github.com/babarot/gist/compare/v2.1.0...v2.2.0) - 2026-11-14\n- One\n"
	for _, v := range []string{"2.3.0", "2.2.0"} {
		if _, err := ParseChangelog(text, v); err == nil {
			t.Errorf("%s: no error", v)
		}
	}
}
