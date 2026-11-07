package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/babarot/gist/v2/internal/fakegist"
	"github.com/babarot/gist/v2/internal/github"
	"github.com/babarot/gist/v2/internal/logging"
)

func TestAuth(t *testing.T) {
	scopes := func(s string) *string { return &s }
	for _, tt := range []struct {
		name   string
		scopes *string
		source string
		code   int
		want   []string
	}{
		{"gh", scopes("gist, repo"), "oauth_token", 0, []string{"user   alice", "token  gh", "gists  read and write"}},
		{"environment", scopes("gist"), "GITHUB_TOKEN", 0, []string{"token  GITHUB_TOKEN"}},
		{"no gist scope", scopes("repo"), "gh", 1, []string{"none: the token lacks the gist scope"}},
		{"fine-grained", nil, "GH_TOKEN", 0, []string{"token  GH_TOKEN", "gists  read (a fine-grained"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := fakegist.New(t)
			s.Scopes = tt.scopes
			c, err := github.New(logging.Discard(), github.Options{Token: "t", Transport: s.Transport()})
			if err != nil {
				t.Fatal(err)
			}
			var out, errOut bytes.Buffer
			if code := Auth(context.Background(), &out, &errOut, tt.source, c); code != tt.code {
				t.Errorf("exit %d, stderr %q", code, errOut.String())
			}
			for _, want := range tt.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("stdout is %q, want %q", out.String(), want)
				}
			}
		})
	}
}

type failing struct{}

func (failing) CheckAccess(context.Context) (github.Access, error) {
	return github.Access{}, errors.New("bad credentials")
}

func TestAuthFails(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Auth(context.Background(), &out, &errOut, "gh", failing{})
	if code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "bad credentials") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out.String(), errOut.String())
	}
}
