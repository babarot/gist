package cmd

import (
	"fmt"
	"log"

	"github.com/b4b4r07/gist/cli"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete gist files",
	Long:  "Delete gist files on the remote",
	RunE:  delete,
}

func delete(cmd *cobra.Command, args []string) (err error) {
	screen, err := cli.NewScreen()
	if err != nil {
		return err
	}

	lines, err := screen.Select()
	if err != nil {
		return err
	}

	for _, line := range lines.Uniq() {
		err = screen.Gist.Delete(line.ID)
		if err != nil {
			log.Printf("[ERROR] %v", err)
		}
		fmt.Printf("Deleted %s\n", line.ID)
	}

	return nil
}

func init() {
	RootCmd.AddCommand(deleteCmd)
}
