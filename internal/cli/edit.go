package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/babarot/gist/v2/internal/cache"
	"github.com/babarot/gist/v2/internal/edit"
	"github.com/babarot/gist/v2/internal/github"
	"github.com/babarot/gist/v2/internal/model"
)

// EditOptions are the flags of gist edit.
type EditOptions struct {
	Description *string
	Add         []string
	Remove      []string
	Rename      []string
	// Base and BaseDescription say what the change must be based on.
	Base            string
	BaseDescription *string
}

// ErrNeedsEditor says the edit has no content, only an editor could give it.
var ErrNeedsEditor = errors.New("say what to change: pipe the content, or use -d, --add, --remove or --rename")

// ErrFileWithFlags says a file to replace was named along with flags, which
// take no content.
var ErrFileWithFlags = errors.New("a file name goes with piped content, not with -d, --add, --remove or --rename")

// Edit changes a gist without an editor.
func (c *CLI) Edit(ctx context.Context, arg, file string, o EditOptions) error {
	ops := o.Description != nil || len(o.Add) > 0 || len(o.Remove) > 0 || len(o.Rename) > 0
	if ops && file != "" {
		// What is piped would be dropped without a word
		return ErrFileWithFlags
	}
	fromStdin := !ops && !c.StdinTTY
	if !ops && !fromStdin {
		return ErrNeedsEditor
	}
	id, err := c.Resolve(ctx, arg)
	if err != nil {
		return err
	}
	var added map[string]string
	if added, err = addedFiles(o.Add); err != nil {
		return err
	}
	renames, err := parseRenames(o.Rename)
	if err != nil {
		return err
	}
	var stdin []byte
	if fromStdin {
		if stdin, err = io.ReadAll(c.Stdin); err != nil {
			return err
		}
	}
	touched := map[string]bool{}
	res, err := c.Editor.Apply(ctx, edit.Request{
		Gist: id, Base: o.Base, BaseDescription: o.BaseDescription,
		Build: func(ctx context.Context, g model.Gist) (model.Change, error) {
			ch := model.Change{Files: map[string]model.FileChange{}}
			if fromStdin {
				return c.stdinChange(ctx, g, file, stdin, touched)
			}
			if o.Description != nil && *o.Description != g.Description {
				ch.Description = o.Description
			}
			return ch, plan(g, &ch, added, o.Remove, renames, touched)
		},
	})
	if changed, ok := errors.AsType[*edit.ChangedError](err); ok {
		return fmt.Errorf("%w; not sent. It is now at revision %s", changed, changed.Current.Revision())
	}
	if err != nil {
		return err
	}
	if !res.Sent {
		fmt.Fprintln(c.Stderr, "no changes · nothing sent")
		return nil
	}
	c.remembered(ctx, res.Gist)
	if res.Overwritten != "" {
		fmt.Fprintf(c.Stderr, "gist: another change came in just before this one and was overwritten: %s/%s\n",
			res.Gist.URL, res.Overwritten)
	}
	for name := range touched {
		if _, ok, err := c.Editor.Draft(id, name); err == nil && ok {
			fmt.Fprintf(c.Stderr, "gist: %s has a draft, which stays; its base is now out of date\n", name)
		}
	}
	return nil
}

// stdinChange is the file replaced with what came on stdin.
func (c *CLI) stdinChange(ctx context.Context, g model.Gist, file string, stdin []byte, touched map[string]bool) (model.Change, error) {
	if file == "" {
		if len(g.Files) != 1 {
			return model.Change{}, fmt.Errorf("gist %s has %d files: name the one to replace", g.ID, len(g.Files))
		}
		file = g.Files[0].Name
	}
	f, err := fileOf(g, file)
	if err != nil {
		return model.Change{}, err
	}
	content, err := sendable("stdin", stdin)
	if err != nil {
		return model.Change{}, err
	}
	// GitHub makes a revision even for the same content, which only saying
	// nothing avoids. A file too large to compare is sent as it is
	if raw, err := c.API.Raw(ctx, f); err == nil && raw == content {
		return model.Change{}, nil
	}
	touched[file] = true
	return model.Change{Files: map[string]model.FileChange{file: {Content: &content}}}, nil
}

