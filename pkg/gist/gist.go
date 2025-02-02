package gist

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/babarot/gist/pkg/git"
	"github.com/google/go-github/github"
)

type Gist struct {
	User   string
	Token  string
	Editor string

	Client Client

	WorkDir string
	Pages   []Page
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

	Repo *git.Repo `json:"-"`
}

// File represents a single file hosted on gist
type File struct {
	Name     string `json:"name"`
	Content  string `json:"content"`
	FullPath string `json:"fullpath"`

	Page `json:"-"`
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
				Page:     page,
			})
		}
	}

	return files
}

func (g *Gist) Checkout() error {
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
				Token:    g.Token,
			})
			if err != nil {
				return
			}
			repo.CloneOrOpen(context.Background())
			page.Repo = repo
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

func (f File) HasUpdated() (bool, error) {
	ctx := context.Background()
	repo := f.Page.Repo
	if repo == nil {
		return false, fmt.Errorf("%s: repository not found", f.Name)
	}
	if err := repo.Open(ctx); err != nil {
		return false, err
	}
	return !repo.IsClean(), nil
}

func (f File) Update() error {
	ctx := context.Background()
	repo := f.Page.Repo
	if repo == nil {
		return fmt.Errorf("%s: repository not found", f.Name)
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
	gist, _, err := g.Client.Gists.Create(context.Background(), &github.Gist{
		Files:       files,
		Description: github.String(page.Description),
		Public:      github.Bool(page.Public),
	})

	return gist.GetHTMLURL(), err
}

func (g Gist) Delete(page Page) error {
	_, err := g.Client.Gists.Delete(context.Background(), page.ID)
	return err
}
