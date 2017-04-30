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

type gistItem struct {
	files gist.Files
	desc  string
}

func new(cmd *cobra.Command, args []string) error {
	var err error
	var gi gistItem

	gist_, err := gist.New(config.Conf.Gist.Token)
	if err != nil {
		return err
	}

	// Make Gist from various conditions
	switch {
	case config.Conf.Flag.FromClipboard:
		gi, err = makeFromClipboard()
	case !terminal.IsTerminal(0):
		gi, err = makeFromStdin()
	case len(args) > 0:
		gi, err = makeFromArguments(args)
	case len(args) == 0:
		gi, err = makeFromEditor()
	}

	if err != nil {
		return err
	}

	url, err := gist_.Create(gi.files, gi.desc)
	if err != nil {
		return err
	}
	util.Underline("Created", url)

	if config.Conf.Flag.OpenURL {
		util.Open(url)
	}
	return nil
}

func makeFromClipboard() (gi gistItem, err error) {
	content, err := clipboard.ReadAll()
	if err != nil {
		return
	}
	if content == "" {
		return gi, errors.New("clipboard is empty")
	}
	filename, err := util.Scan(color.YellowString("Filename> "), !util.ScanAllowEmpty)
	if err != nil {
		return
	}
	desc, err := util.Scan(color.GreenString("Description> "), util.ScanAllowEmpty)
	if err != nil {
		return
	}
	return gistItem{
		files: gist.Files{gist.File{
			Filename: filename,
			Content:  content,
		}},
		desc: desc,
	}, nil
}

func makeFromStdin() (gi gistItem, err error) {
	body, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return
	}
	return gistItem{
		files: gist.Files{gist.File{
			Filename: "stdin",
			Content:  string(body),
		}},
		desc: "",
	}, nil
}

func makeFromEditor() (gi gistItem, err error) {
	filename, err := util.Scan(color.YellowString("Filename> "), !util.ScanAllowEmpty)
	if err != nil {
		return
	}
	f, err := util.TempFile(filename)
	defer os.Remove(f.Name())
	err = util.RunCommand(config.Conf.Core.Editor, f.Name())
	if err != nil {
		return
	}
	desc, err := util.Scan(color.GreenString("Description> "), util.ScanAllowEmpty)
	if err != nil {
		return
	}
	return gistItem{
		files: gist.Files{gist.File{
			Filename: filename,
			Content:  util.FileContent(f.Name()),
		}},
		desc: desc,
	}, nil
}

func makeFromArguments(args []string) (gi gistItem, err error) {
	var (
		gistFiles gist.Files
		files     []string
	)

	// Check if the path is directory
	isdir := func(path string) bool {
		if stat, err := os.Stat(path); err == nil && stat.IsDir() {
			return true
		}
		return false
	}

	for _, arg := range args {
		// if the arg is dir, walk within the dir and add them to slice
		// otherwise (regular file), just add it to slice
		if isdir(arg) {
			err = filepath.Walk(arg, func(arg string, info os.FileInfo, err error) error {
				if strings.HasPrefix(arg, ".") {
					return nil
				}
				if info.IsDir() {
					return nil
				}
				files = append(files, arg)
				return nil
			})
			if err != nil {
				return
			}
		} else {
			files = append(files, arg)
		}
	}

	if len(files) == 0 {
		return gi, errors.New("no files to be able create")
	}

	for _, file := range files {
		fmt.Fprintf(color.Output, "%s %s\n", color.YellowString("Filename>"), file)
		gistFiles = append(gistFiles, gist.File{
			Filename: filepath.Base(file),
			Content:  util.FileContent(file),
		})
	}

	desc, err := util.Scan(color.GreenString("Description> "), util.ScanAllowEmpty)
	if err != nil {
		return
	}

	return gistItem{
		files: gistFiles,
		desc:  desc,
	}, nil
}

func init() {
	RootCmd.AddCommand(newCmd)
	newCmd.Flags().BoolVarP(&config.Conf.Flag.OpenURL, "open", "o", false, "Open with the default browser")
	newCmd.Flags().BoolVarP(&config.Conf.Flag.Private, "private", "p", false, "Create as private gist")
	newCmd.Flags().BoolVarP(&config.Conf.Flag.FromClipboard, "from-clipboard", "c", false, "Create gist from clipboard")
}
