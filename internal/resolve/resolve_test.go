package resolve

import (
	"errors"
	"strings"
	"testing"

	"github.com/babarot/gist/v2/internal/model"
)

const id = "6065a5a0123456789abcdef012345678"

func gist(id, desc string, files ...string) model.Gist {
	g := model.Gist{ID: id, Description: desc}
	for _, f := range files {
		g.Files = append(g.Files, model.File{Name: f})
	}
	return g
}

var gists = []model.Gist{
	gist(id, "first", "deadlink", "00-index.md"),
	gist("6065b5a0123456789abcdef012345678", "second", "00-index.md"),
	gist("1234567", "an old gist", "old.txt"),
}

func TestFind(t *testing.T) {
	for _, tt := range []struct{ name, arg, want string }{
		{"whole ID, even if not listed", "ffffffffffffffffffffffffffffffff", "ffffffffffffffffffffffffffffffff"},
		{"old ID", "1234567", "1234567"},
		{"URL with a user", "https://gist.github.com/alice/" + id, id},
		{"URL without a user", "https://gist.github.com/" + id, id},
		{"URL of a clone", "https://gist.github.com/" + id + ".git", id},
		{"URL with a file", "https://gist.github.com/alice/" + id + "#file-a-md", id},
		{"prefix", "6065a", id},
		{"file name", "deadlink", id},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Find(tt.arg, gists)
			if err != nil || got != tt.want {
				t.Errorf("got %q, %v", got, err)
			}
		})
	}
}

func TestFindAmbiguous(t *testing.T) {
	for _, arg := range []string{"6065", "00-index.md"} {
		_, err := Find(arg, gists)
		var amb *AmbiguousError
		if !errors.As(err, &amb) || len(amb.Candidates) != 2 {
			t.Fatalf("%s: got %v", arg, err)
		}
		if !strings.Contains(err.Error(), id) || !strings.Contains(err.Error(), "second") {
			t.Errorf("the error does not name the candidates: %v", err)
		}
	}
}

func TestFindNothing(t *testing.T) {
	for _, arg := range []string{"nope", "606", ""} {
		var nf *NotFoundError
		if _, err := Find(arg, gists); !errors.As(err, &nf) {
			t.Errorf("%q: got %v", arg, err)
		}
	}
}

func TestPrefixComesBeforeAFileName(t *testing.T) {
	got, err := Find("6065a", []model.Gist{gist("aaaa", "", "6065a"), gist(id, "")})
	if err != nil || got != id {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestCandidatesCannotBreakTheLine(t *testing.T) {
	_, err := Find("same", []model.Gist{gist("a", "x\x1b[2Jy\nz", "same"), gist("b", "", "same")})
	if err == nil || strings.Contains(err.Error(), "\x1b") || strings.Count(err.Error(), "\n") != 2 {
		t.Errorf("got %q", err)
	}
}
