package cmd

import (
	"github.com/b4b4r07/gist/cli"
	"github.com/b4b4r07/gist/cli/gist"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open user's gist",
	Long:  "Open user's gist",
	RunE:  open,
}

func open(cmd *cobra.Command, args []string) (err error) {
	if cli.Conf.Flag.OpenBaseURL {
		return cli.Open(gist.YourURL)
	}

	screen, err := cli.NewScreen()
	if err != nil {
		return err
	}

	lines, err := screen.Select()
	if err != nil {
		return err
	}

	return cli.Open(lines[0].URL)
}

func init() {
	RootCmd.AddCommand(openCmd)
	openCmd.Flags().BoolVarP(&cli.Conf.Flag.OpenBaseURL, "no-select", "", false, "Open only gist base URL without selecting")
	openCmd.Flags().BoolVarP(&cli.Conf.Flag.StarredItems, "starred", "s", false, "Open your starred gist")
}
