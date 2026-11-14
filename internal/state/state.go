// Package state keeps what would be missed if lost: the drafts of edits not
// yet sent, and the locks that keep two gist from one gist's drafts.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// ErrLocked says another gist process is editing the gist.
var ErrLocked = errors.New("being edited in another gist process")

// Store is the state directory.
type Store struct {
	dir string
}

// Open is the store in dir.
func Open(dir string) *Store {
	s := &Store{dir: dir}
	s.renameDrafts()
	return s
}

// settings are what is remembered between launches.
type settings struct {
	PreviewHeight int         `json:"preview_height,omitempty"`
	PaneWidth     int         `json:"pane_width,omitempty"`
	Release       ReleaseSeen `json:"release,omitzero"`
}

// ReleaseSeen is what was last found of gist's releases.
type ReleaseSeen struct {
	// Checked is when they were last read.
	Checked time.Time `json:"checked,omitzero"`
	// Latest is the newest found, and Told the newest told of.
	Latest string `json:"latest,omitempty"`
	Told   string `json:"told,omitempty"`
}

func (s *Store) settingsPath() string { return filepath.Join(s.dir, "state.json") }

func (s *Store) settings() settings {
	var st settings
	if b, err := os.ReadFile(s.settingsPath()); err == nil {
		// A file that cannot be read is what was never written: all of it is
		// a convenience
		_ = json.Unmarshal(b, &st)
	}
	return st
}

// PreviewHeight is the height the preview was left at, or 0.
func (s *Store) PreviewHeight() int { return s.settings().PreviewHeight }

// SavePreviewHeight remembers the height of the preview.
func (s *Store) SavePreviewHeight(h int) error {
	st := s.settings()
	st.PreviewHeight = h
	return s.saveSettings(st)
}

// PaneWidth is the width the pane beside the preview was dragged to, or 0.
func (s *Store) PaneWidth() int { return s.settings().PaneWidth }

// SavePaneWidth remembers the width of the pane; 0 forgets it.
func (s *Store) SavePaneWidth(w int) error {
	st := s.settings()
	st.PaneWidth = w
	return s.saveSettings(st)
}

// ReleaseSeen is what was last found of the releases.
func (s *Store) ReleaseSeen() ReleaseSeen { return s.settings().Release }

// SaveReleaseSeen remembers it.
func (s *Store) SaveReleaseSeen(r ReleaseSeen) error {
	st := s.settings()
	st.Release = r
	return s.saveSettings(st)
}

func (s *Store) saveSettings(st settings) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return writeFile(s.settingsPath(), b, false)
}

// Lock is the hold of one process on a gist. Drafts are written and removed
// only through it, so another process cannot replace a draft or remove it.
type Lock struct {
	store *Store
	gist  string
	file  *os.File
}

