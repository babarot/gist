package recording

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/babarot/gist/v2/internal/fakegist"
)

// shown is how many releases the recording has: the one installed, and the one
// before it to scroll to.
const shown = 2

// changelogHeading is the heading tagpr writes for a release in CHANGELOG.md.
var changelogHeading = regexp.MustCompile(`(?m)^## \[(v\d+\.\d+\.\d+)\]\([^)]*\) - (\d{4}-\d{2}-\d{2})[ \t]*$`)

// ReadChangelog reads the notes of version, and of the release before it, from
// the CHANGELOG.md at path.
func ReadChangelog(path, version string) ([]fakegist.Release, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseChangelog(string(b), version)
}

// ParseChangelog makes the release version of a CHANGELOG.md, and the release
// before it, into releases as GitHub serves them, with the body in the form
// GitHub generates.
func ParseChangelog(text, version string) ([]fakegist.Release, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	heads := changelogHeading.FindAllStringSubmatchIndex(text, -1)
	at := slices.IndexFunc(heads, func(h []int) bool { return text[h[2]:h[3]] == "v"+version })
	if at < 0 {
		return nil, fmt.Errorf("changelog has no release v%s", version)
	}
	heads = heads[at:]
	if len(heads) < shown {
		return nil, fmt.Errorf("changelog has no release before v%s", version)
	}
	heads = heads[:shown]
	var out []fakegist.Release
	for _, h := range heads {
		published, err := time.Parse(time.DateOnly, text[h[4]:h[5]])
		if err != nil {
			return nil, err
		}
		end := len(text)
		if next := changelogHeading.FindStringIndex(text[h[1]:]); next != nil {
			end = h[1] + next[0]
		}
		section := strings.TrimSpace(text[h[1]:end])
		if section == "" {
			return nil, fmt.Errorf("release %s has no notes", text[h[2]:h[3]])
		}
		var body strings.Builder
		body.WriteString("## What's Changed\n")
		for line := range strings.SplitSeq(section, "\n") {
			if rest, ok := strings.CutPrefix(line, "- "); ok {
				line = "* " + rest
			}
			body.WriteString(line)
			body.WriteByte('\n')
		}
		out = append(out, fakegist.Release{TagName: text[h[2]:h[3]], PublishedAt: published, Body: body.String()})
	}
	return out, nil
}
