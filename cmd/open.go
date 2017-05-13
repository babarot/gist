package cmd

//
// import (
// 	"errors"
// 	"net/url"
// 	"path"
//
// 	"github.com/b4b4r07/gist/api"
// 	"github.com/b4b4r07/gist/cli"
// 	"github.com/b4b4r07/gist/util"
// 	"github.com/spf13/cobra"
// )
//
// var openCmd = &cobra.Command{
// 	Use:   "open",
// 	Short: "Open user's gist",
// 	Long:  "Open user's gist",
// 	RunE:  open,
// }
//
// func openURL() error {
// 	gistURL := cli.Conf.Core.BaseURL
// 	if gistURL == "" {
// 		return errors.New("No specified gist base URL")
// 	}
//
// 	u, err := url.Parse(gistURL)
// 	if err != nil {
// 		return err
// 	}
//
// 	q := u.Query()
//
// 	user := cli.Conf.Core.User
// 	if user != "" {
// 		u.Path = path.Join(u.Path, user)
// 	}
//
// 	if cli.Conf.Flag.Sort == "updated" {
// 		q.Set("direction", "desc")
// 		q.Set("sort", "updated")
// 	}
// 	if cli.Conf.Flag.Only == "secret" || cli.Conf.Flag.Only == "private" {
// 		u.Path = path.Join(u.Path, "secret")
// 	}
//
// 	u.RawQuery = q.Encode()
//
// 	return util.Open(u.String())
// }
//
// func open(cmd *cobra.Command, args []string) error {
// 	if cli.Conf.Flag.OpenBaseURL {
// 		return openURL()
// 	}
// 	var err error
//
// 	gist, err := api.New(cli.Conf.Gist.Token)
// 	if err != nil {
// 		return err
// 	}
//
// 	gfs, err := gist.NewScreen()
// 	if err != nil {
// 		return err
// 	}
//
// 	selectedLines, err := util.Filter(gfs.Text)
// 	if err != nil {
// 		return err
// 	}
//
// 	if len(selectedLines) == 0 {
// 		return errors.New("No gist selected")
// 	}
//
// 	line, err := gist.ParseLine(selectedLines[0])
// 	if err != nil {
// 		return err
// 	}
//
// 	url := path.Join(cli.Conf.Core.BaseURL, line.ID)
// 	return util.Open(url)
// }
//
// func init() {
// 	RootCmd.AddCommand(openCmd)
// 	openCmd.Flags().StringVarP(&cli.Conf.Flag.Sort, "sort", "", "created", "Sort by the argument")
// 	openCmd.Flags().StringVarP(&cli.Conf.Flag.Only, "only", "", "", "Open only for the condition")
// 	openCmd.Flags().BoolVarP(&cli.Conf.Flag.OpenBaseURL, "no-select", "", false, "Open only gist base URL without selecting")
// 	openCmd.Flags().BoolVarP(&cli.Conf.Flag.OpenStarredItems, "starred", "s", false, "Open your starred gist")
// }
