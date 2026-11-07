package xdg

import (
	"path/filepath"
	"testing"
)

func TestDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dirs := []struct {
		name     string
		env      string
		fallback string
		get      func() string
	}{
		{"config", "XDG_CONFIG_HOME", ".config/gist", ConfigDir},
		{"state", "XDG_STATE_HOME", ".local/state/gist", StateDir},
		{"cache", "XDG_CACHE_HOME", ".cache/gist", CacheDir},
	}
	for _, d := range dirs {
		t.Run(d.name, func(t *testing.T) {
			fallback := filepath.Join(home, d.fallback)
			for _, tt := range []struct{ name, value, want string }{
				{"unset", "", fallback},
				{"absolute", "/base", "/base/gist"},
				{"relative is ignored", "base", fallback},
			} {
				t.Run(tt.name, func(t *testing.T) {
					t.Setenv(d.env, tt.value)
					if got := d.get(); got != tt.want {
						t.Errorf("got %q, want %q", got, tt.want)
					}
				})
			}
		})
	}
}
