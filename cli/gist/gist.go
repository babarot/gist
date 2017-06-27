package gist

import (
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
	return
}

func (c *Client) Delete(id string) (err error) {
	s := NewSpinner("Deleting...")
	s.Start()
	defer s.Stop()
	return c.gist.Delete(id)
}

func (c *Client) Get(id string) {
}

func (c *Client) compare(file File) {
	c.Get(file.ItemID)
}

func (c *Client) sync(file File) error {
	c.compare(file)
	return nil
}

func (c *Client) Edit(file File) (err error) {
	if err := c.sync(file); err != nil {
		return err
	}
	editor := config.Conf.Core.Editor
	if editor == "" {
		return cli.ErrConfigEditor
	}
	if err := cli.Run(editor, file.Path); err != nil {
		return err
	}
	return c.sync(file)
}
