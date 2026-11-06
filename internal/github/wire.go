package github

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/babarot/gist/v2/internal/model"
)

// The shapes GitHub's JSON comes and goes in.

type user struct {
	Login string `json:"login"`
}

type gist struct {
	ID          string          `json:"id"`
	Description *string         `json:"description"`
	Owner       *user           `json:"owner"`
	Public      bool            `json:"public"`
	HTMLURL     string          `json:"html_url"`
	Comments    int             `json:"comments"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Files       map[string]file `json:"files"`
	History     []revision      `json:"history"`
}

type file struct {
	Filename  string `json:"filename"`
	Type      string `json:"type"`
	Language  string `json:"language"`
	Size      int64  `json:"size"`
	RawURL    string `json:"raw_url"`
	Content   string `json:"content"`
	Encoding  string `json:"encoding"`
	Truncated bool   `json:"truncated"`
}

type revision struct {
	Version string `json:"version"`
}

type comment struct {
	ID        int64     `json:"id"`
	User      *user     `json:"user"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// fileChange is what is sent for a file. A nil one is null, which removes it.
type fileChange struct {
	Content  *string `json:"content,omitempty"`
	Filename *string `json:"filename,omitempty"`
}

func (g gist) model() model.Gist {
	m := model.Gist{
		ID:       g.ID,
		Public:   g.Public,
		URL:      g.HTMLURL,
		Comments: g.Comments,
		Created:  g.CreatedAt,
		Updated:  g.UpdatedAt,
	}
	if g.Description != nil {
		m.Description = *g.Description
	}
	if g.Owner != nil {
		m.Owner = g.Owner.Login
	}
	for name, f := range g.Files {
		m.Files = append(m.Files, model.File{
			Name:      name,
			Type:      f.Type,
			Language:  f.Language,
			Size:      f.Size,
			RawURL:    f.RawURL,
			Content:   f.Content,
			Truncated: f.Truncated,
			Binary:    f.Encoding == "base64" || strings.ContainsRune(f.Content, 0),
		})
	}
	slices.SortFunc(m.Files, func(a, b model.File) int { return cmp.Compare(a.Name, b.Name) })
	for _, r := range g.History {
		m.History = append(m.History, r.Version)
	}
	return m
}

func (c comment) model() model.Comment {
	m := model.Comment{ID: c.ID, Body: c.Body, Created: c.CreatedAt, Updated: c.UpdatedAt}
	if c.User != nil {
		m.Author = c.User.Login
	}
	return m
}
