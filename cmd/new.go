package cmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/b4b4r07/gist/config"
	"github.com/b4b4r07/gist/gist"
	"github.com/b4b4r07/gist/util"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"golang.org/x/crypto/ssh/terminal"
)

var newCmd = &cobra.Command{
	Use:   "new [FILE/DIR]",
	Short: "Create a new gist",
	Long:  `Create a new gist. If you pass file/dir paths, upload those files`,
	RunE:  new,
}

func new(cmd *cobra.Command, args []string) error {
	var fname string
	var desc string
	var err error

	gist_, err := gist.New(config.Conf.Gist.Token)
	if err != nil {
		return err
	}

	var gistFiles []gist.File

	switch {
	case config.Conf.Flag.FromClipboard:
		content, err := clipboard.ReadAll()
		if err != nil {
			return err
		}
		if content == "" {
			return errors.New("clipboard is empty")
		}
		filename, err := util.Scan(color.YellowString("Filename> "), !util.ScanAllowEmpty)
		if err != nil {
			return err
		}
		desc, err = util.Scan(color.GreenString("Description> "), util.ScanAllowEmpty)
		if err != nil {
			return err
		}
		gistFiles = append(gistFiles, gist.File{
			Filename: filename,
			Content:  content,
		})
	case !terminal.IsTerminal(0):
		body, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		gistFiles = append(gistFiles, gist.File{
			Filename: "stdin",
			Content:  string(body),
		})
		desc = ""
	case len(args) > 0:
		target := args[0]
		files := []string{}
		err = filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
			if strings.HasPrefix(path, ".") {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return fmt.Errorf("%s: no files", target)
		}
		for _, file := range files {
			fmt.Fprintf(color.Output, "%s %s\n", color.YellowString("Filename>"), file)
			gistFiles = append(gistFiles, gist.File{
				Filename: filepath.Base(file),
				Content:  util.FileContent(file),
			})
		}
		desc, err = util.Scan(color.GreenString("Description> "), util.ScanAllowEmpty)
		if err != nil {
			return err
		}
	case len(args) == 0:
		filename, err := util.Scan(color.YellowString("Filename> "), !util.ScanAllowEmpty)
		if err != nil {
			return err
		}
		f, err := util.TempFile(filename)
		defer os.Remove(f.Name())
		fname = f.Name()
		err = util.RunCommand(config.Conf.Core.Editor, fname)
		if err != nil {
			return err
		}
		gistFiles = append(gistFiles, gist.File{
			Filename: filename,
			Content:  util.FileContent(fname),
		})
		desc, err = util.Scan(color.GreenString("Description> "), util.ScanAllowEmpty)
		if err != nil {
			return err
		}
	}

	url, err := gist_.Create(gistFiles, desc)
	if err != nil {
		return err
	}
	util.Underline("Created", url)

	if config.Conf.Flag.OpenURL {
		util.Open(url)
	}
	return nil
}

func init() {
	RootCmd.AddCommand(newCmd)
	newCmd.Flags().BoolVarP(&config.Conf.Flag.OpenURL, "open", "o", false, "Open with the default browser")
	newCmd.Flags().BoolVarP(&config.Conf.Flag.Private, "private", "p", false, "Create as private gist")
	newCmd.Flags().BoolVarP(&config.Conf.Flag.FromClipboard, "from-clipboard", "c", false, "Create gist from clipboard")
}
