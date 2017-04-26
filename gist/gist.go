package gist

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/b4b4r07/gist/config"
	"github.com/b4b4r07/gist/util"
	"github.com/briandowns/spinner"
	"github.com/google/go-github/github"
	"github.com/mattn/go-runewidth"
	"github.com/pkg/errors"

	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/oauth2"
)

var (
	SpinnerSymbol int = 14
)

type (
	Items []*github.Gist
	Item  *github.Gist
)

type Gist struct {
	Client *github.Client
	Items  Items
	Config Config
}

type Config struct {
	ShowSpinner bool
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

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)

	return &Gist{
		Client: client,
		Items:  []*github.Gist{},
		Config: Config{
			ShowSpinner: config.Conf.Flag.ShowSpinner,
		},
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

func (g *Gist) getStarredItems() error {
	var items Items

	// Get items from gist.github.com
	gists, resp, err := g.Client.Gists.ListStarred(&github.GistListOptions{})
	if err != nil {
		return errorWrapper(err)
		// return err
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

func getSize() (int, error) {
	w, _, err := terminal.GetSize(int(os.Stdout.Fd()))
	return w, err
}

func (g *Gist) GetRemoteFiles() (gfs GistFiles, err error) {
	if g.Config.ShowSpinner {
		s := spinner.New(spinner.CharSets[SpinnerSymbol], 100*time.Millisecond)
		s.Suffix = " Fetching..."
		s.Start()
		defer s.Stop()
	}

	// fetch remote files
	if config.Conf.Flag.OpenStarredItems {
		err = g.getStarredItems()
	} else {
		err = g.getItems()
	}
	if err != nil {
		return gfs, err
	}

	var files Files
	for _, item := range g.Items {
		if err := cloneGist(item); err != nil {
			continue
		}
		desc := ""
		if item.Description != nil {
			desc = *item.Description
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
	width, _ := getSize()
	if config.Conf.Core.ShowIndicator {
		format = fmt.Sprintf(" %%s %%-%ds\t%%-%ds\t%%s\n", util.LengthID, length)
	}
	width = width - util.LengthID - length
	// TODO
	if width > 50 {
		width -= 10
	}
	for i, file := range files {
		desc := runewidth.Truncate(strings.Replace(file.Description, "\n", " ", -1), width-3, "...")
		if config.Conf.Core.ShowIndicator {
			text += fmt.Sprintf(format, prefixes[i], util.ShortenID(file.ID), file.Filename, desc)
		} else {
			text += fmt.Sprintf(format, util.ShortenID(file.ID), file.Filename, desc)
		}
	}

	return GistFiles{
		Files: files,
		Text:  text,
	}, nil
}

func (g *Gist) Create(files Files, desc string) (url string, err error) {
	if g.Config.ShowSpinner {
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
	item, resp, err := g.Client.Gists.Create(&github.Gist{
		Files:       gistFiles,
		Public:      &public,
		Description: &desc,
	})
	if resp == nil {
		return url, errors.New("Try again when you have a better network connection")
	}
	err = cloneGist(item)
	if err != nil {
		return url, err
	}
	url = *item.HTMLURL
	return url, errors.Wrap(err, "Failed to create")
}

func cloneGist(item *github.Gist) error {
	if util.Exists(*item.ID) {
		return nil
	}

	oldwd, _ := os.Getwd()
	dir := config.Conf.Gist.Dir
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.Mkdir(dir, 0700)
	}
	os.Chdir(dir)
	defer os.Chdir(oldwd)

	// TODO: Start()
	return exec.Command("git", "clone", *item.GitPullURL).Start()
}

func (g *Gist) Delete(id string) error {
	if g.Config.ShowSpinner {
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

func (g *Gist) download(fname string) (done bool, err error) {
	gists := g.Items.Filter(func(i Item) bool {
		return *i.ID == util.GetID(fname)
	})

	for _, gist := range *gists {
		g, _, err := g.Client.Gists.Get(*gist.ID)
		if err != nil {
			return done, err
		}
		// for multiple files in one Gist folder
		for _, f := range g.Files {
			fpath := filepath.Join(config.Conf.Gist.Dir, *gist.ID, *f.Filename)
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

func makeGist(fname string) github.Gist {
	body := util.FileContent(fname)
	return github.Gist{
		Files: map[github.GistFilename]github.GistFile{
			github.GistFilename(filepath.Base(fname)): github.GistFile{
				Content: github.String(body),
			},
		},
	}
}

func (g *Gist) upload(fname string) (done bool, err error) {
	var (
		gistID   = util.GetID(fname)
		gist     = makeGist(fname)
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

	if g.Config.ShowSpinner {
		s := spinner.New(spinner.CharSets[SpinnerSymbol], 100*time.Millisecond)
		s.Suffix = " Checking..."
		s.Start()
		defer func() {
			s.Stop()
			util.Underline(msg, path.Join(config.Conf.Core.BaseURL, util.GetID(fname)))
		}()
	}

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

func (g *Gist) EditDesc(id, desc string) error {
	if g.Config.ShowSpinner {
		s := spinner.New(spinner.CharSets[SpinnerSymbol], 100*time.Millisecond)
		s.Suffix = " Editing..."
		s.Start()
		defer s.Stop()
	}
	item := github.Gist{
		Description: github.String(desc),
	}
	_, _, err := g.Client.Gists.Edit(id, &item)
	return err
}

func (g *Gist) Edit(fname string) error {
	if err := g.Sync(fname); err != nil {
		return err
	}

	editor := config.Conf.Core.Editor
	if editor == "" {
		return errors.New("$EDITOR: not set")
	}

	err := util.RunCommand(editor, fname)
	if err != nil {
		return err
	}

	return g.Sync(fname)
}

func (gfs *GistFiles) ExtendID(id string) string {
	for _, file := range gfs.Files {
		if file.ShortID == id {
			return file.ID
		}
	}
	return ""
}

func (g *Gist) GetItem(id string) Item {
	return g.Items.Filter(func(i Item) bool {
		return *i.ID == id
	}).One()
}

type ParsedLine struct {
	ID, Filename, Description, Path string
}

func (g *Gist) ParseLine(line string) (*ParsedLine, error) {
	l := strings.Split(line, "\t")
	if len(l) != 3 {
		return &ParsedLine{}, errors.New("error")
	}
	var (
		id = func(id string) string {
			id = strings.TrimSpace(id)
			id = strings.TrimLeft(id, " | ")
			id = strings.TrimLeft(id, " + ")
			return id
		}(l[0])
		filename    = strings.TrimSpace(l[1])
		description = l[2]
	)

	// Convert to full id
	for _, item := range g.Items {
		if strings.HasPrefix(*item.ID, id) {
			id = *item.ID
			continue
		}
	}

	return &ParsedLine{
		ID:          id,
		Filename:    filename,
		Description: description,
		Path:        filepath.Join(id, filename),
	}, nil
}
