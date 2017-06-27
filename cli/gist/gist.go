package gist

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/b4b4r07/gist/api"
	"github.com/b4b4r07/gist/cli"
	"github.com/b4b4r07/gist/cli/config"
	"github.com/google/go-github/github"
)

const BaseURL = "https://gist.github.com"

var (
	Dir string
)

type (
	Client struct {
		gist api.Gist
	}
	Item struct {
		ID          string `json:"id"`
		ShortID     string `json:"short_id"`
		Description string `json:"description"`
		Public      bool   `json:"public"`
		Files       []File `json:"files"`
		// original field
		URL  string `json:"url"`
		Path string `json:"path"`
	}
	Items []Item
	File  struct {
		ItemID   string `json:"item_id"`
		Filename string `json:"filename"`
		Content  string `json:"content"`
		// original field
		Path string `json:"path"`
	}
	Files []File
)

func NewClient(token string) (c *Client, err error) {
	gist, err := api.NewGist(token)
	if err != nil {
		return
	}
	return &Client{gist: *gist}, nil
}

func (c *Client) List() (items Items, err error) {
	s := NewSpinner("Fetching...")
	s.Start()
	defer s.Stop()
	resp, err := c.gist.List()
	if err != nil {
		return
	}
	items = convertItems(resp)
	return
}

func (c *Client) Create(files Files, desc string, private bool) (item Item, err error) {
	s := NewSpinner("Creating...")
	s.Start()
	defer s.Stop()
	gistFiles := make(map[github.GistFilename]github.GistFile, len(files))
	for _, file := range files {
		var (
			filename = file.Filename
			content  = file.Content
			gistname = github.GistFilename(filename)
		)
		gistFiles[gistname] = github.GistFile{
			Filename: &filename,
			Content:  &content,
		}
	}
	resp, err := c.gist.Create(gistFiles, desc, private)
	if err != nil {
		return
	}
	files = Files{}
	for _, file := range resp.Files {
		files = append(files, File{
			Filename: *file.Filename,
			Content:  *file.Content,
			Path:     filepath.Join(Dir, *resp.ID, *file.Filename),
			ItemID:   *resp.ID,
		})
	}
	item = Item{
		ID:          *resp.ID,
		ShortID:     ShortenID(*resp.ID),
		Description: *resp.Description,
		Public:      *resp.Public,
		Files:       files,
		URL:         *resp.HTMLURL,
	}
	err = item.Clone()
	return
}

func (c *Client) Delete(id string) (err error) {
	s := NewSpinner("Deleting...")
	s.Start()
	defer s.Stop()
	return c.gist.Delete(id)
}

type diff int

const (
	diffNone diff = iota
	updateLocal
	updateRemote
)

func (d diff) String() string {
	switch d {
	case diffNone:
		return "No need to do anything"
	case updateLocal:
		return "Need to update remote because local is ahead"
	case updateRemote:
		return "Need to update local because remote is ahead"
	default:
		return ""
	}
}

func (c *Client) compare(file File) (kind diff, content string, err error) {
	var (
		remoteContent, localContent string
	)
	fi, err := os.Stat(file.Path)
	if err != nil {
		// TODO:
		// case -> there is a dir but file has already deleted
		// err = g.Clone(item)
		// err = errors.Wrapf(err, "%s: no such file or directory", fname)
		return
	}

	item, err := c.gist.Get(file.ItemID)
	if err != nil {
		return
	}
	data, _ := ioutil.ReadFile(file.Path)
	localContent = string(data)
	for _, f := range item.Files {
		if *f.Filename != filepath.Base(file.Filename) {
			return diffNone, "", errors.New("something")
		}
		remoteContent = *f.Content
	}
	if remoteContent == localContent {
		return diffNone, "", nil
	}

	local := fi.ModTime().UTC()
	remote := item.UpdatedAt.UTC()

	switch {
	case local.After(remote):
		return updateRemote, localContent, nil
	case remote.After(local):
		return updateLocal, remoteContent, nil
	default:
	}

	return diffNone, "", nil
}

func (c *Client) updateLocal(file File) (err error) {
	return ioutil.WriteFile(file.Path, []byte(file.Content), os.ModePerm)
}

func (c *Client) updateRemote(file File) (err error) {
	files := map[github.GistFilename]github.GistFile{
		github.GistFilename(file.Filename): github.GistFile{
			Content: github.String(file.Content),
		},
	}
	return c.gist.Update(file.ItemID, files)
}

func (c *Client) Sync(file File) (err error) {
	s := NewSpinner("Checking...")
	s.Start()
	defer s.Stop()
	kind, newContent, err := c.compare(file)
	file.Content = newContent
	switch kind {
	case updateLocal:
		err = c.updateLocal(file)
	case updateRemote:
		err = c.updateRemote(file)
	}
	return
}

func (c *Client) Edit(file File) (err error) {
	if err := c.Sync(file); err != nil {
		return err
	}
	editor := config.Conf.Core.Editor
	if editor == "" {
		return cli.ErrConfigEditor
	}
	if err := cli.Run(editor, file.Path); err != nil {
		return err
	}
	return c.Sync(file)
}
