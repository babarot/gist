package cmd

import (
	"github.com/b4b4r07/gist/cli/config"
	"github.com/b4b4r07/gist/cli/gist"
	"github.com/b4b4r07/gist/cli/screen"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit the gist file and sync after",
	Long:  "Edit the gist file and sync after",
	RunE:  edit,
}

func edit(cmd *cobra.Command, args []string) (err error) {
	s, err := screen.Open()
	if err != nil {
		return
	}

	rows, err := s.Select()
	if err != nil {
		return
	}

	client, err := gist.NewClient(config.Conf.Gist.Token)
	if err != nil {
		return
	}

	for _, row := range rows {
		if err = client.Edit(row.File); err != nil {
			return
		}
	}

	return
}

func init() {
	RootCmd.AddCommand(editCmd)
}
