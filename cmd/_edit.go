package cmd

import (
	"github.com/b4b4r07/gist/cli"
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

	screen, err := cli.NewScreen()
	if err != nil {
		return err
	}

	rows, err := screen.Select()
	if err != nil {
		return err
	}

	for _, row := range rows {
		err = cli.Edit(screen.Gist, row.Path)
		if err != nil {
			return err
		}
		// TODO: edit description
		if cli.Conf.Flag.OpenURL {
			_ = util.Open(row.URL)
		}
	}

	return nil
}

func init() {
	RootCmd.AddCommand(editCmd)
	editCmd.Flags().BoolVarP(&cli.Conf.Flag.OpenURL, "open", "o", false, "Open with the default browser")
	editCmd.Flags().BoolVarP(&cli.Conf.Flag.EditDesc, "description", "d", false, "Edit only the description")
}
