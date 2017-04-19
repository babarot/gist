package gist

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/b4b4r07/gist/config"
	"github.com/b4b4r07/gist/util"
	"github.com/briandowns/spinner"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

var (
	SpinnerSymbol int  = 14
	ShowSpinner   bool = true
	Verbose       bool = true

	DescriptionEmpty string = "description"
)

type (
	Items []*github.Gist
	Item  *github.Gist
)

type Gist struct {
	Client *github.Client
	Items  Items
}

type File struct {
	ID          string
	ShortID     string
	Filename    string
	Path        string
	FullPath    string
	Content     string
	Description string
}

type Files []File

type GistFiles struct {
	Files []File
	Text  string
}

func New(token string) (*Gist, error) {
	if token == "" {
		return &Gist{}, errors.New("token is missing")
	}

	// TODO: c.f. go-redis
	ShowSpinner = config.Conf.Flag.ShowSpinner
	Verbose = config.Conf.Flag.Verbose

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)

	return &Gist{
		Client: client,
		Items:  []*github.Gist{},
	}, nil
}

// TODO
// replacement with pkg/errors
func errorWrapper(err error) error {
	if strings.Contains(err.Error(), "tcp") {
		return errors.New("Try again when you have a better network connection")
	}
	return err
}

func (g *Gist) getItems() error {
	var items Items

	// Get items from gist.github.com
	gists, resp, err := g.Client.Gists.List("", &github.GistListOptions{})
	if err != nil {
		return errorWrapper(err)
		// return err
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

func (g *Gist) GetRemoteFiles() (gfs GistFiles, err error) {
	if ShowSpinner {
		s := spinner.New(spinner.CharSets[SpinnerSymbol], 100*time.Millisecond)
		s.Suffix = " Fetching..."
		s.Start()
		defer s.Stop()
	}

	// fetch remote files
	err = g.getItems()
	if err != nil {
		return gfs, err
	}

	// for downloading
	oldwd, _ := os.Getwd()
	dir := config.Conf.Gist.Dir
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.Mkdir(dir, 0700)
	}
	os.Chdir(dir)
	defer os.Chdir(oldwd)

	var files Files
	for _, item := range g.Items {
		if !util.Exists(*item.ID) {
			// TODO: Start()
			err := exec.Command("git", "clone", *item.GitPullURL).Start()
			if err != nil {
				continue
			}
		}
		desc := ""
		if item.Description != nil {
			desc = *item.Description
		}
		if desc == DescriptionEmpty {
			desc = ""
		}
		for _, f := range item.Files {
			files = append(files, File{
				ID:          *item.ID,
				ShortID:     util.ShortenID(*item.ID),
				Filename:    *f.Filename,
				Path:        filepath.Join(*item.ID, *f.Filename),
				FullPath:    filepath.Join(config.Conf.Gist.Dir, *item.ID, *f.Filename),
				Description: desc,
			})
		}
	}

	var text string
	var length int
	max := len(files) - 1
	prefixes := make([]string, max+1)
	var previous, current, next string
	for i, file := range files {
		if len(file.Filename) > length {
			length = len(file.Filename)
		}
		current = files[i].ID
		switch {
		case i == 0:
			previous = ""
			next = files[i+1].ID
		case 0 < i && i < max:
			previous = files[i-1].ID
			next = files[i+1].ID
		case i == max:
			previous = files[i-1].ID
			next = ""
		}
		prefixes[i] = " "
		if current == previous {
			prefixes[i] = "|"
			if current != next {
				prefixes[i] = "+"
			}
		}
		if current == next {
			prefixes[i] = "|"
			if current != previous {
				prefixes[i] = "+"
			}
		}
	}
	format := fmt.Sprintf("%%-%ds\t%%-%ds\t%%s\n", util.LengthID, length)
	if config.Conf.Core.ShowIndicator {
		format = fmt.Sprintf(" %%s %%-%ds\t%%-%ds\t%%s\n", util.LengthID, length)
	}
	for i, file := range files {
		if config.Conf.Core.ShowIndicator {
			text += fmt.Sprintf(format, prefixes[i], util.ShortenID(file.ID), file.Filename, file.Description)
		} else {
			text += fmt.Sprintf(format, util.ShortenID(file.ID), file.Filename, file.Description)
		}
	}

	return GistFiles{
		Files: files,
		Text:  text,
	}, nil
}

func (g *Gist) Create(files Files, desc string) (url string, err error) {
	if ShowSpinner {
		s := spinner.New(spinner.CharSets[SpinnerSymbol], 100*time.Millisecond)
		s.Suffix = " Creating..."
		s.Start()
		defer s.Stop()
	}

	public := true
	if config.Conf.Flag.Private {
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
	gistResp, resp, err := g.Client.Gists.Create(&github.Gist{
		Files:       gistFiles,
		Public:      &public,
		Description: &desc,
	})
	if resp == nil {
		return url, errors.New("Try again when you have a better network connection")
	}
	url = *gistResp.HTMLURL
	return url, errors.Wrap(err, "Failed to create")
}

func (g *Gist) Delete(id string) error {
	if ShowSpinner {
		s := spinner.New(spinner.CharSets[SpinnerSymbol], 100*time.Millisecond)
		s.Suffix = " Deleting..."
		s.Start()
		defer s.Stop()
	}
	_, err := g.Client.Gists.Delete(id)
	return err
}

func (f *Files) Filter(fn func(File) bool) *Files {
	files := make(Files, 0)
	for _, file := range *f {
		if fn(file) {
			files = append(files, file)
		}
	}
	return &files
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

// TODO:
// Gist -> []Files
func (g *Gist) Download(fname string) (url string, err error) {
	if ShowSpinner {
		s := spinner.New(spinner.CharSets[SpinnerSymbol], 100*time.Millisecond)
		s.Suffix = " Checking..."
		s.Start()
		defer s.Stop()
	}

	gists := g.Items.Filter(func(i Item) bool {
		return *i.ID == util.GetID(fname)
	})

	for _, gist := range *gists {
		g, _, err := g.Client.Gists.Get(*gist.ID)
		if err != nil {
			return url, err
		}
		// for multiple files in one Gist folder
		for _, f := range g.Files {
			fpath := filepath.Join(config.Conf.Gist.Dir, *gist.ID, *f.Filename)
			content := util.FileContent(fpath)
			// write to the local files if there are some diff
			if *f.Content != content {
				ioutil.WriteFile(fpath, []byte(*f.Content), os.ModePerm)
				// After rewriting returns URL
				url = *gist.HTMLURL
			}
		}
	}
	return url, nil
}

func makeGist(fname string) github.Gist {
	body := util.FileContent(fname)
	return github.Gist{
		Description: github.String("description"),
		Public:      github.Bool(true),
		Files: map[github.GistFilename]github.GistFile{
			github.GistFilename(filepath.Base(fname)): github.GistFile{
				Content: github.String(body),
			},
		},
	}
}

// TODO:
// Gist -> []Files
func (g *Gist) Upload(fname string) (url string, err error) {
	if ShowSpinner {
		s := spinner.New(spinner.CharSets[SpinnerSymbol], 100*time.Millisecond)
		s.Suffix = " Checking..."
		s.Start()
		defer s.Stop()
	}

	var (
		gistID   = util.GetID(fname)
		gist     = makeGist(fname)
		filename = filepath.Base(fname)
		content  = util.FileContent(fname)
	)

	res, _, err := g.Client.Gists.Get(gistID)
	if err != nil {
		return url, err
	}

	name := github.GistFilename(filename)
	if *res.Files[name].Content != content {
		gistResp, _, err := g.Client.Gists.Edit(gistID, &gist)
		if err != nil {
			return url, err
		}
		url = *gistResp.HTMLURL
	}

	return url, nil
}

func (g *Gist) Sync(fname string) error {
	var err error

	if len(g.Items) == 0 {
		err = g.getItems()
		if err != nil {
			return err
		}
	}

	item := g.Items.Filter(func(i Item) bool {
		return *i.ID == util.GetID(fname)
	}).One()

	fi, err := os.Stat(fname)
	if err != nil {
		return errors.Wrapf(err, "%s: no such file or directory", fname)
	}

	local := fi.ModTime().UTC()
	remote := item.UpdatedAt.UTC()

	var (
		msg, url string
	)
	if local.After(remote) {
		url, err = g.Upload(fname)
		if err != nil {
			return err
		}
		msg = "Uploaded"
	} else if remote.After(local) {
		url, err = g.Download(fname)
		if err != nil {
			return err
		}
		msg = "Downloaded"
	} else {
		return errors.New("something wrong")
	}
	if Verbose {
		util.Underline(msg, url)
	}

	return nil
}

func (g *Gist) Edit(fname string) error {
	var err error
	// TODO: use pkg/errors

	err = g.Sync(fname)
	if err != nil {
		return err
	}

	// err = config.Conf.Command(config.Conf.Core.Editor, "", fname)
	err = util.RunCommand(config.Conf.Core.Editor, fname)
	if err != nil {
		return err
	}

	err = g.Sync(fname)
	if err != nil {
		return err
	}

	return nil
}

func (gfs *GistFiles) ExtendID(id string) string {
	for _, file := range gfs.Files {
		if file.ShortID == id {
			return file.ID
		}
	}
	return ""
}
