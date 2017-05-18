package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/b4b4r07/gist/api"
)

var Filename = "cache.json"

type Cache struct {
	Path string
	TTL  time.Duration
	Use  bool
}

func NewCache() *Cache {
	return &Cache{
		Path: filepath.Join(Conf.Gist.Dir, Filename),
		TTL:  Conf.Gist.CacheTTL * time.Minute,
		Use:  Conf.Gist.UseCache,
	}
}

func (c *Cache) Remove() error {
	return os.Remove(c.Path)
}

func (c *Cache) Create(files api.Files) error {
	f, err := os.Create(c.Path)
	if err != nil {
		return err
	}
	return json.NewEncoder(f).Encode(&files)
}

func (c *Cache) GetFiles() (files api.Files, err error) {
	f, err := os.Open(c.Path)
	if err != nil {
		return
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(&files)
	if err != nil {
		return
	}
	return
}

func (c *Cache) Expired() bool {
	if c.TTL == 0 {
		// if ttl is not set or equals zero,
		// it's regard as not caching
		return false
	}
	fi, err := os.Stat(c.Path)
	if err != nil {
		return true
	}
	life := fi.ModTime().Add(c.TTL)
	return life.Before(time.Now())
}