// addedFiles reads the files of --add.
func addedFiles(paths []string) (map[string]string, error) {
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

// parseRenames reads the old=new of --rename.
func parseRenames(args []string) (map[string]string, error) {
	renames := make(map[string]string, len(args))
	for _, a := range args {
		from, to, ok := strings.Cut(a, "=")
		if !ok || from == "" || to == "" {
			return nil, fmt.Errorf("--rename takes old=new, got %q", a)
		}
		renames[from] = to
	}
	return renames, nil
}

// plan adds the changes of files to ch, against the gist as it is. GitHub
// answers some mistakes with success: a rename onto a taken name does nothing
// but make a revision. So they are refused here.
func plan(g model.Gist, ch *model.Change, added map[string]string, removed []string, renames map[string]string, touched map[string]bool) error {
	has := func(name string) bool {
		return slices.ContainsFunc(g.Files, func(f model.File) bool { return f.Name == name })
	}
	for name, content := range added {
		if has(name) {
			return fmt.Errorf("gist %s already has %s", g.ID, name)
		}
		ch.Files[name] = model.FileChange{Content: &content}
	}
	for _, name := range removed {
		if !has(name) {
			return fmt.Errorf("gist %s has no file %q", g.ID, name)
		}
		ch.Files[name] = model.FileChange{Remove: true}
	}
	targets := map[string]bool{}
	for from, to := range renames {
		switch {
		case targets[to]:
			return fmt.Errorf("two files would be named %s", to)
		case !has(from):
			return fmt.Errorf("gist %s has no file %q", g.ID, from)
		case has(to) || added[to] != "":
			return fmt.Errorf("gist %s already has %s", g.ID, to)
		case ch.Files[from].Remove:
			return fmt.Errorf("%s is both removed and renamed", from)
		}
		targets[to] = true
		ch.Files[from] = model.FileChange{Name: &to}
		touched[from] = true
	}
	for _, name := range removed {
		touched[name] = true
	}
	return nil
}

// Delete removes gists, asking first on a terminal. Nothing is removed unless
// every one of them can be found.
func (c *CLI) Delete(ctx context.Context, args []string, yes bool) error {
	ids := make([]string, len(args))
	for i, arg := range args {
		id, err := c.Resolve(ctx, arg)
		if err != nil {
			return err
		}
		ids[i] = id
	}
	if !yes {
		if !c.StdinTTY {
			return errors.New("deleting needs --yes when there is no terminal to ask on")
		}
		for _, id := range ids {
			g, err := c.API.Get(ctx, id)
			if err != nil {
				return err
			}
			fmt.Fprintf(c.Stderr, "%s  %s\n", id[:min(shortID, len(id))], oneLine(g.Description))
		}
		if !c.confirm(fmt.Sprintf("Delete %d gist(s) from GitHub? It cannot be undone [y/N] ", len(ids))) {
			return ErrCanceled
		}
	}
	for _, id := range ids {
		l, err := c.Editor.State.Lock(id)
		if err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
		err = c.API.Delete(ctx, id)
		if releaseErr := l.Release(); err == nil {
			err = releaseErr
		}
		if github.IsNotFound(err) {
			fmt.Fprintf(c.Stderr, "gist: %s was already gone\n", id)
		} else if err != nil {
			return fmt.Errorf("%s: %w", id, err)
		} else {
			fmt.Fprintf(c.Stderr, "✓ deleted %s\n", id)
		}
		c.remember(ctx, func(l *cache.List) {
			l.Gists = slices.DeleteFunc(l.Gists, func(g model.Gist) bool { return g.ID == id })
		})
		c.dropContents(id)
	}
	return nil
}

// confirm asks a question on stderr and reads the answer from stdin.
func (c *CLI) confirm(question string) bool {
	fmt.Fprint(c.Stderr, question)
	var answer string
	_, _ = fmt.Fscanln(c.Stdin, &answer)
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")
}

// OpenGist opens a gist in the browser, or the user's gists when arg is "".
func (c *CLI) OpenGist(ctx context.Context, arg string) error {
	if arg == "" {
		login, err := c.login(ctx)
		if err != nil {
			return err
		}
		return c.Open("https://gist.github.com/" + login)
	}
	id, err := c.Resolve(ctx, arg)
	if err != nil {
		return err
	}
	return c.Open("https://gist.github.com/" + id)
}

// Clone clones a gist with git, the way to work on images and many files,
// which the API cannot carry.
func (c *CLI) Clone(ctx context.Context, arg, dir string) error {
	id, err := c.Resolve(ctx, arg)
	if err != nil {
		return err
	}
	args := []string{"clone", "https://gist.github.com/" + id + ".git"}
	if dir != "" {
		args = append(args, dir)
	}
	return c.Run("git", args...)
}
