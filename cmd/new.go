package cmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/b4b4r07/gist/api"
	"github.com/b4b4r07/gist/cli"
	"github.com/b4b4r07/gist/util"
	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"
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
	files api.Files
	desc  string
}

func new(cmd *cobra.Command, args []string) error {
	var err error
	var gi gistItem

	gist, err := cli.NewGist()
	if err != nil {
		return err
	}

	// Make Gist from various conditions
	switch {
	case cli.Conf.Flag.FromClipboard:
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

	item, err := gist.Create(gi.files, gi.desc)
	if err != nil {
		return err
	}

	if cli.Conf.Gist.UseCache {
		cache := cli.NewCache()
		files, err := cache.Load()
		if err != nil {
			return err
		}
		for _, file := range gi.files {
			// append to the top of slice (unshift)
			files, files[0] = append(files[0:1], files[0:]...), api.File{
				ID:          *item.ID,
				ShortID:     api.ShortenID(*item.ID),
				Filename:    file.Filename,
				Description: *item.Description,
				Public:      *item.Public,
			}
		}
		cache.Create(files)
	}

	util.Underline("Created", *item.HTMLURL)
	if cli.Conf.Flag.OpenURL {
		util.Open(*item.HTMLURL)
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
		files: api.Files{api.File{
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
	filename := util.RandomString(20)
	ext := cli.Conf.Gist.FileExt
	if len(ext) > 0 {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		filename = filename + ext
	}
	return gistItem{
		files: api.Files{api.File{
			Filename: filename,
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

	filename, title, asBlog := func(filename string) (string, string, bool) {
		if !cli.Conf.Flag.BlogMode {
			return filename, "", false
		}
		switch filepath.Ext(filename) {
		case "", ".md", ".mkd", ".markdown":
			return filename + ".md", "# " + filename, true
		default:
			return filename, filename, false
		}
	}(filename)

	f, err := util.TempFile(filename)
	defer os.Remove(f.Name())
	if asBlog {
		f.Write([]byte(title))
		f.Sync()
	}

	err = cli.Run(cli.Conf.Core.Editor, f.Name())
	if err != nil {
		return
	}

	content, _ := util.FileContent(f.Name())
	if content == title {
		return gi, errors.New("no contents")
	}

	desc, err := util.Scan(color.GreenString("Description> "), util.ScanAllowEmpty)
	if err != nil {
		return
	}

	return gistItem{
		files: api.Files{api.File{
			Filename: filename,
			Content:  content,
		}},
		desc: desc,
	}, nil
}

func makeFromArguments(args []string) (gi gistItem, err error) {
	var (
		gistFiles api.Files
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
			if !util.Exists(arg) {
				log.Infof("%s: no such file or directory", arg)
				continue
			}
			files = append(files, arg)
		}
	}

	if len(files) == 0 {
		return gi, errors.New("no files to be able create")
	}

	for _, file := range files {
		fmt.Fprintf(color.Output, "%s %s\n", color.YellowString("Filename>"), file)
		content, _ := util.FileContent(file)
		gistFiles = append(gistFiles, api.File{
			Filename: filepath.Base(file),
			Content:  content,
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
	newCmd.Flags().BoolVarP(&cli.Conf.Flag.OpenURL, "open", "o", false, "Open with the default browser")
	newCmd.Flags().BoolVarP(&cli.Conf.Flag.NewPrivate, "private", "p", false, "Create as private gist")
	newCmd.Flags().BoolVarP(&cli.Conf.Flag.FromClipboard, "from-clipboard", "c", false, "Create gist from clipboard")
}
