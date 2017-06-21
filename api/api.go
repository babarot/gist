package api

import (
	"context"
	"errors"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type (
	Items []*github.Gist
	Item  *github.Gist
)

type Gist struct {
	Client *github.Client
	Items  Items
	Files  []File
}

type File struct {
	ID          string `json:"id"`
	ShortID     string `json:"short_id"`
	Filename    string `json:"filename"`
	Path        string `json:"path"`
	Content     string `json:"-"`
	Description string `json:"description"`
	Public      bool   `json:"public"`
}

type Files []File

func NewGist(token string) (*Gist, error) {
	if token == "" {
		return &Gist{}, errors.New("token is missing")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)

	return &Gist{
		Client: client,
		Items:  []*github.Gist{},
		Files:  Files{},
	}, nil
}

func (g *Gist) List() (err error) {
	var items Items
	ctx := context.Background()

	gists, resp, err := g.Client.Gists.List(ctx, "", &github.GistListOptions{})
	if err != nil {
		return
	}
	items = append(items, gists...)

	// pagenation
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
	var items Items
	ctx := context.Background()
	gists, resp, err := g.Client.Gists.ListStarred(ctx, &github.GistListOptions{})
	if err != nil {
		return
	}
	items = append(items, gists...)

	// pagenation
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

func (g *Gist) Create(files Files, desc string, public bool) (item *github.Gist, err error) {
	// spn := util.NewSpinner("Creating...")
	// spn.Start()
	// defer spn.Stop()

	gistFiles := make(map[github.GistFilename]github.GistFile, len(files))
	for _, file := range files {
		filename := file.Filename
		content := file.Content
		fname := github.GistFilename(filename)
		gistFiles[fname] = github.GistFile{
			Filename: &filename,
			Content:  &content,
		}
	}
	item, resp, err := g.Client.Gists.Create(context.Background(), &github.Gist{
		Files:       gistFiles,
		Public:      &public,
		Description: &desc,
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
	// spn := util.NewSpinner("Deleting...")
	// spn.Start()
	// defer spn.Stop()
	_, err := g.Client.Gists.Delete(context.Background(), id)
	return err
}
