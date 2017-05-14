package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/b4b4r07/gist/cli"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "sync",
	Long:  "sync",
	RunE:  sync,
}

func sync(cmd *cobra.Command, args []string) error {
	gist, err := cli.NewGist()
	if err != nil {
		return err
	}
	err = gist.List()
	if err != nil {
		return err
	}
	root := cli.Conf.Gist.Dir
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			// skip recursive
			if strings.HasPrefix(filepath.Base(path), ".") {
				return filepath.SkipDir
			}
			// skip
			return nil
		}
		_ = cli.Sync(gist, path)
		return nil
	})
}

func init() {
	RootCmd.AddCommand(syncCmd)
}
