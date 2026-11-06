package cli

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/babarot/gist/v2/internal/model"
)

// ViewOptions are the flags of gist view.
type ViewOptions struct {
	Files, JSON, Web bool
}

// View prints a gist's files as written. The content in GitHub's JSON is
// rewritten by the API client and cut at 1 MB, so it is read from raw_url.
func (c *CLI) View(ctx context.Context, arg, file string, o ViewOptions) error {
	id, err := c.resolve(ctx, arg)
	if err != nil {
		return err
	}
	g, err := c.API.Get(ctx, id)
	if err != nil {
		return err
	}
	if o.Web {
		return c.Open(g.URL)
	}
	files := g.Files
	if file != "" {
		f, err := fileOf(g, file)
		if err != nil {
			return err
		}
		files = []model.File{f}
	}
	if o.Files {
		for _, f := range files {
			fmt.Fprintln(c.Stdout, oneLine(f.Name))
		}
		return nil
	}
	if o.JSON {
		return c.viewJSON(ctx, g, files)
	}
	for i, f := range files {
		raw, err := c.API.Raw(ctx, f)
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		if len(files) > 1 {
			if i > 0 {
				fmt.Fprintln(c.Stdout)
			}
			fmt.Fprintf(c.Stdout, "==> %s <==\n", oneLine(f.Name))
		}
		fmt.Fprint(c.Stdout, raw)
	}
	return nil
}

// viewJSON prints the gist with its files as written. A file JSON cannot hold
// unchanged has no content.
func (c *CLI) viewJSON(ctx context.Context, g model.Gist, files []model.File) error {
	out := struct {
		model.Gist
		Revision string `json:"revision"`
	}{g, g.Revision()}
	out.Files = make([]model.File, len(files))
	for i, f := range files {
		raw, err := c.API.Raw(ctx, f)
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		f.Content, f.Truncated = "", false
		if utf8.ValidString(raw) {
			f.Content = raw
		}
		out.Files[i] = f
	}
	return c.printJSON(out)
}
