package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/b4b4r07/gist/cli/gist"
)

var Filename = "cache.json"

type Cache struct {
	Ready   bool
	Path    string
	TTL     time.Duration
	Use     bool
	Updated time.Time
}

func NewCache() *Cache {
	var (
		ready   bool
		updated time.Time
	)
	path := filepath.Join(Conf.Gist.Dir, Filename)
	fi, err := os.Stat(path)
	if err == nil {
		ready = true
		updated = fi.ModTime()
	}
	return &Cache{
		Ready:   ready,
		Path:    path,
		TTL:     Conf.Gist.CacheTTL * time.Minute,
		Use:     Conf.Gist.UseCache,
		Updated: updated,
	}
}

func (c *Cache) Clear() error {
	return os.Remove(c.Path)
}

func (c *Cache) Cache(items gist.Items) error {
	f, err := os.Create(c.Path)
	if err != nil {
		return err
	}
	return json.NewEncoder(f).Encode(&items)
}

func (c *Cache) Load() (items gist.Items, err error) {
	f, err := os.Open(c.Path)
	if err != nil {
		return
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(&items)
	c.pseudoRun()
	return
}

func (c *Cache) Expired() bool {
	if c.TTL == 0 {
		// if TTL is not set or equals zero,
		// it's regard as not caching
		return false
	}
	if !c.Ready {
		// if cache doesn't exist,
		// it's regard as expired
		return true
	}
	ttl := c.Updated.Add(c.TTL)
	return ttl.Before(time.Now())
}

func (c *Cache) Available() bool {
	return c.Use && c.Ready && !c.Expired()
}

func (c *Cache) pseudoRun() {
	time.Sleep(150 * time.Millisecond)
}
