// Package config reads the config file: what the user can set, each with a
// default, so a file that sets nothing is a file that works.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/babarot/gist/v2/internal/near"
)

// The values of the settings that take one of a few.
const (
	ThemeAuto    = "auto"
	ThemeDefault = "default"
	ThemeLatte   = "latte"

	VisibilitySecret = "secret"
	VisibilityPublic = "public"

	LeadDescription = "description"
	LeadFile        = "file"

	MarkdownRendered = "rendered"
	MarkdownPlain    = "plain"
)

// File is the config file.
type File struct {
	Editor  string              `toml:"editor"`
	Theme   string              `toml:"theme"`
	New     New                 `toml:"new"`
	List    List                `toml:"list"`
	Preview Preview             `toml:"preview"`
	Keys    map[string][]string `toml:"keys"`
}

// New is what a new gist starts with.
type New struct {
	Visibility string `toml:"visibility"`
}

// List is how the list looks.
type List struct {
	Lead string `toml:"lead"`
}

// Preview is how the preview looks.
type Preview struct {
	Markdown string `toml:"markdown"`
}

// Default is the settings of a file that sets nothing.
func Default() File {
	return File{
		Theme:   ThemeAuto,
		New:     New{Visibility: VisibilitySecret},
		List:    List{Lead: LeadDescription},
		Preview: Preview{Markdown: MarkdownRendered},
	}
}

// Load reads the file at path. A missing file is the defaults.
func Load(path string) (File, error) {
	cfg := Default()
	src, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	dec := toml.NewDecoder(bytes.NewReader(src)).DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		// The strict error alone does not say which key it is about
		if strict, ok := errors.AsType[*toml.StrictMissingError](err); ok {
			return cfg, fmt.Errorf("%s: unknown setting\n%s", path, strict.String())
		}
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	for _, c := range []struct {
		key, got string
		allowed  []string
	}{
		{"theme", cfg.Theme, []string{ThemeAuto, ThemeDefault, ThemeLatte}},
		{"new.visibility", cfg.New.Visibility, []string{VisibilitySecret, VisibilityPublic}},
		{"list.lead", cfg.List.Lead, []string{LeadDescription, LeadFile}},
		{"preview.markdown", cfg.Preview.Markdown, []string{MarkdownRendered, MarkdownPlain}},
	} {
		if !slices.Contains(c.allowed, c.got) {
			return cfg, fmt.Errorf("%s: %s", path, notOneOf(c.key, c.got, c.allowed))
		}
	}
	return cfg, nil
}

func notOneOf(key, got string, allowed []string) string {
	msg := fmt.Sprintf("%s must be one of %s, got %q", key, strings.Join(allowed, ", "), got)
	if n, ok := near.Closest(got, allowed); ok {
		msg += fmt.Sprintf("; did you mean %q?", n)
	}
	return msg
}

// Template is the file written on first run: every setting at its default,
// commented out.
const Template = `# gist settings. Uncomment a line to change it.

# The editor; empty uses $VISUAL, then $EDITOR, then vi.
# editor = ""

# "auto" follows the terminal's background; "default" is dark, "latte" light.
# theme = "auto"

[new]
# What a new gist is: "secret" or "public".
# visibility = "secret"

[list]
# The first column: "description", or "file" for FILE and DESCRIPTION.
# lead = "description"

[preview]
# How .md files show: "rendered", or "plain" as written.
# markdown = "rendered"

[keys]
# The keys of an action, as the ? list names them.
# quit = ["q"]
`

// WriteTemplate writes Template to path unless a file is already there.
func WriteTemplate(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := f.WriteString(Template); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
