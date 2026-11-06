// Package recording runs the TUI against a made-up GitHub, for the recording in
// the README. It is built only into demo/recording, never into gist.
package recording

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

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

// runner is what the made-up GitHub needs of who runs it.
type runner struct{ cleanups []func() }

func (*runner) Helper() {}

func (*runner) Errorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func (r *runner) Cleanup(f func()) { r.cleanups = append(r.cleanups, f) }

func (r *runner) close() {
	for _, f := range slices.Backward(r.cleanups) {
		f()
	}
}

// releases are the made-up releases. What is found of them is not kept.
type releases struct{ gh *github.Client }

func (r releases) Releases(ctx context.Context) ([]model.Release, error) { return r.gh.Releases(ctx) }
func (releases) Checked(string) error                                    { return nil }
func (releases) Told(string) error                                       { return nil }

// Version is the version the recording shows.
const Version = "2.0.0"

// Run runs the TUI until it quits. Nothing is read from or written to the
// user's files.
func Run() error {
	dir, err := os.MkdirTemp("", "gist-demo")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	r := &runner{}
	defer r.close()
	srv := fakegist.New(r)
	Fill(srv, time.Now())

	log := logging.Discard()
	client, err := github.New(log, github.Options{Token: "demo", Transport: srv.Transport()})
	if err != nil {
		return err
	}
	cfg := config.Default()
	cfg.Theme = config.ThemeDefault
	store := state.Open(filepath.Join(dir, "state"))
	_, err = tui.Run(tui.Options{
		Source: client, Via: "gh", Host: github.Host, CacheDir: filepath.Join(dir, "cache"),
		Layout: store, Editor: &edit.Editor{API: client, State: store}, Config: cfg, Keys: keymap.Default(),
		Log: log, Browse: func(string) error { return nil },
		Release: &tui.Release{Version: Version, Running: Version, How: "brew upgrade babarot/tap/gist", Source: releases{client}},
	})
	return err
}
