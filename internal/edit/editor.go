package edit

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Command is the program to edit paths with, and its arguments: the editor
// set in the config, or $VISUAL, $EDITOR, then vi. It is split on spaces, so
// `code -w` works.
func Command(configured string, paths ...string) []string {
	args := []string{"vi"}
	for _, e := range []string{configured, os.Getenv("VISUAL"), os.Getenv("EDITOR")} {
		if f := strings.Fields(e); len(f) > 0 {
			args = f
			break
		}
	}
	return append(args, paths...)
}

// Workspace is the files of a session on disk, for an editor: each under its
// real name, so the editor highlights it.
type Workspace struct {
	dir   string
	paths map[string]string
	names []string
}

// Workspace writes the content of the files to a directory of their own, which
// only the user can read.
func (s *Session) Workspace() (*Workspace, error) {
	dir, err := os.MkdirTemp("", "gist-"+s.gist[:min(7, len(s.gist))]+"-*")
	if err != nil {
		return nil, err
	}
	w := &Workspace{dir: dir, paths: map[string]string{}}
	for _, f := range s.Files {
		path := filepath.Join(dir, filepath.Base(f.Name))
		if err := os.WriteFile(path, []byte(f.Content), 0o600); err != nil {
			_ = os.RemoveAll(dir)
			return nil, err
		}
		w.paths[f.Name] = path
		w.names = append(w.names, f.Name)
	}
	return w, nil
}

// Paths are the files, in the order of the session.
func (w *Workspace) Paths() []string {
	out := make([]string, len(w.names))
	for i, n := range w.names {
		out[i] = w.paths[n]
	}
	return out
}

// KeptAtError says an edit could not be kept as a draft and its files are left
// on disk, so it is not lost.
type KeptAtError struct {
	Dir string
	Err error
}

func (e *KeptAtError) Error() string {
	return fmt.Sprintf("the edit could not be saved as a draft (%v); it is still in %s", e.Err, e.Dir)
}

func (e *KeptAtError) Unwrap() error { return e.Err }

// ReadBack reads the files the editor left, whatever way it ended, and keeps
// each as a draft. Only then are they removed: until the edit is in a draft
// these files are its only copy. It returns whether anything differs from the
// base. An editor that removed a file leaves the edit of that file as it was.
func (w *Workspace) ReadBack(s *Session) (bool, error) {
	for _, name := range w.names {
		b, err := os.ReadFile(w.paths[name])
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, &KeptAtError{w.dir, err}
		}
		if _, err := s.Keep(name, string(b)); err != nil {
			return false, &KeptAtError{w.dir, err}
		}
	}
	if err := os.RemoveAll(w.dir); err != nil {
		return false, err
	}
	return len(s.Changed()) > 0, nil
}

// Scratch is a file for an editor to write what has no gist yet.
type Scratch struct {
	dir, path string
}

// NewScratch writes content to a file of this name, in a directory of its own
// that only the user can read.
func NewScratch(name, content string) (*Scratch, error) {
	dir, err := os.MkdirTemp("", "gist-new-*")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, filepath.Base(name))
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	return &Scratch{dir, path}, nil
}

// Path is the file.
func (s *Scratch) Path() string { return s.path }

// Read reads what the editor left and removes it.
func (s *Scratch) Read() (string, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return "", err
	}
	return string(b), os.RemoveAll(s.dir)
}
