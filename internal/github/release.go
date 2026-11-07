package github

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/babarot/gist/v2/internal/model"
)

// releasesPath reads gist's own releases, as many as one page holds.
const releasesPath = "repos/babarot/gist/releases?per_page=30"

type releaseResponse struct {
	TagName     string    `json:"tag_name"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	Body        string    `json:"body"`
}

// Releases reads gist's releases, newest first, leaving out drafts and
// prereleases.
func (c *Client) Releases(ctx context.Context) ([]model.Release, error) {
	var resp []releaseResponse
	if err := c.rest.DoWithContext(ctx, http.MethodGet, releasesPath, nil, &resp); err != nil {
		return nil, err
	}
	out := []model.Release{}
	for _, r := range resp {
		if r.Draft || r.Prerelease {
			continue
		}
		out = append(out, model.Release{Version: strings.TrimPrefix(r.TagName, "v"), Published: r.PublishedAt, Body: r.Body})
	}
	return out, nil
}
