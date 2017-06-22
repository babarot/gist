package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	tt "text/template"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

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
			ShortID: *g.ID,
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
	public bool) (item *github.Gist, err error) {
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

func (items *Items) Render(columns []string) []string {
	var lines []string
	max := 0
	for _, item := range *items {
		for _, file := range item.Files {
			if len(file.Filename) > max {
				max = len(file.Filename)
			}
		}
	}
	for _, item := range *items {
		var line string
		var tmpl *tt.Template
		if len(columns) == 0 {
			// default
			columns = []string{"{{.ID}}"}
		}
		fnfmt := fmt.Sprintf("%%-%ds", max)
		for _, file := range item.Files {
			format := columns[0]
			for _, v := range columns[1:] {
				format += "\t" + v
			}
			t, err := tt.New("format").Parse(format)
			if err != nil {
				return []string{}
			}
			tmpl = t
			if tmpl != nil {
				var b bytes.Buffer
				err := tmpl.Execute(&b, map[string]interface{}{
					"ID":          item.ID,
					"Description": item.Description,
					"Filename":    fmt.Sprintf(fnfmt, file.Filename),
				})
				if err != nil {
					return []string{}
				}
				line = b.String()
			}
			lines = append(lines, line)
		}
	}
	return lines
}
