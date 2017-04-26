package cmd

import (
	"errors"
	"net/url"
	"path"

	"github.com/b4b4r07/gist/config"
	"github.com/b4b4r07/gist/gist"
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
	gistURL := config.Conf.Core.BaseURL
	if gistURL == "" {
		return errors.New("No specified gist base URL")
	}

	u, err := url.Parse(gistURL)
	if err != nil {
		return err
	}

	q := u.Query()

	user := config.Conf.Core.User
	if user != "" {
		u.Path = path.Join(u.Path, user)
	}

	if config.Conf.Flag.Sort == "updated" {
		q.Set("direction", "desc")
		q.Set("sort", "updated")
	}
	if config.Conf.Flag.Only == "secret" || config.Conf.Flag.Only == "private" {
		u.Path = path.Join(u.Path, "secret")
	}

	u.RawQuery = q.Encode()

	return util.Open(u.String())
}

func open(cmd *cobra.Command, args []string) error {
	if config.Conf.Flag.NoSelect {
		return openURL()
	}
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

	if len(selectedLines) == 0 {
		return errors.New("No gist selected")
	}

	line, err := gist.ParseLine(selectedLines[0])
	if err != nil {
		return err
	}

	url := path.Join(config.Conf.Core.BaseURL, line.ID)
	return util.Open(url)
}

func init() {
	RootCmd.AddCommand(openCmd)
	openCmd.Flags().StringVarP(&config.Conf.Flag.Sort, "sort", "", "created", "Sort by the argument")
	openCmd.Flags().StringVarP(&config.Conf.Flag.Only, "only", "", "", "Open only for the condition")
	openCmd.Flags().BoolVarP(&config.Conf.Flag.NoSelect, "no-select", "", false, "Open only gist base URL without selecting")
	openCmd.Flags().BoolVarP(&config.Conf.Flag.OpenStarredItems, "starred", "s", false, "Open your starred gist")
}
