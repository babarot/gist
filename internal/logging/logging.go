// Package logging makes the logger gist writes its log file with: JSON lines
// through log/slog, to a file under the state directory, never to the
// terminal the TUI draws on.
//
// The logger is made once, in the app package, and passed down. A log line
// never holds a token, or the content, description or comments of a gist: a
// gist is named by its ID and a file by its name.
package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/nxadm/tail"

	"github.com/babarot/gist/v2/internal/xdg"
)

// Attribute keys every package uses, so a log line can be searched by them.
const (
	KeyRun           = "run"
	KeyVersion       = "version"
	KeyDurationMS    = "duration_ms"
	KeyGHRequestID   = "gh_request_id"
	KeyRateRemaining = "rate_remaining"
)

// Rotation defaults: the file is moved aside when it is over MaxSize at
// startup, and Keep old files are kept.
const (
	DefaultMaxSize = 10 << 20
	DefaultKeep    = 3
)

// DefaultPath is where the log file goes: log/gist.log in the state
// directory.
func DefaultPath() string {
	return filepath.Join(xdg.StateDir(), "log", "gist.log")
}

// ParseLevel reads a level as written in the environment: debug, info, warn
// or error.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return slog.LevelInfo, fmt.Errorf("%q is not a log level: debug, info, warn or error", s)
}

// Options say where and what to log.
type Options struct {
	// Path is the log file; DefaultPath when empty.
	Path string
	// Level is the lowest level written.
	Level slog.Leveler
	// MaxSize and Keep control rotation at startup; the defaults when zero.
	MaxSize int64
	Keep    int
}

// Open rotates the log file if it has grown past MaxSize, opens it for
// appending, and returns a logger writing JSON lines to it. Closing the
// returned closer closes the file.
func Open(o Options) (*slog.Logger, io.Closer, error) {
	if o.Path == "" {
		o.Path = DefaultPath()
	}
	if o.MaxSize <= 0 {
		o.MaxSize = DefaultMaxSize
	}
	if o.Keep <= 0 {
		o.Keep = DefaultKeep
	}
	if err := os.MkdirAll(filepath.Dir(o.Path), 0o700); err != nil {
		return nil, nil, err
	}
	if err := rotate(o.Path, o.MaxSize, o.Keep); err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(o.Path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, err
	}
	return New(f, o.Level), f, nil
}

// New returns a logger writing JSON lines to w at level and above. Tests
// pass a buffer.
func New(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

// Discard is a logger that writes nothing, for code run without one.
func Discard() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// rotate moves path to path.1, path.1 to path.2 and so on, dropping the
// oldest beyond keep, when path is larger than max.
func rotate(path string, max int64, keep int) error {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() <= max {
		return nil
	}
	old := func(n int) string { return fmt.Sprintf("%s.%d", path, n) }
	if err := os.Remove(old(keep)); err != nil && !os.IsNotExist(err) {
		return err
	}
	for n := keep - 1; n >= 1; n-- {
		if err := os.Rename(old(n), old(n+1)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Rename(path, old(1))
}

// NewID returns a short random ID, for a run.
func NewID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(b[:])
}

// Follow writes each line added to the log file at path to w, until ctx is
// done. It follows the file through a rotation, and waits for a file that
// does not exist yet.
func Follow(ctx context.Context, w io.Writer, path string) error {
	t, err := tail.TailFile(path, tail.Config{
		Follow:   true,
		ReOpen:   true,
		Poll:     true,
		Logger:   tail.DiscardingLogger,
		Location: &tail.SeekInfo{Whence: io.SeekEnd},
	})
	if err != nil {
		return err
	}
	defer t.Cleanup()
	defer func() { _ = t.Stop() }()
	for {
		select {
		case <-ctx.Done():
			return nil
		case line, ok := <-t.Lines:
			if !ok {
				return t.Err()
			}
			if line.Err != nil {
				return line.Err
			}
			if _, err := fmt.Fprintln(w, line.Text); err != nil {
				return err
			}
		}
	}
}
