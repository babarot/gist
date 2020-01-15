package cmd

import (
	"os"

	"github.com/b4b4r07/gist/pkg/gist"
	"github.com/k0kubun/pp"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

type editCmd struct {
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
			return c.run(args)
		},
	}

	return editCmd
}

func (c *editCmd) run(args []string) error {
	gist, err := gist.New(os.Getenv("GITHUB_TOKEN"))
	if err != nil {
		return err
	}
	pages, err := gist.List("b4b4r07")
	if err != nil {
		return err
	}
	page, err := c.prompt(pages)
	if err != nil {
		return err
	}
	page, err = gist.Get(page.ID)
	if err != nil {
		return err
	}
	pp.Println(page)
	return nil
}

func (c *editCmd) prompt(pages []gist.Page) (gist.Page, error) {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   promptui.IconSelect + " {{ .ID | cyan }}",
		Inactive: "  {{ .ID | faint }}",
		Selected: promptui.IconGood + " {{ .ID }}",
		Details: `
{{ "Description:" | faint }}	{{ .Description }}
{{ "Public:" | faint }}	{{ .Public }}
		`,
	}

	// searcher := func(input string, index int) bool {
	// 	item := items[index]
	// 	name := strings.Replace(strings.ToLower(item.Slug), " ", "", -1)
	// 	input = strings.Replace(strings.ToLower(input), " ", "", -1)
	// 	return strings.Contains(name, input)
	// }

	prompt := promptui.Select{
		Label:     "Select a page",
		Items:     pages,
		Templates: templates,
		// Searcher:          searcher,
		StartInSearchMode: true,
		HideSelected:      true,
	}
	i, _, err := prompt.Run()
	return pages[i], err
}
