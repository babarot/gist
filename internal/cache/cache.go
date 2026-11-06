// Package cache keeps what was read from GitHub, so the next launch shows it
// at once and asks only for what changed. All of it can be read again: losing
// it costs a slower start, nothing more. Drafts are not kept here.
package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/babarot/gist/v2/internal/model"
)

// version is the format of the files; a file of another one is not used.
const version = 1

// Page is a page of the list as last read.
type Page struct {
	IDs  []string `json:"ids"`
	ETag string   `json:"etag"`
}

// List is a list of gists as last read, without their content, newest update
// first. Only the list of the user's own gists has Pages.
type List struct {
	Gists []model.Gist `json:"gists"`
	Pages []Page       `json:"pages,omitempty"`
	Read  time.Time    `json:"read"`
}

// ETags are those of Pages, the first page's first.
func (l List) ETags() []string {
	etags := make([]string, len(l.Pages))
	for i, p := range l.Pages {
		etags[i] = p.ETag
	}
	return etags
}

// Merge is the list that pages, just read and in any order, make of the list
// l. A page that was not modified holds what it held in l.
func Merge(l List, pages []model.ListPage, read time.Time) List {
	known := make(map[string]model.Gist, len(l.Gists))
	for _, g := range l.Gists {
		known[g.ID] = g
	}
	out := List{Read: read, Pages: make([]Page, 0, len(pages))}
	seen := map[string]bool{}
	add := func(g model.Gist) {
		// While pages are read a new gist can move one onto the next page
		if !seen[g.ID] {
			seen[g.ID] = true
			out.Gists = append(out.Gists, g)
		}
	}
	for _, p := range pages {
		page := Page{ETag: p.ETag}
		if p.NotModified && p.Number >= 1 && p.Number <= len(l.Pages) {
			page.IDs = l.Pages[p.Number-1].IDs
			for _, id := range page.IDs {
				if g, ok := known[id]; ok {
					add(g)
				}
			}
		}
		for _, g := range p.Gists {
			page.IDs = append(page.IDs, g.ID)
			add(g)
		}
		out.Pages = append(out.Pages, page)
	}
	Sort(out.Gists)
	return out
}

// Sort orders gists by their last update, newest first.
func Sort(gists []model.Gist) {
	slices.SortStableFunc(gists, func(a, b model.Gist) int { return b.Updated.Compare(a.Updated) })
}

// file is a list on disk, with whose it is.
type file struct {
	Version int    `json:"version"`
	Host    string `json:"host"`
	Viewer  string `json:"viewer"`
	List
}

// Store is one file holding a list for one account on one host.
type Store struct {
	path, host, viewer string
}

// Open is the store in path for viewer on host.
func Open(path, host, viewer string) *Store {
	return &Store{path: path, host: host, viewer: viewer}
}

// Load reads the list. A missing file, or one of another format, host or
// viewer, gives no list. One that cannot be read gives no list and the error,
// for the log.
func (s *Store) Load() (List, bool, error) {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return List{}, false, nil
	}
	if err != nil {
		return List{}, false, err
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		return List{}, false, fmt.Errorf("%s: %w", s.path, err)
	}
	if f.Version != version || f.Host != s.host || f.Viewer != s.viewer {
		return List{}, false, nil
	}
	return f.List, true, nil
}

// Save replaces the list. The content of a file is left out: it is kept by
// Contents, and only for as long as it is current.
func (s *Store) Save(l List) error {
	l.Gists = slices.Clone(l.Gists)
	for i := range l.Gists {
		l.Gists[i].Files = slices.Clone(l.Gists[i].Files)
		for j := range l.Gists[i].Files {
			l.Gists[i].Files[j].Content = ""
		}
	}
	return write(s.path, file{Version: version, Host: s.host, Viewer: s.viewer, List: l})
}

// Contents keeps gists read whole, for showing, one file each in a directory.
type Contents struct {
	dir string
}

// OpenContents is the contents in dir.
func OpenContents(dir string) *Contents { return &Contents{dir: dir} }

func (c *Contents) path(id string) string { return filepath.Join(c.dir, id+".json") }

// Load reads the gist as kept, if it is still as of updated, the time the list
// says it last changed.
func (c *Contents) Load(id string, updated time.Time) (model.Gist, bool) {
	b, err := os.ReadFile(c.path(id))
	if err != nil {
		return model.Gist{}, false
	}
	var f struct {
		Version int `json:"version"`
		model.Gist
	}
	if json.Unmarshal(b, &f) != nil || f.Version != version || !f.Updated.Equal(updated) {
		return model.Gist{}, false
	}
	return f.Gist, true
}

// Save keeps g.
func (c *Contents) Save(g model.Gist) error {
	return write(c.path(g.ID), struct {
		Version int `json:"version"`
		model.Gist
	}{version, g})
}

// Remove drops what is kept of the gist, which need not be there.
func (c *Contents) Remove(id string) error {
	return ignoreMissing(os.Remove(c.path(id)))
}

func ignoreMissing(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// write replaces path through a temporary file of its own, so two gist
// writing at once never rename each other's half-written file. Only the user
// may read it, as it names secret gists.
func write(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
