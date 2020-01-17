package gist

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/b4b4r07/gist/pkg/git"
	"github.com/b4b4r07/gist/pkg/shell"
	"github.com/google/go-github/github"
)

type Gist struct {
	WorkDir string
	User    string

	Pages []Page
}

// Page represents gist page itself
type Page struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	User        string    `json:"user"`
	URL         string    `json:"url"`
	Public      bool      `json:"public"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Files       []File    `json:"files"`
}

// File represents a single file hosted on gist
type File struct {
	Name     string
	Content  string
	FullPath string

	Gist Page
}

func (g Gist) Files() []File {
	var files []File
	for _, page := range g.Pages {
		for _, file := range page.Files {
			path := filepath.Join(g.WorkDir, g.User, page.ID, file.Name)
			content, _ := ioutil.ReadFile(path)
			files = append(files, File{
				Name:     file.Name,
				Content:  string(content),
				FullPath: path,
				Gist:     page,
			})
		}
	}

	return files
}

func (g Gist) Update() error {
	token := os.Getenv("GITHUB_TOKEN")
	ch := make(chan Page, len(g.Pages))
	wg := new(sync.WaitGroup)

	for _, page := range g.Pages {
		page := page
		wg.Add(1)
		go func() {
			defer func() {
				ch <- page
				wg.Done()
			}()
			repo, err := git.NewRepo(git.Config{
				URL:      page.URL,
				WorkDir:  filepath.Join(g.WorkDir, g.User, page.ID),
				Username: g.User,
				Token:    token,
			})
			if err != nil {
				return
			}
			repo.CloneOrOpen(context.Background())
		}()
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	pages := []Page{}
	for p := range ch {
		pages = append(pages, p)
	}

	sort.Slice(pages, func(i, j int) bool {
		return pages[i].CreatedAt.After(pages[j].CreatedAt)
	})

	g.Pages = pages

	return nil
}

func (f File) Edit() error {
	vim := shell.New("vim", f.FullPath)
	ctx := context.Background()
	return vim.Run(ctx)
}

func (f File) Upload() error {
	ctx := context.Background()
	token := os.Getenv("GITHUB_TOKEN")
	repo, err := git.NewRepo(git.Config{
		URL:      f.Gist.URL,
		WorkDir:  filepath.Dir(f.FullPath),
		Username: f.Gist.User,
		Token:    token,
	})
	if err != nil {
		return err
	}
	if err := repo.Open(ctx); err != nil {
		return err
	}
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

func (g Gist) Create(page Page) (string, error) {
	files := make(map[github.GistFilename]github.GistFile)
	for _, file := range page.Files {
		name := github.GistFilename(file.Name)
		files[name] = github.GistFile{
			Filename: github.String(file.Name),
			Content:  github.String(file.Content),
		}
	}
	client := NewClient(os.Getenv("GITHUB_TOKEN"))
	gist, _, err := client.Gists.Create(context.Background(), &github.Gist{
		Files:       files,
		Description: github.String(page.Description),
		Public:      github.Bool(page.Public),
	})

	return gist.GetHTMLURL(), err
}

func (g Gist) Delete(page Page) error {
	client := NewClient(os.Getenv("GITHUB_TOKEN"))
	_, err := client.Gists.Delete(context.Background(), page.ID)
	return err
}
