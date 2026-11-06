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

func TestEditOnATerminalNeedsSomethingToDo(t *testing.T) {
	c, s, _, _ := terminal(t, "")
	g := s.Add("", false, map[string]string{"a.txt": "a"})
	if err := c.Edit(ctx, g.ID, "", EditOptions{}); !errors.Is(err, ErrNeedsEditor) {
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
