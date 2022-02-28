package cmd

import (
	"context"
	"fmt"

	"github.com/b4b4r07/gist/pkg/shell"
	"github.com/b4b4r07/gist/pkg/spin"
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

	editor := shell.New(c.editor, file.FullPath)
	if err := editor.Run(context.Background()); err != nil {
		return err
	}

	updated, err := file.HasUpdated()
	if err != nil {
		return err
	}

	if !updated {
		return nil
	}

	s := spin.New("%s Pushing...")
	s.Start()
	defer s.Stop()

	if err := file.Update(); err != nil {
		return err
	}

	s.Stop()
	fmt.Printf("Pushed: %s\n", file.URL)

	return nil
}
