package gist

import (
	"encoding/json"
	"os"
)

type cache struct {
	Pages []Page `json:"pages"`

	path string
}

func newCache(path string) *cache {
	return &cache{path: path, Pages: []Page{}}
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

func (c *cache) delete() error {
	return os.Remove(c.path)
}
