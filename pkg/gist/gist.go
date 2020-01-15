package gist

import (
	"context"
	"errors"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Client represents gist client
type Client struct {
	*github.Client
}

// New returns Gist structure
func New(token string) (Client, error) {
	if token == "" {
		return Client{}, errors.New("github token is missing")
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)
	return Client{Client: client}, nil
}

// Page represents gist page itself
type Page struct {
	ID          string
	Description string
	Public      bool
	Files       map[string]string
}

// List lists gist pages
func (c Client) List(user string) ([]Page, error) {
	opt := &github.GistListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	var gists []*github.Gist
	for {
		results, resp, err := c.Gists.List(context.Background(), user, opt)
		if err != nil {
			return []Page{}, err
		}
		gists = append(gists, results...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	var pages []Page
	for _, gist := range gists {
		pages = append(pages, Page{
			ID:          gist.GetID(),
			Description: gist.GetDescription(),
			Public:      gist.GetPublic(),
		})
	}
	return pages, nil
}

// Get gets gist page in detail
func (c Client) Get(id string) (Page, error) {
	gist, _, err := c.Gists.Get(context.Background(), id)
	if err != nil {
		return Page{}, err
	}
	files := make(map[string]string)
	for name, file := range gist.Files {
		files[string(name)] = file.GetContent()
	}
	return Page{
		ID:          gist.GetID(),
		Description: gist.GetDescription(),
		Public:      gist.GetPublic(),
		Files:       files,
	}, nil
}

// Create creates gist page
func (c Client) Create(page Page) error {
	files := make(map[github.GistFilename]github.GistFile)
	for name, content := range page.Files {
		fn := github.GistFilename(name)
		files[fn] = github.GistFile{
			Filename: github.String(name),
			Content:  github.String(content),
		}
	}
	_, _, err := c.Gists.Create(context.Background(), &github.Gist{
		Files:       files,
		Description: github.String(page.Description),
		Public:      github.Bool(page.Public),
	})
	return err
}
