package cmd

import (
	"fmt"

	"github.com/caarlos0/spin"
	"github.com/spf13/cobra"
)

type deleteCmd struct {
	meta
}

// newDeleteCmd creates a new delete command
func newDeleteCmd() *cobra.Command {
	c := &deleteCmd{}

	deleteCmd := &cobra.Command{
		Use:                   "delete",
		Short:                 "Delete gist file",
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

	return deleteCmd
}

func (c *deleteCmd) run(args []string) error {
	file, err := c.prompt()
	if err != nil {
		return err
	}

	s := spin.New("%s Deleting page...")
	s.Start()
	defer s.Stop()

	if err := c.gist.Delete(file.Gist); err != nil {
		return err
	}
	fmt.Println("Deleted")

	s.Stop()
	c.cache.Delete()

	return nil
}
