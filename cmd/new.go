package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/b4b4r07/gist/cli"
	"github.com/b4b4r07/gist/cli/config"
	"github.com/b4b4r07/gist/cli/gist"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
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

func new(cmd *cobra.Command, args []string) (err error) {
	var gi gistItem

	client, err := gist.NewClient(config.Conf.Gist.Token)
	if err != nil {
		return
	}

	// Make Gist from various conditions
	switch {
	// case config.Conf.Flag.FromClipboard:
	// 	gi, err = makeFromClipboard()
	// case !terminal.IsTerminal(0):
	// 	gi, err = makeFromStdin()
	case len(args) > 0:
		gi, err = makeFromArguments(args)
	case len(args) == 0:
		gi, err = makeFromEditor()
	}
	if err != nil {
		return
	}

	gist.Dir = config.Conf.Gist.Dir
	item, err := client.Create(gi.files, gi.desc, config.Conf.Flag.NewPrivate)
	if err != nil {
		return
	}
	item.Clone()

	if config.Conf.Gist.UseCache {
		cache := cli.NewCache()
		if items, err := cache.Load(); err == nil {
			// append to the top of slice (unshift)
			if len(items) > 0 {
				items, items[0] = append(items[0:1], items[0:]...), item
			} else {
				items = append(items, item)
			}
			cache.Cache(items)
		}
	}

	cli.Underline("Created", item.URL)
	if config.Conf.Flag.OpenURL {
		return cli.Open(item.URL)
	}

	return nil
}

func makeFromEditor() (gi gistItem, err error) {
	filename, err := cli.Scan(color.YellowString("Filename> "), !cli.ScanAllowEmpty)
	if err != nil {
		return
	}

	filename, title, asBlog := func(filename string) (string, string, bool) {
		if !config.Conf.Flag.BlogMode {
			return filename, "", false
		}
		switch filepath.Ext(filename) {
		case "", ".md", ".mkd", ".markdown":
			return filename + ".md", "# " + filename, true
		default:
			return filename, filename, false
		}
	}(filename)

	f, err := cli.TempFile(filename)
	defer os.Remove(f.Name())
	if asBlog {
		f.Write([]byte(title))
		f.Sync()
	}

	err = cli.Run(config.Conf.Core.Editor, f.Name())
	if err != nil {
		return
	}

	content, _ := cli.FileContent(f.Name())
	if content == title {
		return gi, errors.New("no contents")
	}

	desc, err := cli.Scan(color.GreenString("Description> "), cli.ScanAllowEmpty)
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
			if _, err := os.Stat(arg); err != nil {
				fmt.Fprintf(os.Stderr, "%s: no such file or directory\n", arg)
				continue
			}
			files = append(files, arg)
		}
	}

	switch len(files) {
	case 0:
		return gi, errors.New("no files to be able create")
	case 1:
		file := files[0]
		cli.ScanDefaultString = file
		file, err = cli.Scan(color.YellowString("Filename> "), !cli.ScanAllowEmpty)
		if err != nil {
			return
		}
		content, _ := cli.FileContent(files[0]) // Use original file: files[0]
		gistFiles = append(gistFiles, gist.File{
			Filename: filepath.Base(file),
			Content:  content,
		})
		cli.ScanDefaultString = "" // reset deafult string
	default:
		for _, file := range files {
			fmt.Fprintf(color.Output, "%s %s\n", color.YellowString("Filename>"), file)
			content, _ := cli.FileContent(file)
			gistFiles = append(gistFiles, gist.File{
				Filename: filepath.Base(file),
				Content:  content,
			})
		}
	}

	desc, err := cli.Scan(color.GreenString("Description> "), cli.ScanAllowEmpty)
	if err != nil {
		return
	}

	return gistItem{
		files: gistFiles,
		desc:  desc,
	}, nil
}

/*
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
	ext := config.Conf.Gist.FileExt
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
*/

func init() {
	RootCmd.AddCommand(newCmd)
	newCmd.Flags().BoolVarP(&config.Conf.Flag.OpenURL, "open", "o", false, "Open with the default browser")
	newCmd.Flags().BoolVarP(&config.Conf.Flag.NewPrivate, "private", "p", false, "Create as private gist")
	newCmd.Flags().BoolVarP(&config.Conf.Flag.FromClipboard, "from-clipboard", "c", false, "Create gist from clipboard")
}
