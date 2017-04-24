package cmd

import (
	"fmt"
	"log"

	"github.com/b4b4r07/gist/config"
	"github.com/b4b4r07/gist/gist"
	"github.com/b4b4r07/gist/util"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete gist files",
	Long:  "Delete gist files on the remote",
	RunE:  delete,
}

func delete(cmd *cobra.Command, args []string) error {
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

	var ids []string
	for _, line := range selectedLines {
		if line == "" {
			continue
		}
		parsedLine, err := gist.ParseLine(line)
		if err != nil {
			continue
		}
		ids = append(ids, parsedLine.ID)
	}

	ids = util.UniqueArray(ids)
	for _, id := range ids {
		err = gist.Delete(id)
		if err != nil {
			log.Printf("[ERROR] %v", err)
		}
		fmt.Printf("Deleted %s\n", id)
	}

	return nil
}

func init() {
	RootCmd.AddCommand(deleteCmd)
}
