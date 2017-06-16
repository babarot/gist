package cmd

import (
	"os"

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
		fi, err := os.Stat(line.Path)
		if err != nil {
			continue
		}
		var (
			origPerm = fi.Mode().Perm()
			execPerm = os.FileMode(0755).Perm()
		)
		if origPerm != execPerm {
			os.Chmod(line.Path, execPerm)
			defer os.Chmod(line.Path, origPerm)
		}
		if err := cli.Run(line.Path); err != nil {
			continue
		}
	}

	return nil
}

func init() {
	RootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVarP(&cli.Conf.Flag.StarredItems, "starred", "s", false, "Open your starred gist")
}
