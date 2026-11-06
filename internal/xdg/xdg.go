// Package xdg says where gist keeps its files, following the XDG base
// directory specification. Each directory is gist's own, under the base
// directory the environment names.
package xdg

import (
	"os"
	"path/filepath"
)

// ConfigDir holds what the user writes: $XDG_CONFIG_HOME/gist, or
// ~/.config/gist.
func ConfigDir() string { return dir("XDG_CONFIG_HOME", ".config") }

// StateDir holds what would be missed if lost, such as drafts:
// $XDG_STATE_HOME/gist, or ~/.local/state/gist.
func StateDir() string { return dir("XDG_STATE_HOME", filepath.Join(".local", "state")) }

// CacheDir holds what can be read again from GitHub: $XDG_CACHE_HOME/gist,
// or ~/.cache/gist.
func CacheDir() string { return dir("XDG_CACHE_HOME", ".cache") }

// dir is gist's directory under the base directory that env names, or under
// fallback in the home directory. The specification has a relative path in
// env ignored.
func dir(env, fallback string) string {
	if base := os.Getenv(env); filepath.IsAbs(base) {
		return filepath.Join(base, "gist")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, fallback, "gist")
}
