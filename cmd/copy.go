package cmd

import (
	"github.com/atotto/clipboard"
	"github.com/b4b4r07/gist/cli"
	"github.com/b4b4r07/gist/util"
	"github.com/spf13/cobra"
)

var copyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Copy contents from gist files",
	Long:  "Copy contents from gist files",
	RunE:  copy,
}

func copy(cmd *cobra.Command, args []string) error {
	screen, err := cli.NewScreen()
	if err != nil {
		return err
	}

	lines, err := screen.Select()
	if err != nil {
		return err
	}
	line := lines[0]
	content := util.FileContent(line.Path)

	return clipboard.WriteAll(content)
}

func init() {
	RootCmd.AddCommand(copyCmd)
}
