package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Core Core
	Gist Gist
	Flag Flag
}

type Core struct {
	Editor        string `toml:"editor"`
	SelectCmd     string `toml:"selectcmd"`
	TomlFile      string `toml:"tomlfile"`
	User          string `toml:"user"`
	ShowIndicator bool   `toml:"show_indicator"`
	BaseURL       string `toml:"base_url"`
}

type Gist struct {
	Token string `toml:"token"`
	Dir   string `toml:"dir"`
}

type Flag struct {
	OpenURL          bool   `toml:"open_url"`
	Private          bool   `toml:"private"`
	Verbose          bool   `toml:"verbose"`
	ShowSpinner      bool   `toml:"show_spinner"`
	Sort             string `toml:"sort"`
	Only             string `toml:"only"`
	NoSelect         bool   `toml:"no_select"`
	EditDesc         bool   `toml:"edit_desc"`
	OpenStarredItems bool
}

var Conf Config

func expandPath(s string) string {
	if len(s) >= 2 && s[0] == '~' && os.IsPathSeparator(s[1]) {
		if runtime.GOOS == "windows" {
			s = filepath.Join(os.Getenv("USERPROFILE"), s[2:])
		} else {
			s = filepath.Join(os.Getenv("HOME"), s[2:])
		}
	}
	return os.Expand(s, os.Getenv)
}

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

	cfg.Gist.Token = os.Getenv("GITHUB_TOKEN")
	cfg.Core.Editor = os.Getenv("EDITOR")
	if cfg.Core.Editor == "" {
		cfg.Core.Editor = "vim"
	}
	cfg.Core.SelectCmd = "fzf-tmux --multi:fzf --multi:peco"
	cfg.Core.TomlFile = file
	cfg.Core.User = os.Getenv("USER")
	cfg.Core.ShowIndicator = true
	cfg.Core.BaseURL = "https://gist.github.com"
	dir := filepath.Join(filepath.Dir(file), "files")
	os.MkdirAll(dir, 0700)
	cfg.Gist.Dir = dir
	cfg.Flag.ShowSpinner = true
	cfg.Flag.Verbose = true

	return toml.NewEncoder(f).Encode(cfg)
}
