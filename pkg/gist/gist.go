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
	Files []File

	WorkDir string
	User    string

	cache *cache
}

// Page represents gist page itself
type Page struct {
	User        string            `json:"user"`
	ID          string            `json:"id"`
	Description string            `json:"description"`
	URL         string            `json:"url"`
	Public      bool              `json:"public"`
	CreatedAt   time.Time         `json:"created_at"`
	Files       map[string]string `json:"files"`
}

// File represents a single file hosted on gist
type File struct {
	Name    string
	Content string

	FullPath string

	Gist Page
}

func New(user, workDir string) Gist {
	gist := Gist{
		WorkDir: workDir,
		User:    user,
	}
	files, err := gist.list()
	if err != nil {
		panic(err)
	}
	gist.Files = files
	return gist
}

func (g Gist) clone(pages []Page) []Page {
	token := os.Getenv("GITHUB_TOKEN")

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
				URL:      page.URL,
				WorkDir:  filepath.Join(g.WorkDir, g.User, page.ID),
				Username: g.User,
				Token:    token,
			})
			if err != nil {
				return
			}
			repo.CloneOrOpen(context.Background())
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

	return pages
}

func (g Gist) list() ([]File, error) {
	token := os.Getenv("GITHUB_TOKEN")

	f := filepath.Join(g.WorkDir, "cache.json")
	c := newCache(f)
	c.open()

	var pages []Page
	switch len(c.Pages) {
	case 0:
		client := newClient(token)
		results, err := client.List(g.User)
		if err != nil {
			return []File{}, err
		}
		pages = results
	default:
		pages = c.Pages
	}

	pages = g.clone(pages)

	var files []File
	for _, page := range pages {
		for name, content := range page.Files {
			files = append(files, File{
				Name:     name,
				Content:  content,
				FullPath: filepath.Join(g.WorkDir, g.User, page.ID, name),
				Gist:     page,
			})
		}
	}

	c.save(pages)
	return files, nil
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
	return repo.Push(ctx)
}

func (g Gist) Create(page Page) error {
	// defer g.cache.delete()
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
