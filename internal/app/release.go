package app

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/babarot/gist/v2/internal/github"
	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/release"
	"github.com/babarot/gist/v2/internal/state"
	"github.com/babarot/gist/v2/internal/tui"
	"github.com/babarot/gist/v2/internal/version"
)

// EnvNoUpdateCheck, set to anything, keeps gist from looking for new releases.
const EnvNoUpdateCheck = "GIST_NO_UPDATE_CHECK"

// releaseOptions are the releases as the TUI starts with them. Their notes can
// always be opened, but only a release build looks for a newer one by itself:
// whoever built the rest updates it.
func releaseOptions(src *releaseSource, check bool, now time.Time) *tui.Release {
	r := &tui.Release{
		Version: version.String(), Running: version.Version, How: release.Page,
		FromSource: !version.IsRelease(), Source: src,
	}
	if r.FromSource {
		return r
	}
	r.How = updateHow()
	if check {
		seen := src.store.ReleaseSeen()
		r.Tell, r.Latest, r.Told, r.Due = true, seen.Latest, seen.Told, release.Due(seen.Checked, now)
	}
	return r
}

// updateHow says how the running executable is updated.
func updateHow() string {
	exe, err := os.Executable()
	if err != nil {
		return release.Page
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return release.How(exe)
}

// releaseSource reads the releases from GitHub, and keeps what was found.
type releaseSource struct {
	gh    *github.Client
	store *state.Store
}

func (s *releaseSource) Releases(ctx context.Context) ([]model.Release, error) {
	return s.gh.Releases(ctx)
}

func (s *releaseSource) Checked(latest string) error {
	seen := s.store.ReleaseSeen()
	seen.Checked = time.Now()
	if latest != "" {
		seen.Latest = latest
	}
	return s.store.SaveReleaseSeen(seen)
}

func (s *releaseSource) Told(v string) error {
	seen := s.store.ReleaseSeen()
	seen.Told = v
	return s.store.SaveReleaseSeen(seen)
}
