package cmd

import (
	// "path"
	"fmt"
	// "path/filepath"

	// "github.com/b4b4r07/gist/api"
	"github.com/b4b4r07/gist/cli"
	// "github.com/b4b4r07/gist/util"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit the gist file and sync after",
	Long:  "Edit the gist file and sync after",
	RunE:  edit,
}

func edit(cmd *cobra.Command, args []string) error {
	var err error

	// gist, err := api.New(cli.Conf.Gist.Token)
	// if err != nil {
	// 	return err
	// }

	screen, err := cli.NewScreen()
	if err != nil {
		return err
	}

	lines, err := screen.Select()
	if err != nil {
		return err
	}
	// selectedLines, err := util.Filter(screen.Text)
	// if err != nil {
	// 	return err
	// }

	fmt.Printf("%#v\n", lines)
	return nil
	// for _, line := range selectedLines {
	// 	if line == "" {
	// 		continue
	// 	}
	// 	line, err := gist.ParseLine(line)
	// 	if err != nil {
	// 		continue
	// 	}
	//
	// 	if cli.Conf.Flag.EditDesc {
	// 		util.ScanDefaultString = line.Description
	// 		desc, err := util.Scan(line.Filename+"> ", util.ScanAllowEmpty)
	// 		if err != nil {
	// 			return err
	// 		}
	// 		err = gist.EditDesc(line.ID, desc)
	// 		if err != nil {
	// 			return err
	// 		}
	// 	} else {
	// 		file := filepath.Join(cli.Conf.Gist.Dir, line.ID, line.Filename)
	// 		err = gist.Edit(file)
	// 		if err != nil {
	// 			return err
	// 		}
	// 	}
	//
	// 	if cli.Conf.Flag.OpenURL {
	// 		url := path.Join(cli.Conf.Core.BaseURL, line.ID)
	// 		_ = util.Open(url)
	// 	}
	// }
	//
	// return nil
}

func init() {
	RootCmd.AddCommand(editCmd)
	editCmd.Flags().BoolVarP(&cli.Conf.Flag.OpenURL, "open", "o", false, "Open with the default browser")
	editCmd.Flags().BoolVarP(&cli.Conf.Flag.EditDesc, "description", "d", false, "Edit only the description")
}
