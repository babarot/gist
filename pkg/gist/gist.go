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
	"github.com/google/go-github/github"
)

type Gist struct {
	Pages []Page
	Files []File

	WorkDir string
	User    string

	cache *cache
}

func New(user, workDir string) Gist {
	f := filepath.Join(workDir, "cache.json")
	c := newCache(f)
	c.open()
	return Gist{
		WorkDir: workDir,
		User:    user,
		cache:   c,
	}
}

// Page represents gist page itself
type Page struct {
	ID          string            `json:"id"`
	Description string            `json:"description"`
	Public      bool              `json:"public"`
	CreatedAt   time.Time         `json:"created_at"`
	Files       map[string]string `json:"files"`

	Repo *git.Repo `json:"-"`
}

// File represents a single file hosted on gist
type File struct {
	Name    string
	Content string

	Gist Page
}

func (g Gist) List() ([]File, error) {
	token := os.Getenv("GITHUB_TOKEN")

	var pages []Page
	switch len(g.cache.Pages) {
	case 0:
		client := newClient(token)
		results, err := client.List(g.User)
		if err != nil {
			return []File{}, err
		}
		pages = results
	default:
		pages = g.cache.Pages
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
			repo, err := git.NewRepo(git.Config{
				URL:      fmt.Sprintf("https://gist.github.com/%s/%s", g.User, page.ID),
				WorkDir:  filepath.Join(g.WorkDir, g.User, page.ID),
				Username: g.User,
				Token:    token,
			})
			if err != nil {
				return
			}
			repo.CloneOrOpen(context.Background())
			page.Repo = repo
			files := make(map[string]string)
			for name := range page.Files {
				content, err := ioutil.ReadFile(filepath.Join(repo.Path(), name))
				if err != nil {
					return
				}
				files[name] = string(content)
			}
			page.Files = files
		}()
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	pages = []Page{}

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

	g.cache.save(pages)
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

func (g Gist) Create(page Page) error {
	defer g.cache.delete()
	client := newClient(os.Getenv("GITHUB_TOKEN"))
	files := make(map[github.GistFilename]github.GistFile)
	for name, content := range page.Files {
		fn := github.GistFilename(name)
		files[fn] = github.GistFile{
			Filename: github.String(name),
			Content:  github.String(content),
		}
	}
	_, _, err := client.Gists.Create(context.Background(), &github.Gist{
		Files:       files,
		Description: github.String(page.Description),
		Public:      github.Bool(page.Public),
	})
	return err
}
