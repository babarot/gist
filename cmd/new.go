package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/b4b4r07/gist/config"
	"github.com/b4b4r07/gist/gist"
	"github.com/b4b4r07/gist/util"
	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new [FILE/DIR]",
	Short: "Create a new gist",
	Long:  `Create a new gist. If you pass file/dir paths, upload those files`,
	RunE:  new,
}

func scan(message string) (string, error) {
	tmp := "/tmp"
	if runtime.GOOS == "windows" {
		tmp = os.Getenv("TEMP")
	}
	l, err := readline.NewEx(&readline.Config{
		Prompt:            message,
		HistoryFile:       filepath.Join(tmp, "gist.txt"),
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	})
	if err != nil {
		return "", err
	}
	defer l.Close()

	for {
		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return line, nil
	}
	return "", errors.New("canceled")
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

	// TODO: refactoring
	if len(args) > 0 {
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
	} else {
		filename, err := scan(color.YellowString("Filename> "))
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
	}

	desc, err = scan(color.GreenString("Description> "))
	if err != nil {
		return err
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
}
