package gist

import (
	"context"
	"os"

	"github.com/google/go-github/github"
)

// Page represents gist page itself
type Page struct {
	ID          string
	Description string
	Public      bool
	Files       map[string]string
}

// File represents a single file hosted on gist
type File struct {
	Name    string
	Content string
	Gist    Page
}

func List(user string) ([]File, error) {
	client, err := newClient(os.Getenv("GITHUB_TOKEN"))
	if err != nil {
		return []File{}, err
	}
	pages, err := client.List(user)
	if err != nil {
		return []File{}, err
	}
	var files []File
	for _, page := range pages {
		for name, content := range page.Files {
			files = append(files, File{
				Name:    name,
				Content: content,
				Gist:    page,
			})
		}
	}
	return files, nil
}

func Create(page Page) error {
	client, err := newClient(os.Getenv("GITHUB_TOKEN"))
	if err != nil {
		return err
	}
	files := make(map[github.GistFilename]github.GistFile)
	for name, content := range page.Files {
		fn := github.GistFilename(name)
		files[fn] = github.GistFile{
			Filename: github.String(name),
			Content:  github.String(content),
		}
	}
	_, _, err = client.Gists.Create(context.Background(), &github.Gist{
		Files:       files,
		Description: github.String(page.Description),
		Public:      github.Bool(page.Public),
	})
	return err
}
