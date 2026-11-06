package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/babarot/gist/v2/internal/edit"
	"github.com/babarot/gist/v2/internal/fakegist"
	"github.com/babarot/gist/v2/internal/github"
	"github.com/babarot/gist/v2/internal/logging"
	"github.com/babarot/gist/v2/internal/state"
)

var ctx = context.Background()

// terminal is a CLI as run on a terminal, with what the user will type.
func terminal(t *testing.T, typed string) (*CLI, *fakegist.Server, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	s := fakegist.New(t)
	api, err := github.New(logging.Discard(), github.Options{Token: "t", Transport: s.Transport()})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	dir := t.TempDir()
	return &CLI{
		Stdin: strings.NewReader(typed), Stdout: &out, Stderr: &errOut,
		StdinTTY: true, StdoutTTY: true,
		API:      api,
		Editor:   &edit.Editor{API: api, State: state.Open(filepath.Join(dir, "state"))},
		Log:      logging.Discard(),
		CacheDir: filepath.Join(dir, "cache"),
		Now:      func() time.Time { return fakegist.Epoch.Add(3 * time.Hour) },
		Open:     func(string) error { return errors.New("no browser in a test") },
		Run:      func(string, ...string) error { return errors.New("no program in a test") },
	}, s, &out, &errOut
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteAsksOnATerminal(t *testing.T) {
	for _, tt := range []struct {
		typed   string
		deleted bool
	}{{"y\n", true}, {"yes\n", true}, {"n\n", false}, {"\n", false}, {"", false}} {
		t.Run(tt.typed, func(t *testing.T) {
			c, s, _, errOut := terminal(t, tt.typed)
			g := s.Add("the description", false, map[string]string{"a.txt": "a"})
			err := c.Delete(ctx, []string{g.ID}, false)
			if tt.deleted && err != nil {
				t.Fatal(err)
			}
			if !tt.deleted && !errors.Is(err, ErrCanceled) {
				t.Errorf("got %v", err)
			}
			if got := s.Get(g.ID) == nil; got != tt.deleted {
				t.Errorf("deleted is %v", got)
			}
			if !strings.Contains(errOut.String(), "the description") || !strings.Contains(errOut.String(), "cannot be undone") {
				t.Errorf("stderr is %q", errOut)
			}
		})
	}
}

func TestListTableOnATerminal(t *testing.T) {
	c, s, out, _ := terminal(t, "")
	s.Add("a long description "+strings.Repeat("x", 80), true, map[string]string{"a.txt": "a"})
	g := s.Add("", false, map[string]string{"named.txt": "n"})
	s.Comment(g.ID, "bob", "hi")
	if err := c.List(ctx, ListOptions{}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "ID ") {
		t.Fatalf("got %q", out)
	}
	if !strings.HasPrefix(lines[1], g.ID[:7]+" ") || !strings.Contains(lines[1], "named.txt") || !strings.Contains(lines[1], "secret") || !strings.HasSuffix(lines[1], "2h") {
		t.Errorf("a gist with no description: %q", lines[1])
	}
	if !strings.Contains(lines[2], "…") {
		t.Errorf("a long description: %q", lines[2])
	}
}

func TestAgo(t *testing.T) {
	for d, want := range map[time.Duration]string{
		5 * time.Second:       "now",
		5 * time.Minute:       "5m",
		3 * time.Hour:         "3h",
		48 * time.Hour:        "2d",
		120 * 24 * time.Hour:  "4mo",
		3650 * 24 * time.Hour: "10y",
	} {
		if got := ago(d); got != want {
			t.Errorf("%v: got %q, want %q", d, got, want)
		}
	}
}

func TestOpen(t *testing.T) {
	c, s, _, _ := terminal(t, "")
	g := s.Add("", false, map[string]string{"a.txt": "a"})
	var opened []string
	c.Open = func(url string) error { opened = append(opened, url); return nil }
	if err := c.OpenGist(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.OpenGist(ctx, ""); err != nil {
		t.Fatal(err)
	}
	want := []string{"https://gist.github.com/" + g.ID, "https://gist.github.com/alice"}
	if !slices.Equal(opened, want) {
		t.Errorf("got %v", opened)
	}
}

func TestClone(t *testing.T) {
	c, s, _, _ := terminal(t, "")
	g := s.Add("", false, map[string]string{"a.txt": "a"})
	var ran [][]string
	c.Run = func(name string, args ...string) error {
		ran = append(ran, append([]string{name}, args...))
		return nil
	}
	if err := c.Clone(ctx, g.ID, ""); err != nil {
		t.Fatal(err)
	}
	if err := c.Clone(ctx, g.ID, "dir"); err != nil {
		t.Fatal(err)
	}
	url := "https://gist.github.com/" + g.ID + ".git"
	if !slices.Equal(ran[0], []string{"git", "clone", url}) || !slices.Equal(ran[1], []string{"git", "clone", url, "dir"}) {
		t.Errorf("ran %v", ran)
	}
}

// editing is what the editor does to the files it is given.
func editing(c *CLI, write map[string]string) {
	c.Run = func(_ string, args ...string) error {
		for _, p := range args {
			if s, ok := write[filepath.Base(p)]; ok {
				if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func TestEditOneFileInTheEditor(t *testing.T) {
	c, s, _, errOut := terminal(t, "y\n")
	g := s.Add("", false, map[string]string{"a.txt": "old\n", "b.txt": "b\n"})
	editing(c, map[string]string{"a.txt": "new\n"})
	if err := c.Edit(ctx, g.ID, "a.txt", EditOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := s.Get(g.ID).Revisions[0].Files["a.txt"]; got != "new\n" {
		t.Errorf("got %q", got)
	}
	for _, want := range []string{"-old", "+new", "(+1 −1)"} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("no %q in %q", want, errOut)
		}
	}
}

func TestEditAllFilesGoInOneChange(t *testing.T) {
	c, s, _, _ := terminal(t, "y\n")
	g := s.Add("", false, map[string]string{"a.txt": "a", "b.txt": "b", "c.txt": "c"})
	editing(c, map[string]string{"a.txt": "A", "b.txt": "B"})
	if err := c.Edit(ctx, g.ID, "", EditOptions{}); err != nil {
		t.Fatal(err)
	}
	got := s.Get(g.ID)
	files := got.Revisions[0].Files
	if len(got.Revisions) != 2 || files["a.txt"] != "A" || files["b.txt"] != "B" || files["c.txt"] != "c" {
		t.Errorf("%d revisions, files %q", len(got.Revisions), files)
	}
}

func TestEditWithNothingChanged(t *testing.T) {
	c, s, _, errOut := terminal(t, "")
	g := s.Add("", false, map[string]string{"a.txt": "a"})
	editing(c, nil)
	if err := c.Edit(ctx, g.ID, "", EditOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(s.Get(g.ID).Revisions) != 1 || !strings.Contains(errOut.String(), "nothing sent") {
		t.Errorf("stderr %q", errOut)
	}
}

func TestEditSaidNoKeepsADraft(t *testing.T) {
	c, s, _, errOut := terminal(t, "n\n")
	g := s.Add("", false, map[string]string{"a.txt": "a"})
	editing(c, map[string]string{"a.txt": "edited"})
	if err := c.Edit(ctx, g.ID, "", EditOptions{}); !errors.Is(err, ErrCanceled) {
		t.Fatalf("got %v", err)
	}
	if d, ok, _ := c.Editor.Draft(g.ID, "a.txt"); !ok || string(d.Content) != "edited" {
		t.Errorf("no draft: %+v", d)
	}
	if !strings.Contains(errOut.String(), "kept as a draft") {
		t.Errorf("stderr %q", errOut)
	}
}

func TestEditYesAsksNothing(t *testing.T) {
	c, s, _, _ := terminal(t, "")
	g := s.Add("", false, map[string]string{"a.txt": "a"})
	editing(c, map[string]string{"a.txt": "edited"})
	if err := c.Edit(ctx, g.ID, "", EditOptions{Yes: true}); err != nil {
		t.Fatal(err)
	}
	if got := s.Get(g.ID).Revisions[0].Files["a.txt"]; got != "edited" {
		t.Errorf("got %q", got)
	}
}

func TestEditAnEditorThatFailsKeepsTheEdit(t *testing.T) {
	c, s, _, _ := terminal(t, "n\n")
	g := s.Add("", false, map[string]string{"a.txt": "a"})
	c.Run = func(_ string, args ...string) error {
		if err := os.WriteFile(args[0], []byte("edited"), 0o600); err != nil {
			return err
		}
		return errors.New("the editor exploded")
	}
	if err := c.Edit(ctx, g.ID, "", EditOptions{}); !errors.Is(err, ErrCanceled) {
		t.Fatalf("got %v", err)
	}
	if _, ok, _ := c.Editor.Draft(g.ID, "a.txt"); !ok {
		t.Error("the edit is gone")
	}
}

func TestEditContinuesFromADraft(t *testing.T) {
	c, s, _, _ := terminal(t, "n\n\ny\n")
	g := s.Add("", false, map[string]string{"a.txt": "a"})
	editing(c, map[string]string{"a.txt": "drafted"})
	if err := c.Edit(ctx, g.ID, "", EditOptions{}); !errors.Is(err, ErrCanceled) {
		t.Fatalf("got %v", err)
	}
	var saw string
	c.Run = func(_ string, args ...string) error {
		b, err := os.ReadFile(args[0])
		saw = string(b)
		return err
	}
	if err := c.Edit(ctx, g.ID, "", EditOptions{}); err != nil {
		t.Fatal(err)
	}
	if saw != "drafted" {
		t.Errorf("the editor got %q", saw)
	}
	if got := s.Get(g.ID).Revisions[0].Files["a.txt"]; got != "drafted" {
		t.Errorf("got %q", got)
	}
}

func TestEditCanDiscardADraft(t *testing.T) {
	c, s, _, _ := terminal(t, "n\nd\n")
	g := s.Add("", false, map[string]string{"a.txt": "a"})
	editing(c, map[string]string{"a.txt": "drafted"})
	if err := c.Edit(ctx, g.ID, "", EditOptions{}); !errors.Is(err, ErrCanceled) {
		t.Fatalf("got %v", err)
	}
	var saw string
	c.Run = func(_ string, args ...string) error {
		b, err := os.ReadFile(args[0])
		saw = string(b)
		return err
	}
	if err := c.Edit(ctx, g.ID, "", EditOptions{}); err != nil {
		t.Fatal(err)
	}
	if saw != "a" {
		t.Errorf("the editor got %q", saw)
	}
	if _, ok, _ := c.Editor.Draft(g.ID, "a.txt"); ok {
		t.Error("the draft stayed")
	}
}

func TestEditConflict(t *testing.T) {
	for _, tt := range []struct {
		name, typed string
		o           EditOptions
		want        string
		canceled    bool
	}{
		{"overwrite", "y\no\n", EditOptions{}, "mine", false},
		{"force", "y\n", EditOptions{Force: true}, "mine", false},
		{"no", "y\n\n", EditOptions{}, "theirs", true},
		{"reload and push", "y\nr\ny\n", EditOptions{}, "mine", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, s, _, _ := terminal(t, tt.typed)
			g := s.Add("", false, map[string]string{"a.txt": "a"})
			c.Run = func(_ string, args ...string) error {
				s.Edit(g.ID, "a.txt", "theirs")
				return os.WriteFile(args[0], []byte("mine"), 0o600)
			}
			err := c.Edit(ctx, g.ID, "", tt.o)
			if tt.canceled != errors.Is(err, ErrCanceled) || (!tt.canceled && err != nil) {
				t.Fatalf("got %v", err)
			}
			if got := s.Get(g.ID).Revisions[0].Files["a.txt"]; got != tt.want {
				t.Errorf("got %q", got)
			}
		})
	}
}

func TestEditEmptiedFileIsNotRemoved(t *testing.T) {
	c, s, _, _ := terminal(t, "y\n")
	g := s.Add("", false, map[string]string{"a.txt": "a", "b.txt": "b"})
	editing(c, map[string]string{"a.txt": ""})
	err := c.Edit(ctx, g.ID, "", EditOptions{})
	if err == nil || !strings.Contains(err.Error(), "removes an empty file") {
		t.Fatalf("got %v", err)
	}
	if _, ok := s.Get(g.ID).Revisions[0].Files["a.txt"]; !ok {
		t.Error("the file was removed")
	}
	if _, ok, _ := c.Editor.Draft(g.ID, "a.txt"); !ok {
		t.Error("the edit is not in a draft")
	}
}

func TestEditOnlyBinaryFiles(t *testing.T) {
	c, s, _, _ := terminal(t, "")
	g := s.Add("", false, map[string]string{"a.png": "\x00\x01"})
	if err := c.Edit(ctx, g.ID, "", EditOptions{}); err == nil || !strings.Contains(err.Error(), "gist clone") {
		t.Errorf("got %v", err)
	}
}

func TestNewOnATerminalNeedsSomethingToRead(t *testing.T) {
	c, _, _, _ := terminal(t, "")
	if err := c.New(ctx, nil, NewOptions{}); !errors.Is(err, ErrNothingToCreate) {
		t.Errorf("got %v", err)
	}
}

func TestCopyGoesThroughStderr(t *testing.T) {
	c, _, out, errOut := terminal(t, "")
	path := filepath.Join(t.TempDir(), "a.txt")
	writeFile(t, path, "a")
	if err := c.New(ctx, []string{path}, NewOptions{Copy: true}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") || !strings.Contains(errOut.String(), "\x1b]52;c;") {
		t.Errorf("stdout %q, stderr %q", out, errOut)
	}
}
