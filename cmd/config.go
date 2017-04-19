package cmd

import (
	"path/filepath"

	"github.com/b4b4r07/gist/config"
	"github.com/b4b4r07/gist/util"
	"github.com/spf13/cobra"
)

var confCmd = &cobra.Command{
	Use:   "config",
	Short: "Config the setting file",
	Long:  "Config the setting file with your editor (default: vim)",
	RunE:  conf,
}

func conf(cmd *cobra.Command, args []string) error {
	editor := config.Conf.Core.Editor
	tomlfile := config.Conf.Core.TomlFile
	if tomlfile == "" {
		dir, _ := config.GetDefaultDir()
		tomlfile = filepath.Join(dir, "config.toml")
	}
	return util.RunCommand(editor, tomlfile)
}

func init() {
	RootCmd.AddCommand(confCmd)
}
