package api

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/google/go-github/github"
	runewidth "github.com/mattn/go-runewidth"
	"golang.org/x/oauth2"
)

var IDLength = 9

type (
	Gist struct {
		Client *github.Client
		Items  []*github.Gist
	}

	Item struct {
		ID          string
		ShortID     string
		Description string
		Public      bool
		Files       []File
	}
	Items []Item
	File  struct {
		Filename string
		Content  string
	}
	Files []File
)

const (
	// EnvToken is GitHub token string
	EnvToken = "GITHUB_TOKEN"
)

func NewGist(token string) (*Gist, error) {
	token = strings.TrimPrefix(token, "$")
	if token == EnvToken {
		token = os.Getenv(EnvToken)
	}
	if token == "" {
		return &Gist{}, errors.New("github token is missing")
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)
	return &Gist{Client: client}, nil
}

func (g *Gist) List() (items Items, err error) {
	ctx := context.Background()
	gists, resp, err := g.Client.Gists.List(ctx, "", &github.GistListOptions{})
	if err != nil {
		return
	}
	var gs []*github.Gist
	gs = append(gs, gists...)

	for i := 2; i <= resp.LastPage; i++ {
		gists, _, err := g.Client.Gists.List(ctx, "", &github.GistListOptions{
			ListOptions: github.ListOptions{Page: i},
		})
		if err != nil {
			continue
		}
		gs = append(gs, gists...)
	}

	for _, g := range gs {
		var files Files
		for _, file := range g.Files {
			files = append(files, File{
				Filename: *file.Filename,
			})
		}
		items = append(items, Item{
			ID:      *g.ID,
			ShortID: runewidth.Truncate(*g.ID, IDLength, ""),
			Files:   files,
			Description: func() (desc string) {
				if g.Description != nil {
					desc = *g.Description
				}
				return
			}(),
			Public: *g.Public,
		})
	}
	return
}

func (g *Gist) ListStarred() (items []*github.Gist, err error) {
	return
}

func (g *Gist) Create(
	files map[github.GistFilename]github.GistFile,
	desc string,
	private bool) (item *github.Gist, err error) {
	public := !private
	item, resp, err := g.Client.Gists.Create(context.Background(), &github.Gist{
		Files:       files,
		Description: &desc,
		Public:      &public,
	})
	if item == nil {
		err = errors.New("Gist item is nil")
	}
	if resp == nil {
		err = errors.New("Try again when you have a better network connection")
	}
	return
}

func (g *Gist) Delete(id string) (err error) {
	_, err = g.Client.Gists.Delete(context.Background(), id)
	return
}

func (g *Gist) Get(id string) (item *github.Gist, err error) {
	item, _, err = g.Client.Gists.Get(context.Background(), id)
	return
}

func (g *Gist) Update(id string, files map[github.GistFilename]github.GistFile) (err error) {
	_, _, err = g.Client.Gists.Edit(context.Background(), id, &github.Gist{Files: files})
	return
}
