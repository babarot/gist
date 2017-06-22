package api

import (
	"context"
	"errors"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type Gist struct {
	Client *github.Client
	Items  Items
}

type (
	Items []*github.Gist
	Item  *github.Gist
)

func NewGist(token string) (*Gist, error) {
	if token == "" {
		return &Gist{}, errors.New("token is missing")
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)
	return &Gist{Client: client}, nil
}

func (g *Gist) List() (err error) {
	var items []*github.Gist
	ctx := context.Background()

	gists, resp, err := g.Client.Gists.List(ctx, "", &github.GistListOptions{})
	if err != nil {
		return
	}
	items = append(items, gists...)

	for i := 2; i <= resp.LastPage; i++ {
		gists, _, err := g.Client.Gists.List(ctx, "", &github.GistListOptions{
			ListOptions: github.ListOptions{Page: i},
		})
		if err != nil {
			continue
		}
		items = append(items, gists...)
	}

	if len(items) == 0 {
		err = errors.New("no items")
		return
	}
	g.Items = items
	return
}

func (g *Gist) ListStarred() (err error) {
	var items []*github.Gist
	ctx := context.Background()
	gists, resp, err := g.Client.Gists.ListStarred(ctx, &github.GistListOptions{})
	if err != nil {
		return
	}
	items = append(items, gists...)

	for i := 2; i <= resp.LastPage; i++ {
		gists, _, err := g.Client.Gists.ListStarred(ctx, &github.GistListOptions{
			ListOptions: github.ListOptions{Page: i},
		})
		if err != nil {
			continue
		}
		items = append(items, gists...)
	}

	if len(items) == 0 {
		err = errors.New("no items")
		return
	}
	g.Items = items
	return
}

func (g *Gist) Create(files map[github.GistFilename]github.GistFile, desc string, public bool) (item *github.Gist, err error) {
	// gistFiles := make(map[github.GistFilename]github.GistFile, len(items))
	// for _, item := range items {
	// 	var (
	// 		filename = item.Filename
	// 		content  = item.Content
	// 		file     = github.GistFilename(filename)
	// 	)
	// 	gistFiles[file] = github.GistFile{
	// 		Filename: &filename,
	// 		Content:  &content,
	// 	}
	// }
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
	// TODO: there is a responsibility on cli side
	// err = g.Clone(item)
	return
}

func (g *Gist) Delete(id string) error {
	_, err := g.Client.Gists.Delete(context.Background(), id)
	return err
}
