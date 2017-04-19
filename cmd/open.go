package cmd

import (
	"errors"
	"net/url"
	"path"

	"github.com/b4b4r07/gist/config"
	"github.com/b4b4r07/gist/util"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open user's gist",
	Long:  "Open user's gist",
	RunE:  open,
}

func open(cmd *cobra.Command, args []string) error {
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

func init() {
	RootCmd.AddCommand(openCmd)
	openCmd.Flags().StringVarP(&config.Conf.Flag.Sort, "sort", "", "created", "Sort")
	openCmd.Flags().StringVarP(&config.Conf.Flag.Only, "only", "", "", "Only")
}
