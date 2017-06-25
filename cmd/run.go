package cmd

import (
	"log"

	"github.com/b4b4r07/gist/cli"
	"github.com/b4b4r07/gist/cli/screen"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the gist snippet as a script",
	Long:  "Run the gist snippet as a script",
	RunE:  run,
}

func run(cmd *cobra.Command, args []string) error {
	s, err := screen.NewScreen()
	if err != nil {
		return err
	}

	rows, err := s.Select()
	if err != nil {
		return err
	}

	for _, row := range rows {
		if err := row.File.Execute(); err != nil {
			log.Print(err)
			continue
		}
	}

	return nil
}

func init() {
	RootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVarP(&cli.Conf.Flag.StarredItems, "starred", "s", false, "Open your starred gist")
}
