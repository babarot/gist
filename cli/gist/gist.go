package gist

import (
	"errors"

	_ "github.com/b4b4r07/gist/api/gist"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type Gist struct {
	Client *github.Client
	Items  []*github.Gist
}

func NewGist(token string) (g *Gist, err error) {
	if token == "" {
		err = errors.New("token is missing")
		return
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)

	return &Gist{Client: client}, nil
}
