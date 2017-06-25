package cmd

import (
	"github.com/b4b4r07/gist/cli"
	"github.com/b4b4r07/gist/cli/config"
	"github.com/b4b4r07/gist/cli/gist"
	"github.com/b4b4r07/gist/cli/screen"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open user's gist",
	Long:  "Open user's gist",
	RunE:  open,
}

func open(cmd *cobra.Command, args []string) (err error) {
	if config.Conf.Flag.OpenBaseURL {
		return cli.Open(gist.YourURL)
	}

	s, err := screen.New()
	if err != nil {
		return err
	}

	lines, err := s.Select()
	if err != nil {
		return err
	}

	return cli.Open(lines[0].URL)
}

func init() {
	RootCmd.AddCommand(openCmd)
	openCmd.Flags().BoolVarP(&config.Conf.Flag.OpenBaseURL, "no-select", "", false, "Open only gist base URL without selecting")
	openCmd.Flags().BoolVarP(&config.Conf.Flag.StarredItems, "starred", "s", false, "Open your starred gist")
}
