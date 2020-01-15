package gist

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
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
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Public      bool      `json:"public"`
	CreatedAt   time.Time `json:"created_at"`

	Files map[string]string `json:"files"`
	Repo  *git.GitRepo      `json:"-"`
}

// File represents a single file hosted on gist
type File struct {
	Name    string
	Content string

	Gist Page
}

type cache struct {
	Pages []Page `json:"pages"`

	path string
}

func newCache(path string) cache {
	return cache{path: path}
}

func (c *cache) open() error {
	f, err := os.Open(c.path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(&c)
}

func (c *cache) save(pages []Page) error {
	f, err := os.Create(c.path)
	if err != nil {
		return err
	}
	defer f.Close()
	c.Pages = pages
	return json.NewEncoder(f).Encode(&c)
}

func List(user, workDir string) ([]File, error) {
	token := os.Getenv("GITHUB_TOKEN")

	var pages []Page
	f := filepath.Join(workDir, "cache.json")
	c := newCache(f)
	c.open()
	defer c.save(pages)

	switch len(c.Pages) {
	case 0:
		client := newClient(token)
		results, err := client.List(user)
		if err != nil {
			return []File{}, err
		}
		pages = results
	default:
		pages = c.Pages
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
			repo, err := git.NewGitRepo(git.Config{
				URL:      fmt.Sprintf("https://gist.github.com/%s/%s", user, page.ID),
				WorkDir:  workDir,
				Username: user,
				Token:    token,
			})
			if err != nil {
				log.Println(err)
			}
			repo.CloneOrOpen(context.Background())
			page.Repo = repo
			files := make(map[string]string)
			for name := range page.Files {
				content, err := ioutil.ReadFile(filepath.Join(repo.Path(), name))
				if err != nil {
					log.Println(err)
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
