package cmd

import (
	"path"
	"path/filepath"

	"github.com/b4b4r07/gist/config"
	"github.com/b4b4r07/gist/gist"
	"github.com/b4b4r07/gist/util"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit the gist file and sync after",
	Long:  "Edit the gist file and sync after",
	RunE:  edit,
}

func edit(cmd *cobra.Command, args []string) error {
	var err error

	gist, err := gist.New(config.Conf.Gist.Token)
	if err != nil {
		return err
	}

	gfs, err := gist.GetRemoteFiles()
	if err != nil {
		return err
	}

	selectedLines, err := util.Filter(gfs.Text)
	if err != nil {
		return err
	}

	for _, line := range selectedLines {
		if line == "" {
			continue
		}
		parsedLine, err := util.ParseLine(line)
		if err != nil {
			continue
		}

		file := filepath.Join(config.Conf.Gist.Dir, gfs.ExtendID(parsedLine.ID), parsedLine.Filename)
		err = gist.Edit(file)
		if err != nil {
			return err
		}

		if config.Conf.Flag.OpenURL {
			url := path.Join(config.Conf.Core.BaseURL, gfs.ExtendID(parsedLine.ID))
			_ = util.Open(url)
		}
	}

	return nil
}

func init() {
	RootCmd.AddCommand(editCmd)
	editCmd.Flags().BoolVarP(&config.Conf.Flag.OpenURL, "open", "o", false, "Open with the default browser")
}
