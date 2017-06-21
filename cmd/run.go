package cmd

import (
	"github.com/b4b4r07/gist/cli"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the gist snippet as a script",
	Long:  "Run the gist snippet as a script",
	RunE:  run,
}

func run(cmd *cobra.Command, args []string) error {
	screen, err := cli.NewScreen()
	if err != nil {
		return err
	}

	lines, err := screen.Select()
	if err != nil {
		return err
	}

	for _, line := range lines {
		if err := cli.Runnable(line.Path); err != nil {
			continue
		}
		if err := cli.Run(line.Path, args...); err != nil {
			continue
		}
	}

	return nil
}

func init() {
	RootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVarP(&cli.Conf.Flag.StarredItems, "starred", "s", false, "Open your starred gist")
}
