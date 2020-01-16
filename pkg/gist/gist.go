package gist

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/b4b4r07/gist/pkg/git"
	"github.com/b4b4r07/gist/pkg/shell"
	"github.com/caarlos0/spin"
	"github.com/google/go-github/github"
)

type Gist struct {
	WorkDir string
	User    string

	Pages []Page

	cache *cache
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

func New() Gist {
	workDir := filepath.Join(os.Getenv("HOME"), ".gist")
	f := filepath.Join(workDir, "cache.json")
	c := newCache(f)
	return Gist{
		User:    os.Getenv("USER"),
		WorkDir: workDir,
		cache:   c,
	}
}

func (g Gist) Files() []File {
	token := os.Getenv("GITHUB_TOKEN")

	// load cache
	g.cache.open()

	switch len(g.cache.Pages) {
	case 0:
		s := spin.New("%s Fetching pages...")
		s.Start()
		client := newClient(token)
		pages, err := client.List(g.User)
		if err != nil {
			panic(err)
		}
		g.Pages = pages
		s.Stop()
	default:
		g.Pages = g.cache.Pages
	}
	g.cache.save(g.Pages)

	g.update()

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

func (g Gist) update() error {
	s := spin.New("%s Checking pages...")
	s.Start()
	defer s.Stop()

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

func (f *File) Edit() error {
	vim := shell.New("vim", f.FullPath)
	ctx := context.Background()
	if err := vim.Run(ctx); err != nil {
		return err
	}
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
	s := spin.New("%s Pushing...")
	s.Start()
	defer func() {
		s.Stop()
		fmt.Println("Pushed")
		os.Remove(filepath.Join(os.Getenv("HOME"), ".gist", "cache.json")) // TODO
	}()
	return repo.Push(ctx)
}

func (g Gist) Create(page Page) error {
	s := spin.New("%s Creating page...")
	s.Start()
	defer g.cache.delete()

	files := make(map[github.GistFilename]github.GistFile)
	for _, file := range page.Files {
		name := github.GistFilename(file.Name)
		files[name] = github.GistFile{
			Filename: github.String(file.Name),
			Content:  github.String(file.Content),
		}
	}
	client := newClient(os.Getenv("GITHUB_TOKEN"))
	gist, _, err := client.Gists.Create(context.Background(), &github.Gist{
		Files:       files,
		Description: github.String(page.Description),
		Public:      github.Bool(page.Public),
	})

	s.Stop()
	fmt.Println(gist.GetHTMLURL())
	return err
}

func (g Gist) Delete(page Page) error {
	s := spin.New("%s Deleting page...")
	s.Start()
	defer g.cache.delete()
	client := newClient(os.Getenv("GITHUB_TOKEN"))
	_, err := client.Gists.Delete(context.Background(), page.ID)
	s.Stop()
	fmt.Println("Deleted")
	return err
}
