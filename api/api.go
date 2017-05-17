package api

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/b4b4r07/gist/util"
	"github.com/google/go-github/github"
	"github.com/mattn/go-runewidth"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

type (
	Items []*github.Gist
	Item  *github.Gist
)

type Gist struct {
	Client *github.Client
	Items  Items
	Config Config
	Files  Files
}

type Config struct {
	Token      string
	BaseURL    string
	NewPrivate bool
	ClonePath  string
}

type File struct {
	ID          string `json:"id"`
	ShortID     string `json:"short_id"`
	Filename    string `json:"filename"`
	Path        string `json:"path"`
	Content     string `json:"-"`
	Description string `json:"description"`
	Public      bool   `json:"-"`
}

type Files []File

func NewGist(cfg Config) (*Gist, error) {
	if cfg.Token == "" {
		return &Gist{}, errors.New("token is missing")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.Token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)

	return &Gist{
		Client: client,
		Items:  []*github.Gist{},
		Config: cfg,
		Files:  Files{},
	}, nil
}

func (g *Gist) List() error {
	var items Items

	ctx := context.Background()

	// List items from gist.github.com
	gists, resp, err := g.Client.Gists.List(ctx, "", &github.GistListOptions{})
	if err != nil {
		return err
	}
	items = append(items, gists...)

	// pagenation
	for i := 2; i <= resp.LastPage; i++ {
		gists, _, err := g.Client.Gists.List(ctx, "", &github.GistListOptions{
			ListOptions: github.ListOptions{Page: i},
		})
		if err != nil {
			continue
		}
		items = append(items, gists...)
	}
	g.Items = items

	if len(g.Items) == 0 {
		return errors.New("no items")
	}
	return nil
}

func (g *Gist) ListStarred() error {
	var items Items

	ctx := context.Background()
	// List items from gist.github.com
	gists, resp, err := g.Client.Gists.ListStarred(ctx, &github.GistListOptions{})
	if err != nil {
		return err
	}
	items = append(items, gists...)

	// pagenation
	for i := 2; i <= resp.LastPage; i++ {
		gists, _, err := g.Client.Gists.ListStarred(ctx, &github.GistListOptions{
			ListOptions: github.ListOptions{Page: i},
		})
		if err != nil {
			continue
		}
		items = append(items, gists...)
	}
	g.Items = items

	if len(g.Items) == 0 {
		return errors.New("no items")
	}
	return nil
}

func (g *Gist) Create(files Files, desc string) (url string, err error) {
	spn := util.NewSpinner("Creating...")
	spn.Start()
	defer spn.Stop()

	public := true
	if g.Config.NewPrivate {
		public = false
	}
	gistFiles := make(map[github.GistFilename]github.GistFile, len(files))
	for _, file := range files {
		filename := file.Filename
		content := file.Content
		fname := github.GistFilename(filename)
		gistFiles[fname] = github.GistFile{
			Filename: &filename,
			Content:  &content,
		}
	}
	ctx := context.Background()
	item, resp, err := g.Client.Gists.Create(ctx, &github.Gist{
		Files:       gistFiles,
		Public:      &public,
		Description: &desc,
	})
	if item == nil {
		panic("item is nil")
	}
	if resp == nil {
		return url, errors.New("Try again when you have a better network connection")
	}
	err = g.Clone(item)
	if err != nil {
		return url, err
	}
	url = *item.HTMLURL
	return url, errors.Wrap(err, "Failed to create")
}

func (g *Gist) Clone(item *github.Gist) error {
	if util.Exists(*item.ID) {
		return nil
	}

	oldwd, _ := os.Getwd()
	dir := g.Config.ClonePath
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.Mkdir(dir, 0700)
	}
	os.Chdir(dir)
	defer os.Chdir(oldwd)

	// TODO: Start()
	return exec.Command("git", "clone", *item.GitPullURL).Start()
}

func (g *Gist) Delete(id string) error {
	spn := util.NewSpinner("Deleting...")
	spn.Start()
	defer spn.Stop()
	_, err := g.Client.Gists.Delete(context.Background(), id)
	return err
}

func (i *Items) Filter(fn func(Item) bool) *Items {
	items := make(Items, 0)
	for _, item := range *i {
		if fn(item) {
			items = append(items, item)
		}
	}
	return &items
}

func (i *Items) One() Item {
	var item Item
	if len(*i) > 0 {
		return (*i)[0]
	}
	return item
}

func getID(file string) string {
	switch strings.Count(file, "/") {
	case 0:
		return file
	case 1:
		return filepath.Base(file)
	default:
		id := filepath.Base(filepath.Dir(file))
		if id == "files" {
			id = filepath.Base(file)
		}
		return id
	}
}

