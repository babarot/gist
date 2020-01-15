package cmd

import (
	"github.com/b4b4r07/gist/pkg/gist"
	"github.com/k0kubun/pp"
	"github.com/spf13/cobra"
)

type listCmd struct {
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
			return c.run(args)
		},
	}

	return listCmd
}

func (c *listCmd) run(args []string) error {
	files, err := gist.List("b4b4r07")
	if err != nil {
		return err
	}
	pp.Println(files)
	return nil
}
