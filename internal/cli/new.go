package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/babarot/gist/v2/internal/cache"
	"github.com/babarot/gist/v2/internal/model"
)

// defaultFilename names what comes from stdin, as GitHub does.
const defaultFilename = "gistfile1.txt"

// ErrNothingToCreate says gist new was given nothing to read.
var ErrNothingToCreate = errors.New("give files to create the gist from, or pipe the content")

// NewOptions are the flags of gist new.
type NewOptions struct {
	Description string
	Public      bool
	Filename    string
	Open, Copy  bool
	JSON        bool
}

// New creates a gist from files, or from stdin, and prints its URL.
func (c *CLI) New(ctx context.Context, paths []string, o NewOptions) error {
	files, err := c.newFiles(paths, o.Filename)
	if err != nil {
		return err
	}
	g, err := c.API.Create(ctx, model.NewGist{Description: o.Description, Public: o.Public, Files: files})
	if err != nil {
		return err
	}
	return c.announce(ctx, g, o)
}

// announce keeps a new gist in the list and says where it is.
func (c *CLI) announce(ctx context.Context, g model.Gist, o NewOptions) error {
	c.remember(ctx, func(l *cache.List) {
		g := g
		g.History = nil
		l.Gists = append(l.Gists, g)
	})
	if o.JSON {
		names := make([]string, len(g.Files))
		for i, f := range g.Files {
			names[i] = f.Name
		}
		if err := c.printJSON(struct {
			ID    string   `json:"id"`
			URL   string   `json:"url"`
			Files []string `json:"files"`
		}{g.ID, g.URL, names}); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(c.Stdout, g.URL)
	}
	if o.Copy {
		if err := c.copyURL(g.URL); err != nil {
			fmt.Fprintf(c.Stderr, "gist: %v\n", err)
		}
	}
	if o.Open {
		return c.Open(g.URL)
	}
	return nil
}

// NewInForm runs the form, and says what the form made as New would.
func (c *CLI) NewInForm(ctx context.Context, o NewOptions, form func() (*model.Gist, error)) error {
	g, err := form()
	if err != nil {
		return err
	}
	if g == nil {
		return ErrCanceled
	}
	return c.announce(ctx, *g, o)
}

func (c *CLI) newFiles(paths []string, filename string) (map[string]string, error) {
	if len(paths) == 0 {
		if c.StdinTTY {
			return nil, ErrNothingToCreate
		}
		b, err := io.ReadAll(c.Stdin)
		if err != nil {
			return nil, err
		}
		if filename == "" {
			filename = defaultFilename
		}
		content, err := sendable("stdin", b)
		if err != nil {
			return nil, err
		}
		return map[string]string{filename: content}, nil
	}
	if filename != "" {
		return nil, errors.New("--filename names what comes from stdin, not a file")
	}
	files := make(map[string]string, len(paths))
	for _, p := range paths {
		name := filepath.Base(p)
		if _, taken := files[name]; taken {
			return nil, fmt.Errorf("two files are named %s", name)
		}
		content, err := readFile(p)
		if err != nil {
			return nil, err
		}
		files[name] = content
	}
	return files, nil
}
