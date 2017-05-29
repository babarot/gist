package cmd

import (
	"fmt"

	"github.com/b4b4r07/gist/cli"
	"github.com/b4b4r07/gist/util"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get gist content",
	Long:  "Get gist content",
	RunE:  get,
}

func get(cmd *cobra.Command, args []string) error {
	screen, err := cli.NewScreen()
	if err != nil {
		return err
	}

	lines, err := screen.Select()
	if err != nil {
		return err
	}

	for _, line := range lines {
		content, err := util.FileContent(line.Path)
		if err != nil {
			continue
		}
		fmt.Printf(content)
	}

	return nil
}

func init() {
	RootCmd.AddCommand(getCmd)
}
