package cmd

//
// import (
// 	"errors"
// 	"path/filepath"
//
// 	"github.com/atotto/clipboard"
// 	"github.com/b4b4r07/gist/api"
// 	"github.com/b4b4r07/gist/cli"
// 	"github.com/b4b4r07/gist/util"
// 	"github.com/spf13/cobra"
// )
//
// var copyCmd = &cobra.Command{
// 	Use:   "copy",
// 	Short: "Copy contents from gist files",
// 	Long:  "Copy contents from gist files",
// 	RunE:  copy,
// }
//
// func copy(cmd *cobra.Command, args []string) error {
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
// 		return errors.New("no files selected")
// 	}
//
// 	line, err := gist.ParseLine(selectedLines[0])
// 	if err != nil {
// 		return err
// 	}
//
// 	file := filepath.Join(cli.Conf.Gist.Dir, line.Path)
// 	content := util.FileContent(file)
//
// 	return clipboard.WriteAll(content)
// }
//
// func init() {
// 	RootCmd.AddCommand(copyCmd)
// }
