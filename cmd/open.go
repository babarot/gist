package cmd

import (
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

type openCmd struct {
	meta
}

// newOpenCmd creates a new open command
func newOpenCmd() *cobra.Command {
	c := &openCmd{}

	openCmd := &cobra.Command{
		Use:                   "open",
		Short:                 "Open gist file",
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

	return openCmd
}

func (c *openCmd) run(args []string) error {
	file, err := c.prompt()
	if err != nil {
		return err
	}
	return browser.OpenURL(file.Page.URL)
}
