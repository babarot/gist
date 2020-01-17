package cmd

import (
	"context"
	"fmt"

	"github.com/b4b4r07/gist/pkg/shell"
	"github.com/caarlos0/spin"
	"github.com/spf13/cobra"
)

type editCmd struct {
	meta
}

// newEditCmd creates a new edit command
func newEditCmd() *cobra.Command {
	c := &editCmd{}

	editCmd := &cobra.Command{
		Use:                   "edit",
		Short:                 "Edit gist files",
		Aliases:               []string{},
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
		SilenceErrors:         true,
		Args:                  cobra.MaximumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := c.meta.init(args); err != nil {
				return err
			}
			return c.run(args)
		},
	}

	return editCmd
}

func (c *editCmd) run(args []string) error {
	file, err := c.prompt()
	if err != nil {
		return err
	}

	editor := shell.New(c.gist.Editor, file.FullPath)
	if err := editor.Run(context.Background()); err != nil {
		return err
	}

	s := spin.New("%s Pushing...")
	s.Start()
	defer s.Stop()

	if err := file.Upload(); err != nil {
		return err
	}

	s.Stop()
	fmt.Println("Pushed")

	return nil
}
