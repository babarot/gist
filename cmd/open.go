package cmd

import (
	"errors"
	"net/url"
	"path"

	"github.com/b4b4r07/gist/cli"
	"github.com/b4b4r07/gist/util"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open user's gist",
	Long:  "Open user's gist",
	RunE:  open,
}

func openURL() error {
	gistURL := cli.Conf.Gist.BaseURL
	if gistURL == "" {
		return errors.New("No specified gist base URL")
	}

	u, err := url.Parse(gistURL)
	if err != nil {
		return err
	}

	q := u.Query()

	user := cli.Conf.Core.User
	if user != "" {
		u.Path = path.Join(u.Path, user)
	}
	u.RawQuery = q.Encode()

	return util.Open(u.String())
}

func open(cmd *cobra.Command, args []string) (err error) {
	if cli.Conf.Flag.OpenBaseURL {
		return openURL()
	}

	screen, err := cli.NewScreen()
	if err != nil {
		return err
	}

	lines, err := screen.Select()
	if err != nil {
		return err
	}
	line := lines[0]

	return util.Open(line.URL)
}

func init() {
	RootCmd.AddCommand(openCmd)
	openCmd.Flags().BoolVarP(&cli.Conf.Flag.OpenBaseURL, "no-select", "", false, "Open only gist base URL without selecting")
	openCmd.Flags().BoolVarP(&cli.Conf.Flag.StarredItems, "starred", "s", false, "Open your starred gist")
}