func (g *Gist) download(fname string) (done bool, err error) {
	gists := g.Items.Filter(func(i Item) bool {
		return *i.ID == getID(fname)
	})

	for _, gist := range *gists {
		item, _, err := g.Client.Gists.Get(context.Background(), *gist.ID)
		if err != nil {
			return done, err
		}
		// for multiple files in one Gist folder
		for _, f := range item.Files {
			fpath := filepath.Join(g.Config.ClonePath, *gist.ID, *f.Filename)
			content, err := util.FileContent(fpath)
			if err != nil {
				continue
			}
			// write to the local files if there are some diff
			if *f.Content != content {
				ioutil.WriteFile(fpath, []byte(*f.Content), os.ModePerm)
				done = true
			}
		}
	}
	return done, nil
}

func (g *Gist) upload(fname string) (done bool, err error) {
	var (
		gistID = getID(fname)
		gist   = func(fname string) github.Gist {
			content, _ := util.FileContent(fname)
			return github.Gist{
				Files: map[github.GistFilename]github.GistFile{
					github.GistFilename(filepath.Base(fname)): github.GistFile{
						Content: github.String(content),
					},
				},
			}
		}(fname)
		filename   = filepath.Base(fname)
		content, _ = util.FileContent(fname)
	)

	ctx := context.Background()
	res, _, err := g.Client.Gists.Get(ctx, gistID)
	if err != nil {
		return done, err
	}

	name := github.GistFilename(filename)
	if *res.Files[name].Content != content {
		_, _, err := g.Client.Gists.Edit(ctx, gistID, &gist)
		if err != nil {
			return done, err
		}
		done = true
	}

	return done, nil
}

func (g *Gist) Compare(fname string) (kind, content string, err error) {
	if len(g.Items) == 0 {
		err = g.List()
		if err != nil {
			return
		}
	}

	fi, err := os.Stat(fname)
	if err != nil {
		err = errors.Wrapf(err, "%s: no such file or directory", fname)
		return
	}

	item := g.Items.Filter(func(i Item) bool {
		return *i.ID == getID(fname)
	}).One()
	if item == nil {
		err = fmt.Errorf("%s: not found in gist", getID(fname))
		return
	}

	ctx := context.Background()
	gist, _, err := g.Client.Gists.Get(ctx, *item.ID)
	if err != nil {
		return
	}
	var (
		remoteContent, localContent string
	)
	localContent, _ = util.FileContent(fname)
	for _, file := range gist.Files {
		if *file.Filename != filepath.Base(fname) {
			return "", "", fmt.Errorf("%s: not found on cloud", filepath.Base(fname))
		}
		remoteContent = *file.Content
	}
	if remoteContent == localContent {
		return "equal", "", nil
	}

	local := fi.ModTime().UTC()
	remote := item.UpdatedAt.UTC()

	switch {
	case local.After(remote):
		return "local", localContent, nil
	case remote.After(local):
		return "remote", remoteContent, nil
	default:
	}

	return "equal", "", nil
}

func (g *Gist) UpdateLocal(fname, content string) error {
	return ioutil.WriteFile(fname, []byte(content), os.ModePerm)
}

func (g *Gist) UpdateRemote(fname, content string) error {
	var (
		gist = func(fname string) github.Gist {
			return github.Gist{
				Files: map[github.GistFilename]github.GistFile{
					github.GistFilename(filepath.Base(fname)): github.GistFile{
						Content: github.String(content),
					},
				},
			}
		}(fname)
		id = getID(fname)
	)
	ctx := context.Background()
	_, _, err := g.Client.Gists.Edit(ctx, id, &gist)
	return err
}

func (g *Gist) Sync(fname string) (err error) {
	var msg string
	spn := util.NewSpinner("Checking...")
	spn.Start()
	defer func() {
		spn.Stop()
		util.Underline(msg, path.Join(g.Config.BaseURL, getID(fname)))
	}()

	kind, content, err := g.Compare(fname)
	if err != nil {
		return err
	}
	switch kind {
	case "local":
		err = g.UpdateRemote(fname, content)
		msg = "Uploaded"
	case "remote":
		err = g.UpdateLocal(fname, content)
		msg = "Downloaded"
	case "equal":
		// Do nothing
	case "":
		// Locally but not remote
	default:
	}

	return err
}

func (g *Gist) ExpandID(shortID string) (longID string, err error) {
	if len(g.Items) == 0 {
		return "", errors.New("no gist items")
	}
	for _, item := range g.Items {
		longID = *item.ID
		if shortID == ShortenID(longID) {
			return longID, nil
		}
	}
	return "", errors.New("no matched ID")
}

var IDLength int = 9

func ShortenID(longID string) string {
	return runewidth.Truncate(longID, IDLength, "")
}