// Lock takes the gist's lock, failing with ErrLocked when another process has
// it. The operating system lets it go if the process dies.
func (s *Store) Lock(gist string) (*Lock, error) {
	dir := filepath.Join(s.dir, "locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, gist+".lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	ok, err := tryLock(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	if !ok {
		f.Close()
		return nil, ErrLocked
	}
	return &Lock{store: s, gist: gist, file: f}, nil
}

// Release lets the lock go.
func (l *Lock) Release() error { return l.file.Close() }

// Draft is an edit not yet sent. Its text is kept as bytes, as a file need not
// be valid UTF-8 and JSON strings would change what is not.
type Draft struct {
	Gist string `json:"gist"`
	File string `json:"file"`
	// Revision and Original are what the edit started from, and what it is
	// compared with.
	Revision string `json:"revision"`
	Original []byte `json:"original"`
	// Content is the file as edited.
	Content []byte    `json:"content"`
	Created time.Time `json:"created"`
}

func (s *Store) draftDir(gist string) string { return filepath.Join(s.DraftsDir(), gist) }

// escapedPart is what url.PathEscape leaves, an escape or a capital.
var escapedPart = regexp.MustCompile(`%[0-9A-F]{2}|[A-Z]`)

// draftName is the file of a draft. Capitals are escaped too: A.go and a.go are
// two files of a gist, but one file on the disks of macOS and Windows.
func draftName(file string) string {
	return escapedPart.ReplaceAllStringFunc(url.PathEscape(file), func(m string) string {
		if len(m) > 1 {
			return m
		}
		return fmt.Sprintf("%%%02X", m[0])
	}) + ".json"
}

func (s *Store) draftPath(gist, file string) string {
	return filepath.Join(s.draftDir(gist), draftName(file))
}

// renameDrafts moves the drafts kept before capitals were escaped to the names
// they are looked for by. One that cannot be read or moved is left where it is.
func (s *Store) renameDrafts() {
	gists, err := s.DraftGists()
	if err != nil {
		return
	}
	for _, gist := range gists {
		entries, err := os.ReadDir(s.draftDir(gist))
		if err != nil {
			continue
		}
		for _, e := range entries {
			path := filepath.Join(s.draftDir(gist), e.Name())
			d, ok, err := readDraft(path)
			if err != nil || !ok || d.Gist != gist {
				continue
			}
			want := s.draftPath(gist, d.File)
			if want == path {
				continue
			}
			if _, err := os.Stat(want); errors.Is(err, fs.ErrNotExist) {
				_ = os.Rename(path, want)
			}
		}
	}
}

// SaveDraft keeps d, for the gist of the lock. It is written whole or not at
// all, so a crash leaves no broken draft.
func (l *Lock) SaveDraft(d Draft) error {
	if d.Gist != l.gist {
		return fmt.Errorf("draft of %s under the lock of %s", d.Gist, l.gist)
	}
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	// The draft holds the only copy of the edit: it must be on the disk
	// before the editor's file is removed
	return writeFile(l.store.draftPath(d.Gist, d.File), b, true)
}

// writeFile replaces path whole, so a crash leaves no broken file.
func writeFile(path string, b []byte, sync bool) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if sync {
		if err := tmp.Sync(); err != nil {
			tmp.Close()
			return err
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// DeleteDraft removes the draft of a file of the lock's gist, which need not
// exist.
func (l *Lock) DeleteDraft(file string) error {
	err := os.Remove(l.store.draftPath(l.gist, file))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	// The gist's directory goes with its last draft
	_ = os.Remove(l.store.draftDir(l.gist))
	return nil
}

// Draft reads the draft of a file.
func (s *Store) Draft(gist, file string) (Draft, bool, error) {
	return readDraft(s.draftPath(gist, file))
}

func readDraft(path string) (Draft, bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Draft{}, false, nil
	}
	if err != nil {
		return Draft{}, false, err
	}
	var d Draft
	if err := json.Unmarshal(b, &d); err != nil {
		return Draft{}, false, fmt.Errorf("%s: %w", path, err)
	}
	return d, true, nil
}

// DeleteDrafts removes all the drafts of the lock's gist.
func (l *Lock) DeleteDrafts() error { return os.RemoveAll(l.store.draftDir(l.gist)) }

// HasDraft says a file has a draft, without reading it.
func (s *Store) HasDraft(gist, file string) bool {
	_, err := os.Stat(s.draftPath(gist, file))
	return err == nil
}

// DraftsDir is where drafts are kept.
func (s *Store) DraftsDir() string { return filepath.Join(s.dir, "drafts") }

// DraftGists are the gists that have drafts.
func (s *Store) DraftGists() ([]string, error) {
	entries, err := os.ReadDir(s.DraftsDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var gists []string
	for _, e := range entries {
		if e.IsDir() {
			gists = append(gists, e.Name())
		}
	}
	return gists, nil
}

// Drafts reads the drafts of a gist.
func (s *Store) Drafts(gist string) ([]Draft, error) {
	entries, err := os.ReadDir(s.draftDir(gist))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var drafts []Draft
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		d, ok, err := readDraft(filepath.Join(s.draftDir(gist), e.Name()))
		if err != nil {
			return nil, err
		}
		if ok {
			drafts = append(drafts, d)
		}
	}
	return drafts, nil
}
