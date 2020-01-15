package cmd

import (
	"github.com/k0kubun/pp"
	"github.com/spf13/cobra"
)

type listCmd struct {
	meta
}

// newListCmd creates a new list command
func newListCmd() *cobra.Command {
	c := &listCmd{}

	listCmd := &cobra.Command{
		Use:                   "list",
		Short:                 "List gist files",
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

	return listCmd
}

func (c *listCmd) run(args []string) error {
	pp.Println(c.files)
	return nil
}
