package gist

import (
	"encoding/json"
	"os"
)

type Cache struct {
	Token string `json:"token"`
	Pages []Page `json:"pages"`
	Path  string `json:"-"`
}

func NewCache(path string) *Cache {
	return &Cache{
		Token: os.Getenv("GITHUB_TOKEN"),
		Pages: []Page{},
		Path:  path,
	}
}

func (c *Cache) Open() error {
	f, err := os.Open(c.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(&c)
}

func (c *Cache) Save(pages []Page) error {
	f, err := os.Create(c.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	c.Pages = pages
	return json.NewEncoder(f).Encode(&c)
}

func (c *Cache) Delete() error {
	return os.Remove(c.Path)
}
