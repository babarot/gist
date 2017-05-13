package api

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	// "github.com/b4b4r07/gist/cli"
	"github.com/b4b4r07/gist/util"
	"github.com/briandowns/spinner"
	"github.com/google/go-github/github"
	// "github.com/mattn/go-runewidth"
	"github.com/pkg/errors"

	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/oauth2"
)

var (
	SpinnerSymbol int = 14
	IDLength      int = 9
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
	ShowSpinner       bool
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
			// ShowSpinner:       cli.Conf.Flag.ShowSpinner,
			// OpenStarredItems:  cli.Conf.Flag.OpenStarredItems,
			// ShowIndicator:     cli.Conf.Flag.ShowSpinner,
			// NewPrivate:        cli.Conf.Flag.NewPrivate,
			// Dir:               cli.Conf.Gist.Dir,
			// BaseURL:           cli.Conf.Core.BaseURL,
			// Editor:            cli.Conf.Core.Editor,
			// ShowPrivateSymbol: cli.Conf.Screen.ShowPrivateSymbol,
			Dir:     "/Users/b4b4r07/.config/gist/files",
			BaseURL: "https://gist.github.com",
			Editor:  "vim",
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

func (g *Gist) Get() error {
	return g.getItems()
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

func (g *Gist) Create(files Files, desc string) (url string, err error) {
	if g.Config.ShowSpinner {
		s := spinner.New(spinner.CharSets[SpinnerSymbol], 100*time.Millisecond)
		s.Suffix = " Creating..."
		s.Start()
		defer s.Stop()
	}

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
	err = g.cloneGist(item)
	if err != nil {
		return url, err
	}
	url = *item.HTMLURL
	return url, errors.Wrap(err, "Failed to create")
}

func (g *Gist) cloneGist(item *github.Gist) error {
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
		gistID   = getID(fname)
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
			util.Underline(msg, path.Join(g.Config.BaseURL, getID(fname)))
		}()
	}

	if len(g.Items) == 0 {
		err = g.getItems()
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

	editor := g.Config.Editor
	if editor == "" {
		return errors.New("$EDITOR: not set")
	}

	err := util.RunCommand(editor, fname)
	if err != nil {
		return err
	}

	return g.Sync(fname)
}

func (g *Gist) GetItem(id string) Item {
	return g.Items.Filter(func(i Item) bool {
		return *i.ID == id
	}).One()
}

func (g *Gist) expandID(shortID string) (longID string, err error) {
	if len(g.Files) == 0 {
		return "", errors.New("no gist items")
	}
	for _, file := range g.Files {
		if shortID == file.ShortID {
			longID = file.ID
		}
	}
	if longID == "" {
		err = errors.New("no matched id in fetched items")
	}
	return longID, err
}

func (g *Gist) ParseLine(line string) (*File, error) {
	lineItems := []string{
		"id", "filename", "description",
		// Example Line:
		//   89fbb2c227    bashpipe.go    Execute Piped Shell Commands in Go
	}

	l := strings.Split(line, "\t")
	if len(l) != len(lineItems) {
		return &File{}, errors.New("Failed to parse the selected line")
	}

	trimDirSymbol := func(id string) string {
		id = strings.TrimSpace(id)
		id = strings.TrimLeft(id, " | ")
		id = strings.TrimLeft(id, " + ")
		return id
	}
	trimPrivateSymbol := func(filename string) string {
		filename = strings.TrimSpace(filename)
		filename = strings.TrimLeft(filename, "* ")
		return filename
	}

	var (
		id          = trimDirSymbol(l[0])
		filename    = trimPrivateSymbol(l[1])
		description = l[2]
	)

	id, err := g.expandID(id)
	if err != nil {
		return &File{}, err
	}

	return &File{
		ID:          id,
		Filename:    filename,
		Path:        filepath.Join(id, filename),
		Description: description,
		Content:     "", // no need
	}, nil
}
