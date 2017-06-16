package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Core CoreConfig `toml:"core"`
	Gist GistConfig `toml:"gist"`
	Flag FlagConfig `toml:"flag"`
}

type CoreConfig struct {
	Editor    string `toml:"editor"`
	SelectCmd string `toml:"selectcmd"`
	TomlFile  string `toml:"tomlfile"`
	User      string `toml:"user"`
}

type GistConfig struct {
	Token    string        `toml:"token"`
	BaseURL  string        `toml:"base_url"`
	Dir      string        `toml:"dir"`
	FileExt  string        `toml:"file_ext"`
	UseCache bool          `toml:"use_cache"`
	CacheTTL time.Duration `toml:"cache_ttl"`
}

type FlagConfig struct {
	Verbose           bool `toml:"verbose"`
	OpenURL           bool `toml:"open_url"`
	NewPrivate        bool `toml:"new_private"`
	OpenBaseURL       bool `toml:"open_base_url"`
	ShowIndicator     bool `toml:"show_indicator"`
	ShowPrivateSymbol bool `toml:"show_private_symbol"`
	BlogMode          bool `toml:"blog_mode"`
	StarredItems      bool `toml:"starred"`

	EditDesc      bool
	FromClipboard bool
}

var Conf Config

func GetDefaultDir() (string, error) {
	var dir string

	switch runtime.GOOS {
	default:
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	case "windows":
		dir = os.Getenv("APPDATA")
		if dir == "" {
			dir = filepath.Join(os.Getenv("USERPROFILE"), "Application Data")
		}
	}
	dir = filepath.Join(dir, "gist")

	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return dir, fmt.Errorf("cannot create directory: %v", err)
	}

	return dir, nil
}

func (cfg *Config) LoadFile(file string) error {
	_, err := os.Stat(file)
	if err == nil {
		_, err := toml.DecodeFile(file, cfg)
		if err != nil {
			return err
		}
		return nil
	}

	if !os.IsNotExist(err) {
		return err
	}
	f, err := os.Create(file)
	if err != nil {
		return err
	}

	cfg.Core.Editor = os.Getenv("EDITOR")
	if cfg.Core.Editor == "" {
		cfg.Core.Editor = "vim"
	}
	cfg.Core.SelectCmd = "fzf-tmux --multi:fzf --multi:peco"
	cfg.Core.TomlFile = file
	cfg.Core.User = os.Getenv("USER")

	cfg.Gist.Token = os.Getenv("GITHUB_TOKEN")
	cfg.Gist.BaseURL = "https://gist.github.com"
	dir := filepath.Join(filepath.Dir(file), "files")
	os.MkdirAll(dir, 0700)
	cfg.Gist.Dir = dir
	cfg.Gist.FileExt = ".patch"
	cfg.Gist.UseCache = true
	cfg.Gist.CacheTTL = 60 * 24

	cfg.Flag.Verbose = true
	cfg.Flag.OpenURL = false
	cfg.Flag.NewPrivate = false
	cfg.Flag.OpenBaseURL = false
	cfg.Flag.ShowIndicator = true
	cfg.Flag.ShowPrivateSymbol = false
	cfg.Flag.StarredItems = false

	return toml.NewEncoder(f).Encode(cfg)
}
