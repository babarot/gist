package recording

import (
	"context"
	"testing"
	"time"

	"github.com/babarot/gist/v2/internal/fakegist"
	"github.com/babarot/gist/v2/internal/github"
	"github.com/babarot/gist/v2/internal/logging"
)

func TestTheMadeUpGitHubAnswers(t *testing.T) {
	srv := fakegist.New(t)
	now := time.Now()
	Fill(srv, now)
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
	rel, err := c.Releases(ctx)
	if err != nil || len(rel) != 2 || rel[0].Version != Version {
		t.Errorf("%+v, %v", rel, err)
	}
}
