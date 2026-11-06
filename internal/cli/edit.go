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
	"github.com/babarot/gist/v2/internal/diff"
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
	// Yes sends what was edited without asking, and Force goes over what
	// changed on GitHub.
	Yes, Force bool
}

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
		return c.editInEditor(ctx, arg, file, o)
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

// editInEditor edits files in the editor: one, or with no name all of them
// together, what changed going in one change.
func (c *CLI) editInEditor(ctx context.Context, arg, file string, o EditOptions) error {
	id, err := c.Resolve(ctx, arg)
	if err != nil {
		return err
	}
	g, err := c.API.Get(ctx, id)
	if err != nil {
		return err
	}
	var names []string
	if file != "" {
		f, err := fileOf(g, file)
		if err != nil {
			return err
		}
		names = []string{f.Name}
	} else {
		for _, f := range g.Files {
			if !f.Binary {
				names = append(names, f.Name)
			}
		}
	}
	if len(names) == 0 {
		return fmt.Errorf("gist %s has no text file to edit: use gist clone", id)
	}
	s, err := c.Editor.Begin(ctx, id, names...)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()
	if err := c.continueDrafts(ctx, s); err != nil {
		return err
	}
	for {
		work, err := s.Workspace()
		if err != nil {
			return err
		}
		args := edit.Command(c.EditorCommand, work.Paths()...)
		if err := c.Run(args[0], args[1:]...); err != nil {
			// What it left is read all the same: it may be all there is
			c.Log.Warn("the editor ended badly", "err", err.Error())
		}
		changed, err := work.ReadBack(s)
		if err != nil {
			return err
		}
		if !changed {
			fmt.Fprintln(c.Stderr, "no changes · nothing sent")
			return nil
		}
		again, err := c.confirmAndPush(ctx, s, o)
		if err != nil || !again {
			return err
		}
	}
}

// continueDrafts asks whether to go on from the drafts the session found.
func (c *CLI) continueDrafts(ctx context.Context, s *edit.Session) error {
	var drafted []string
	for _, f := range s.Files {
		if f.FromDraft {
			drafted = append(drafted, f.Name)
		}
	}
	if len(drafted) == 0 {
		return nil
	}
	fmt.Fprintf(c.Stderr, "%s %s a draft that was not pushed.\n", strings.Join(drafted, ", "),
		map[bool]string{true: "has", false: "have"}[len(drafted) == 1])
	if c.ask("Continue from it, or start from GitHub and discard it? [C/d] ") == "d" {
		return s.DropDrafts(ctx)
	}
	return nil
}

// confirmAndPush shows what would be sent and sends it, and returns whether the
// user wants to go back to the editor.
func (c *CLI) confirmAndPush(ctx context.Context, s *edit.Session, o EditOptions) (bool, error) {
	for {
		c.printDiff(s)
		if !o.Yes {
			switch c.ask("Push this to GitHub? [y/e/N] ") {
			case "y", "yes":
			case "e":
				return true, nil
			default:
				fmt.Fprintln(c.Stderr, "kept as a draft")
				return false, ErrCanceled
			}
		}
		res, err := s.Push(ctx, o.Force)
		if moved, ok := errors.AsType[*edit.ChangedError](err); ok {
			fmt.Fprintf(c.Stderr, "the gist changed on GitHub %s; it is at revision %s.\n",
				ago(c.Now().Sub(moved.Current.Updated))+" ago", moved.Current.Revision())
			switch c.ask("Overwrite it, or reload and show your edit against it? [o/r/N] ") {
			case "o":
				res, err = s.Push(ctx, true)
			case "r":
				changed, err := s.Reload(ctx)
				if err != nil {
					return false, err
				}
				if !changed {
					fmt.Fprintln(c.Stderr, "no changes · nothing sent")
					return false, nil
				}
				continue
			default:
				fmt.Fprintln(c.Stderr, "kept as a draft")
				return false, ErrCanceled
			}
		}
		if err != nil {
			if errors.Is(err, github.ErrEmptyContent) {
				return false, errors.New("GitHub removes an empty file: use --remove to remove it. The edit is kept as a draft")
			}
			return false, err
		}
		c.remembered(ctx, res.Gist)
		if res.Overwritten != "" {
			fmt.Fprintf(c.Stderr, "gist: another change came in just before this one and was overwritten: %s/%s\n",
				res.Gist.URL, res.Overwritten)
		}
		return false, nil
	}
}

// printDiff shows what would be sent, on stderr, as stdout is for data.
func (c *CLI) printDiff(s *edit.Session) {
	for _, f := range s.Changed() {
		d := diff.Compare(f.Original, f.Content)
		fmt.Fprintf(c.Stderr, "--- %s (+%d −%d)\n", oneLine(f.Name), d.Added, d.Removed)
		for _, l := range d.Lines {
			switch l.Kind {
			case diff.Hunk:
				fmt.Fprintln(c.Stderr, l.Text)
			case diff.Added:
				fmt.Fprintln(c.Stderr, "+"+l.Text)
			case diff.Removed:
				fmt.Fprintln(c.Stderr, "-"+l.Text)
			default:
				fmt.Fprintln(c.Stderr, " "+l.Text)
			}
		}
	}
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

// confirm asks a question and says the answer was yes.
func (c *CLI) confirm(question string) bool {
	a := c.ask(question)
	return a == "y" || a == "yes"
}

// ask asks a question on stderr and reads the answer from stdin, in lower case.
func (c *CLI) ask(question string) string {
	fmt.Fprint(c.Stderr, question)
	var answer string
	_, _ = fmt.Fscanln(c.Stdin, &answer)
	return strings.ToLower(answer)
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
