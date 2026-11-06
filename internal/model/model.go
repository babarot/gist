// Package model holds the types the other packages share: gists, their
// files and comments, as gist uses them rather than as GitHub sends them.
package model

import "time"

// Gist is a gist. A list from GitHub leaves out what the fields say they
// need Get for.
type Gist struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Owner       string    `json:"owner"`
	Public      bool      `json:"public"`
	URL         string    `json:"url"`
	Comments    int       `json:"comments"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
	// Files are sorted by name, the order GitHub shows them in.
	Files []File `json:"files"`
	// History is the revisions, newest first. Only a single gist has it, not
	// a list. Changing just the description makes no revision.
	History []string `json:"history,omitempty"`
}

// Revision is the gist's latest revision, or "" when History was not read.
func (g Gist) Revision() string {
	if len(g.History) == 0 {
		return ""
	}
	return g.History[0]
}

// File is a file of a gist.
type File struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Language string `json:"language,omitempty"`
	Size     int64  `json:"size"`
	// RawURL points at this file at the revision read.
	RawURL string `json:"raw_url"`
	// Content is for showing: only a single gist has it, and it is not the
	// file as written, since the API client rewrites control characters and
	// Truncated cuts it. Editing and printing use the raw content instead.
	Content   string `json:"content,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

// Comment is a comment on a gist.
type Comment struct {
	ID      int64     `json:"id"`
	Author  string    `json:"author"`
	Body    string    `json:"body"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// NewGist is a gist to create.
type NewGist struct {
	Description string
	Public      bool
	// Files are the names and contents.
	Files map[string]string
}

// FileChange is what to do to a file of a gist.
type FileChange struct {
	// Remove removes the file. The other fields are then not used.
	Remove bool
	// Content is the new content, nil to leave it.
	Content *string
	// Name is the new name, nil to leave it.
	Name *string
}

// Change is what to change in a gist.
type Change struct {
	// Description is nil to leave it.
	Description *string
	// Files are by the name the file has now. A name the gist does not have
	// adds a file, which takes Content.
	Files map[string]FileChange
}

// Empty says the change changes nothing.
func (c Change) Empty() bool { return c.Description == nil && len(c.Files) == 0 }

// ListPage is a page of the list of the user's gists as read.
type ListPage struct {
	Number int
	// Last is the number of the last page, when the page says, which only the
	// first does and not when it is NotModified.
	Last int
	// ETag asks for the page again only if it changed.
	ETag string
	// NotModified says the page is as it was for ETag, and Gists is empty.
	NotModified bool
	Gists       []Gist
}
