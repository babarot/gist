package api

import (
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
	Token             string
	ShowIndicator     bool
	OpenStarredItems  bool
	NewPrivate        bool
	Dir               string
	BaseURL           string
	Editor            string
	ShowPrivateSymbol bool
}

type File struct {
	ID          string
	ShortID     string
	Filename    string
	Path        string
	Content     string
	Description string
	Public      bool
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

func (g *Gist) Get() error {
	var items Items

	// Get items from gist.github.com
	gists, resp, err := g.Client.Gists.List("", &github.GistListOptions{})
	if err != nil {
		return err
	}
	items = append(items, gists...)

	// pagenation
	for i := 2; i <= resp.LastPage; i++ {
		gists, _, err := g.Client.Gists.List("", &github.GistListOptions{
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

func (g *Gist) GetStars() error {
	var items Items

	// Get items from gist.github.com
	gists, resp, err := g.Client.Gists.ListStarred(&github.GistListOptions{})
	if err != nil {
		return err
	}
	items = append(items, gists...)

	// pagenation
	for i := 2; i <= resp.LastPage; i++ {
		gists, _, err := g.Client.Gists.ListStarred(&github.GistListOptions{
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
	item, resp, err := g.Client.Gists.Create(&github.Gist{
		Files:       gistFiles,
		Public:      &public,
		Description: &desc,
	})
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
	dir := g.Config.Dir
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
	_, err := g.Client.Gists.Delete(id)
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
		return filepath.Base(filepath.Dir(file))
	}
}

func (g *Gist) download(fname string) (done bool, err error) {
	gists := g.Items.Filter(func(i Item) bool {
		return *i.ID == getID(fname)
	})

	for _, gist := range *gists {
		item, _, err := g.Client.Gists.Get(*gist.ID)
		if err != nil {
			return done, err
		}
		// for multiple files in one Gist folder
		for _, f := range item.Files {
			fpath := filepath.Join(g.Config.Dir, *gist.ID, *f.Filename)
			content := util.FileContent(fpath)
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
			return github.Gist{
				Files: map[github.GistFilename]github.GistFile{
					github.GistFilename(filepath.Base(fname)): github.GistFile{
						Content: github.String(util.FileContent(fname)),
					},
				},
			}
		}(fname)
		filename = filepath.Base(fname)
		content  = util.FileContent(fname)
	)

	res, _, err := g.Client.Gists.Get(gistID)
	if err != nil {
		return done, err
	}

	name := github.GistFilename(filename)
	if *res.Files[name].Content != content {
		_, _, err := g.Client.Gists.Edit(gistID, &gist)
		if err != nil {
			return done, err
		}
		done = true
	}

	return done, nil
}

func (g *Gist) Sync(fname string) error {
	var (
		err error
		msg string
	)

	spn := util.NewSpinner("Cheking...")
	spn.Start()
	defer func() {
		defer spn.Stop()
		util.Underline(msg, path.Join(g.Config.BaseURL, getID(fname)))
	}()

	if len(g.Items) == 0 {
		err = g.Get()
		if err != nil {
			return err
		}
	}

	item := g.Items.Filter(func(i Item) bool {
		return *i.ID == getID(fname)
	}).One()

	fi, err := os.Stat(fname)
	if err != nil {
		return errors.Wrapf(err, "%s: no such file or directory", fname)
	}

	local := fi.ModTime().UTC()
	remote := item.UpdatedAt.UTC()

	switch {
	case local.After(remote):
		done, err := g.upload(fname)
		if err != nil {
			return err
		}
		if done {
			msg = "Uploaded"
		}
	case remote.After(local):
		done, err := g.download(fname)
		if err != nil {
			return err
		}
		if done {
			msg = "Downloaded"
		}
	default:
	}

	return nil
}

func (g *Gist) ExpandID(shortID string) (longID string, err error) {
	if len(g.Items) == 0 {
		return "", errors.New("bad")
	}
	for _, item := range g.Items {
		longID = *item.ID
		if shortID == ShortenID(longID) {
			return longID, nil
		}
	}
	return "", errors.New("bad")
}

var IDLength int = 9

func ShortenID(longID string) string {
	return runewidth.Truncate(longID, IDLength, "")
}

// func (g *Gist) EditDesc(id, desc string) error {
// 	spn := util.NewSpinner("Editing...")
// 	spn.Start()
// 	defer spn.Stop()
// 	item := github.Gist{
// 		Description: github.String(desc),
// 	}
// 	_, _, err := g.Client.Gists.Edit(id, &item)
// 	return err
// }
//
// func (g *Gist) Edit(fname string) error {
// 	if err := g.Sync(fname); err != nil {
// 		return err
// 	}
//
// 	editor := g.Config.Editor
// 	if editor == "" {
// 		return errors.New("$EDITOR: not set")
// 	}
//
// 	err := util.RunCommand(editor, fname)
// 	if err != nil {
// 		return err
// 	}
//
// 	return g.Sync(fname)
// }
