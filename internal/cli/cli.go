// Package cli holds what the subcommands do, once internal/app has parsed
// their arguments. What a command is asked for goes to stdout; what it has to
// say about how that went goes to stderr.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/babarot/gist/v2/internal/cache"
	"github.com/babarot/gist/v2/internal/edit"
	"github.com/babarot/gist/v2/internal/github"
	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/resolve"
)

// ErrCanceled says the user said no. The exit code is 130 and nothing more
// is printed.
var ErrCanceled = errors.New("canceled")

// CLI is what the commands run with.
type CLI struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	// StdinTTY and StdoutTTY say they are terminals.
	StdinTTY, StdoutTTY bool

	API    *github.Client
	Editor *edit.Editor
	Log    *slog.Logger
	// Open opens a URL in the browser.
	Open func(url string) error
	// Run runs a program on the user's terminal.
	Run func(name string, args ...string) error
	// CacheDir is where lists and contents are kept.
	CacheDir string
	// Now is the time.
	Now func() time.Time

	viewer string
}

// login is who the token belongs to, asked once.
func (c *CLI) login(ctx context.Context) (string, error) {
	if c.viewer == "" {
		v, err := c.API.Viewer(ctx)
		if err != nil {
			return "", err
		}
		c.viewer = v
	}
	return c.viewer, nil
}

// store is the file of the user's list, or of the starred one.
func (c *CLI) store(ctx context.Context, name string) (*cache.Store, error) {
	login, err := c.login(ctx)
	if err != nil {
		return nil, err
	}
	return cache.Open(filepath.Join(c.CacheDir, name), github.Host, login), nil
}

// load is the list kept in the store. A list that cannot be read is logged
// and read again, as it would be if it had never been kept.
func (c *CLI) load(s *cache.Store) (cache.List, bool) {
	l, ok, err := s.Load()
	if err != nil {
		c.Log.Warn("the kept list cannot be read", "err", err.Error())
	}
	return l, ok
}

// mine is the list of the user's gists: as kept, or read when there is none or
// when refresh asks for it. Pages that did not change are not read again.
func (c *CLI) mine(ctx context.Context, refresh bool) ([]model.Gist, error) {
	s, err := c.store(ctx, cache.MineFile)
	if err != nil {
		return nil, err
	}
	l, ok := c.load(s)
	if ok && !refresh {
		return l.Gists, nil
	}
	pages, err := c.API.List(ctx, l.ETags(), func(model.ListPage) {})
	if err != nil {
		return nil, err
	}
	l = cache.Merge(l, pages, c.Now())
	c.save(s, l)
	return l.Gists, nil
}

// starred is the list of the gists the user starred, kept the same way.
func (c *CLI) starred(ctx context.Context, refresh bool) ([]model.Gist, error) {
	s, err := c.store(ctx, cache.StarredFile)
	if err != nil {
		return nil, err
	}
	if l, ok := c.load(s); ok && !refresh {
		return l.Gists, nil
	}
	gists, err := c.API.Starred(ctx)
	if err != nil {
		return nil, err
	}
	cache.Sort(gists)
	c.save(s, cache.List{Gists: gists, Read: c.Now()})
	return gists, nil
}

// save keeps a list. A list not kept costs the next run a read, so it is
// not worth failing the command for.
func (c *CLI) save(s *cache.Store, l cache.List) {
	if err := s.Save(l); err != nil {
		c.Log.Warn("the list cannot be kept", "err", err.Error())
	}
}

// Resolve turns what the user wrote into a gist's ID. What is not found in the
// kept list is looked for in a fresh one, as it may have been made elsewhere.
func (c *CLI) Resolve(ctx context.Context, arg string) (string, error) {
	if id, ok := resolve.Direct(arg); ok {
		return id, nil
	}
	gists, err := c.mine(ctx, false)
	if err != nil {
		return "", err
	}
	id, err := resolve.Find(arg, gists)
	if _, notFound := errors.AsType[*resolve.NotFoundError](err); !notFound {
		return id, err
	}
	gists, err = c.mine(ctx, true)
	if err != nil {
		return "", err
	}
	return resolve.Find(arg, gists)
}

// remember updates the kept list after a change made here, so the next read
// is not of a list out of date.
func (c *CLI) remember(ctx context.Context, change func(*cache.List)) {
	s, err := c.store(ctx, cache.MineFile)
	if err != nil {
		c.Log.Warn("the kept list cannot be updated", "err", err.Error())
		return
	}
	if l, ok := c.load(s); ok {
		change(&l)
		cache.Sort(l.Gists)
		c.save(s, l)
	}
}

// remembered puts g in the kept list, in place of the gist it was.
func (c *CLI) remembered(ctx context.Context, g model.Gist) {
	c.remember(ctx, func(l *cache.List) {
		g.History = nil
		for i := range l.Gists {
			if l.Gists[i].ID == g.ID {
				l.Gists[i] = g
				return
			}
		}
		l.Gists = append(l.Gists, g)
	})
	c.dropContents(g.ID)
}

func (c *CLI) dropContents(id string) {
	if err := cache.OpenContents(filepath.Join(c.CacheDir, "contents")).Remove(id); err != nil {
		c.Log.Warn("the kept content cannot be removed", "gist", id, "err", err.Error())
	}
}

// fileOf finds a file by name.
func fileOf(g model.Gist, name string) (model.File, error) {
	for _, f := range g.Files {
		if f.Name == name {
			return f, nil
		}
	}
	return model.File{}, fmt.Errorf("gist %s has no file %q", g.ID, name)
}

// readFile reads a file to send, refusing what the API cannot take.
func readFile(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return "", fmt.Errorf("%s is a directory", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sendable(path, b)
}

// sendable checks content can be sent through the API, which carries text
// and takes no empty file.
func sendable(name string, b []byte) (string, error) {
	if len(b) == 0 {
		return "", fmt.Errorf("%s is empty, which GitHub does not take", name)
	}
	if strings.ContainsRune(string(b), 0) || !utf8.Valid(b) {
		return "", fmt.Errorf("%s is binary, which the GitHub API cannot carry; use gist clone and git push", name)
	}
	return string(b), nil
}

// oneLine keeps text from GitHub to one line of a list, and from reaching the
// terminal as an escape sequence.
func oneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return ' '
		}
		return r
	}, s)
}
