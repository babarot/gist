package gist

import (
	"context"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type Client struct {
	*github.Client
}

func NewClient(token string) Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	return Client{github.NewClient(tc)}
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
		var files []File
		for name := range gist.Files {
			files = append(files, File{Name: string(name)})
		}
		pages = append(pages, Page{
			ID:          gist.GetID(),
			Description: gist.GetDescription(),
			Public:      gist.GetPublic(),
			CreatedAt:   gist.GetCreatedAt(),
			UpdatedAt:   gist.GetUpdatedAt(),
			Files:       files,
			URL:         gist.GetHTMLURL(),
			User:        user,
		})
	}
	return pages, nil
}
