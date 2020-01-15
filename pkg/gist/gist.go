package gist

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/b4b4r07/gist/pkg/git"
	"github.com/b4b4r07/gist/pkg/shell"
	"github.com/google/go-github/github"
)

// Page represents gist page itself
type Page struct {
	ID          string
	Description string
	Public      bool
	Files       map[string]string
	Repo        *git.GitRepo
	CreatedAt   time.Time
}

// File represents a single file hosted on gist
type File struct {
	Name    string
	Content string

	Gist Page
}

func List(user, workDir string) ([]File, error) {
	token := os.Getenv("GITHUB_TOKEN")

	client, err := newClient(token)
	if err != nil {
		return []File{}, err
	}

	pages, err := client.List(user)
	if err != nil {
		return []File{}, err
	}

	ch := make(chan Page, len(pages))
	wg := new(sync.WaitGroup)

	for _, page := range pages {
		page := page
		wg.Add(1)
		go func() {
			defer func() {
				ch <- page
				wg.Done()
			}()
			repo, _ := git.NewGitRepo(git.Config{
				URL:      fmt.Sprintf("https://gist.github.com/%s/%s", user, page.ID),
				WorkDir:  workDir,
				Username: user,
				Token:    token,
			})
			repo.CloneOrOpen(context.Background())
			page.Repo = repo
		}()
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	// pages = []Page{}
	pages = make([]Page, len(pages))
	for p := range ch {
		pages = append(pages, p)
	}
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].CreatedAt.After(pages[j].CreatedAt)
	})

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

func (f *File) Edit() error {
	path := filepath.Join(f.Gist.Repo.Path(), f.Name)
	vim := shell.New("vim", path)
	ctx := context.Background()
	if err := vim.Run(ctx); err != nil {
		return err
	}
	repo := f.Gist.Repo
	if repo.IsClean() {
		// no need to push
		return nil
	}
	if err := repo.Add(f.Name); err != nil {
		return err
	}
	if err := repo.Commit("update"); err != nil {
		return err
	}
	return repo.Push(ctx)
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
