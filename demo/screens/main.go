// screens walks gist against a made-up GitHub of its own, pressing its keys,
// and writes every screen it comes to, with what each key leads to. The page
// draws them, so that it shows gist itself and not a copy of it.
//
// Usage: go run ./demo/screens docs/screens.json
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/babarot/gist/v2/internal/config"
	"github.com/babarot/gist/v2/internal/edit"
	"github.com/babarot/gist/v2/internal/fakegist"
	"github.com/babarot/gist/v2/internal/github"
	"github.com/babarot/gist/v2/internal/logging"
	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/state"
	"github.com/babarot/gist/v2/internal/tui"
	"github.com/babarot/gist/v2/internal/tui/keymap"
)

const (
	width, height = 120, 34
	// version is the version the screens show.
	version = "2.0.0"
	// maxScreens stops a walk that has found something without an end.
	maxScreens = 2000
)

// keys are those the page answers. esc is left out: on the list it quits.
var keys = []string{"j", "k", "h", "l", "1", "2", "?", "J", "K"}

type runner struct{ cleanups []func() }

func (*runner) Helper()                   {}
func (*runner) Errorf(f string, a ...any) { log.Printf(f, a...) }
func (r *runner) Cleanup(f func())        { r.cleanups = append(r.cleanups, f) }

type releases struct{ gh *github.Client }

func (r releases) Releases(ctx context.Context) ([]model.Release, error) { return r.gh.Releases(ctx) }
func (releases) Checked(string) error                                    { return nil }
func (releases) Told(string) error                                       { return nil }

// grab asks for the screen from inside the program, where it is safe to read.
type grab struct{ screen chan string }

type program struct{ *tui.Model }

func (p program) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if g, ok := msg.(grab); ok {
		g.screen <- p.Model.View().Content
		return p, nil
	}
	_, cmd := p.Model.Update(msg)
	return p, cmd
}

func press(k string) tea.Msg {
	return tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
}

type walk struct {
	p       *tea.Program
	screens []string
	seen    map[[32]byte]int
	next    []map[string]int
}

func (w *walk) shot() string {
	c := make(chan string, 1)
	w.p.Send(grab{c})
	select {
	case s := <-c:
		return s
	case <-time.After(5 * time.Second):
		log.Fatalf("gist stopped answering after %d screens", len(w.screens))
		return ""
	}
}

// settled waits for the screen to stop changing, as files are read.
func (w *walk) settled() string {
	last, same := w.shot(), 0
	for range 80 {
		time.Sleep(5 * time.Millisecond)
		s := w.shot()
		if s != last {
			last, same = s, 0
			continue
		}
		if same++; same == 3 {
			break
		}
	}
	return last
}

func (w *walk) add(s string) int {
	id := sha256.Sum256([]byte(s))
	if i, ok := w.seen[id]; ok {
		return i
	}
	i := len(w.screens)
	w.seen[id] = i
	w.screens = append(w.screens, s)
	w.next = append(w.next, map[string]int{})
	return i
}

func (w *walk) untried(i int) string {
	for _, k := range keys {
		if _, ok := w.next[i][k]; !ok {
			return k
		}
	}
	return ""
}

// route finds the keys to the nearest screen with a key not yet tried.
func (w *walk) route(from int) (int, []string, bool) {
	type step struct {
		at   int
		keys []string
	}
	seen := map[int]bool{from: true}
	for q := []step{{from, nil}}; len(q) > 0; q = q[1:] {
		s := q[0]
		if w.untried(s.at) != "" {
			return s.at, s.keys, true
		}
		for _, k := range keys {
			if n := w.next[s.at][k]; !seen[n] {
				seen[n] = true
				q = append(q, step{n, append(slices.Clip(s.keys), k)})
			}
		}
	}
	return 0, nil, false
}

func run(out string) error {
	dir, err := os.MkdirTemp("", "gist-screens")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	r := &runner{}
	srv := fakegist.New(r)
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	fill(srv, now)
	discard := logging.Discard()
	client, err := github.New(discard, github.Options{Token: "demo", Transport: srv.Transport()})
	if err != nil {
		return err
	}
	cfg := config.Default()
	cfg.Theme = config.ThemeDefault
	store := state.Open(filepath.Join(dir, "state"))
	m := tui.New(tui.Options{
		Source: client, Via: "gh", Host: github.Host, CacheDir: filepath.Join(dir, "cache"),
		Layout: store, Editor: &edit.Editor{API: client, State: store}, Config: cfg, Keys: keymap.Default(),
		Log: discard, Browse: func(string) error { return nil }, Now: func() time.Time { return now },
		Release: &tui.Release{Version: version, Running: version, How: "brew upgrade babarot/tap/gist", Source: releases{client}},
	})
	p := tea.NewProgram(program{m}, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithWindowSize(width, height))
	go func() { _, _ = p.Run() }()
	defer p.Kill()
	time.Sleep(300 * time.Millisecond)

	w := &walk{p: p, seen: map[[32]byte]int{}}
	// To the starred gists and back, for the walk to start with both lists read
	// and past the message of what was saved, which never comes back.
	w.settled()
	p.Send(press("2"))
	w.settled()
	p.Send(press("1"))
	cur, tried, missed := w.add(w.settled()), 0, map[int]int{}
	for len(w.screens) < maxScreens {
		at, route, ok := w.route(cur)
		if !ok {
			break
		}
		for _, k := range route {
			p.Send(press(k))
		}
		if len(route) > 0 {
			if cur = w.add(w.settled()); cur != at {
				// A screen can look the same with another state behind it, such
				// as how far the preview scrolled, and then the keys to it do
				// not lead back. Give up on what it had left to try.
				if missed[at]++; missed[at] == 3 {
					for _, k := range keys {
						if _, ok := w.next[at][k]; !ok {
							w.next[at][k] = at
						}
					}
				}
				continue
			}
		}
		k := w.untried(cur)
		p.Send(press(k))
		n := w.add(w.settled())
		w.next[cur][k] = n
		cur = n
		if tried++; tried%200 == 0 {
			log.Printf("%d keys tried, %d screens", tried, len(w.screens))
		}
	}
	help := make([]bool, len(w.screens))
	for i, s := range w.screens {
		text := ansi.Strip(s)
		help[i] = strings.Contains(text, "keys") && strings.Contains(text, "quit")
	}
	data, err := json.Marshal(map[string]any{"width": width, "height": height, "screens": w.screens, "next": w.next, "help": help})
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("%d screens, %d bytes\n", len(w.screens), len(data))
	return nil
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: screens FILE")
	}
	if err := run(os.Args[1]); err != nil {
		log.Fatal(err)
	}
}
