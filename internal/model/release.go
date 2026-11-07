package model

import "time"

// Release is a release of gist on GitHub.
type Release struct {
	// Version is the tag without its leading v, such as "2.0.0".
	Version   string
	Published time.Time
	// Body is the release notes, Markdown as GitHub generated them.
	Body string
}
