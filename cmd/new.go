package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/b4b4r07/gist/pkg/gist"
	"github.com/b4b4r07/gist/pkg/shell"
	"github.com/b4b4r07/gist/pkg/spin"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

type newCmd struct {
	meta

	private bool

	validator promptui.ValidateFunc
}

// newNewCmd creates a new new command
func newNewCmd() *cobra.Command {
	c := &newCmd{}

	newCmd := &cobra.Command{
		Use:                   "new",
		Short:                 "Create gist file",
		Aliases:               []string{},
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
		SilenceErrors:         true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO:
			// new command doesn't need to fetch all repos data
			// so divide this function to init and load.
			if err := c.meta.init(args); err != nil {
				return err
			}
			return c.run(args)
		},
	}

	f := newCmd.Flags()
	f.BoolVarP(&c.private, "private", "p", false, "make private")

	return newCmd
}

func (c *newCmd) run(args []string) error {
	c.validator = func(input string) error {
		if len(input) == 0 {
			return errors.New("filename must have more than 1 characters")
		}
		return nil
	}

	files, err := c.withNoArg()
	if err != nil {
		return err
	}

	prompt := promptui.Prompt{
		Label:    "Description",
		Validate: c.validator,
	}
	desc, err := prompt.Run()
	if err != nil {
		return err
	}

	s := spin.New("%s Creating page...")
	s.Start()
	defer s.Stop()

	url, err := c.gist.Create(gist.Page{
		Files:       files,
		Description: desc,
		Public:      !c.private,
	})
	if err != nil {
		return err
	}

	s.Stop()
	fmt.Println(url)

	return nil
}

func (c *newCmd) withNoArg() ([]gist.File, error) {
	var files []gist.File
	tmpfile, err := ioutil.TempFile("", "gist")
	if err != nil {
		return files, err
	}
	defer os.Remove(tmpfile.Name())
	defer tmpfile.Close()

	editor := shell.New(c.editor, tmpfile.Name())
	if err := editor.Run(context.Background()); err != nil {
		return files, err
	}

	content, err := ioutil.ReadFile(tmpfile.Name())
	if err != nil {
		return files, err
	}

	prompt := promptui.Prompt{
		Label:    "Filename",
		Validate: c.validator,
	}

	name, err := prompt.Run()
	if err != nil {
		return files, err
	}

	files = append(files, gist.File{
		Name:    name,
		Content: string(content),
	})

	return files, nil
}
