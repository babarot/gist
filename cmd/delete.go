package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/b4b4r07/gist/cli/config"
	"github.com/b4b4r07/gist/cli/gist"
	"github.com/b4b4r07/gist/cli/screen"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete gist files",
	Long:  "Delete gist files on the remote",
	RunE:  delete,
}

func delete(cmd *cobra.Command, args []string) (err error) {
	s, err := screen.Open()
	if err != nil {
		return
	}

	rows, err := s.Select()
	if err != nil {
		return
	}

	rows = rows.Unique()
	if len(rows) > 0 {
		gist.NewCache().Clear()
	}

	client, err := gist.NewClient(config.Conf.Gist.Token)
	if err != nil {
		return
	}

	for _, row := range rows {
		err = client.Delete(row.ID)
		if err != nil {
			log.Printf("failed to delete from gist: %s\n", row.ID)
			continue
		}
		// remove from local
		_ = os.Remove(row.File.Path)
		fmt.Printf("Deleted %s\n", row.ID)
	}

	return nil
}

func init() {
	RootCmd.AddCommand(deleteCmd)
}
