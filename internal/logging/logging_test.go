package logging

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDefaultPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	if got, want := DefaultPath(), filepath.Join(dir, "gist", "log", "gist.log"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseLevel(t *testing.T) {
	for _, tt := range []struct {
		in      string
		want    slog.Level
		wantErr bool
	}{
		{"", slog.LevelInfo, false},
		{"debug", slog.LevelDebug, false},
		{" WARN ", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"loud", slog.LevelInfo, true},
	} {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseLevel(tt.in)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("got %v, %v", got, err)
			}
		})
	}
}

func TestOpenAppendsOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "gist.log")
	for _, msg := range []string{"one", "two"} {
		log, c, err := Open(Options{Path: path, Level: slog.LevelInfo})
		if err != nil {
			t.Fatal(err)
		}
		log.Info(msg)
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"msg":"one"`) || !strings.Contains(string(b), `"msg":"two"`) {
		t.Errorf("log is %q", b)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); runtime.GOOS != "windows" && got != 0o600 {
		t.Errorf("mode is %o, want 600", got)
	}
}

func TestOpenRotates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gist.log")
	for _, f := range []struct{ name, body string }{
		{path, "current, over the size"},
		{path + ".1", "older"},
		{path + ".2", "oldest"},
	} {
		if err := os.WriteFile(f.name, []byte(f.body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	_, c, err := Open(Options{Path: path, Level: slog.LevelInfo, MaxSize: 5, Keep: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		path + ".1": "current, over the size",
		path + ".2": "older",
		path:        "",
	} {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != want {
			t.Errorf("%s holds %q, want %q", name, b, want)
		}
	}
	if _, err := os.Stat(path + ".3"); !os.IsNotExist(err) {
		t.Errorf("the oldest file should be gone: %v", err)
	}
}

func TestNewID(t *testing.T) {
	if a, b := NewID(), NewID(); len(a) != 8 || a == b {
		t.Errorf("got %q and %q", a, b)
	}
}

// lockedBuffer is written by Follow while the test reads it.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func TestFollow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gist.log")
	if err := os.WriteFile(path, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out lockedBuffer
	done := make(chan error, 1)
	go func() { done <- Follow(ctx, &out, path) }()

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	// Follow starts at the end of the file at some moment after this, so
	// lines are added until one reaches it
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(out.String(), "after\n") {
		if time.Now().After(deadline) {
			t.Fatalf("no line came through: %q", out.String())
		}
		if _, err := f.WriteString("after\n"); err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Errorf("Follow returned %v", err)
	}
	if strings.Contains(out.String(), "before") {
		t.Errorf("what was already in the file came through: %q", out.String())
	}
}
