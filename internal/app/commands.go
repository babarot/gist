package app

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/babarot/gist/v2/internal/cli"
	"github.com/babarot/gist/v2/internal/config"
)

func newListCommand(o *options, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	var f cli.ListOptions
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "print your gists",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return o.run(cmd, stdin, stdout, stderr, func(ctx context.Context, c *cli.CLI) error {
				return c.List(ctx, f)
			})
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&f.Public, "public", false, "only public gists")
	flags.BoolVar(&f.Secret, "secret", false, "only secret gists")
	flags.BoolVar(&f.Starred, "starred", false, "the gists you starred")
	flags.IntVar(&f.Limit, "limit", 0, "print at most this many")
	flags.BoolVar(&f.JSON, "json", false, "print JSON")
	flags.BoolVar(&f.Refresh, "refresh", false, "read the list from GitHub again")
	cmd.MarkFlagsMutuallyExclusive("public", "secret")
	return cmd
}

func newViewCommand(o *options, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	var f cli.ViewOptions
	cmd := &cobra.Command{
		Use:     "view <gist> [file]",
		Aliases: []string{"cat"},
		Short:   "print a gist's files as they are written",
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.run(cmd, stdin, stdout, stderr, func(ctx context.Context, c *cli.CLI) error {
				return c.View(ctx, args[0], at(args, 1), f)
			})
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&f.Files, "files", false, "print only the names of the files")
	flags.BoolVar(&f.JSON, "json", false, "print JSON, with the revision that gist edit --base takes")
	flags.BoolVar(&f.Web, "web", false, "open it in the browser")
	return cmd
}

// at is the argument at i, or "".
func at(args []string, i int) string {
	if i < len(args) {
		return args[i]
	}
	return ""
}

func newNewCommand(o *options, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	var f cli.NewOptions
	var public, secret bool
	cmd := &cobra.Command{
		Use:     "new [file...]",
		Aliases: []string{"create"},
		Short:   "create a gist and print its URL; it is secret unless --public",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := o.readConfig()
			if err != nil {
				fmt.Fprintf(stderr, "gist: %v\n", err)
				return exitCode(1)
			}
			f.Public = public || (!secret && cfg.New.Visibility == config.VisibilityPublic)
			return o.run(cmd, stdin, stdout, stderr, func(ctx context.Context, c *cli.CLI) error {
				return c.New(ctx, args, f)
			})
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&f.Description, "desc", "d", "", "the description")
	flags.BoolVar(&public, "public", false, "listed on your profile and searchable")
	flags.BoolVarP(&secret, "secret", "p", false, "only people with the URL can see it")
	flags.StringVarP(&f.Filename, "filename", "f", "", "the file name of what comes from stdin")
	flags.BoolVarP(&f.Open, "open", "o", false, "open it in the browser")
	flags.BoolVarP(&f.Copy, "copy", "c", false, "copy the URL to the clipboard")
	flags.BoolVar(&f.JSON, "json", false, "print JSON")
	cmd.MarkFlagsMutuallyExclusive("public", "secret")
	return cmd
}

func newEditCommand(o *options, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	var f cli.EditOptions
	var description, baseDescription string
	cmd := &cobra.Command{
		Use:   "edit <gist> [file]",
		Short: "change a gist: pipe a file's content, or use the flags",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("desc") {
				f.Description = &description
			}
			if cmd.Flags().Changed("base-desc") {
				if f.Description == nil {
					return errors.New("--base-desc goes with -d")
				}
				f.BaseDescription = &baseDescription
			}
			return o.run(cmd, stdin, stdout, stderr, func(ctx context.Context, c *cli.CLI) error {
				return c.Edit(ctx, args[0], at(args, 1), f)
			})
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&description, "desc", "d", "", "the new description")
	flags.StringArrayVar(&f.Add, "add", nil, "add this file")
	flags.StringArrayVar(&f.Remove, "remove", nil, "remove this file from the gist")
	flags.StringArrayVar(&f.Rename, "rename", nil, "rename a file: old=new")
	flags.StringVar(&f.Base, "base", "", "stop unless the gist is at this revision")
	flags.StringVar(&baseDescription, "base-desc", "", "stop unless the description is this")
	return cmd
}

func newDeleteCommand(o *options, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "delete <gist>...",
		Aliases: []string{"rm"},
		Short:   "delete gists from GitHub",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.run(cmd, stdin, stdout, stderr, func(ctx context.Context, c *cli.CLI) error {
				return c.Delete(ctx, args, yes)
			})
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "do not ask")
	return cmd
}

func newOpenCommand(o *options, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:     "open [gist]",
		Aliases: []string{"browse"},
		Short:   "open a gist in the browser, or your list of gists",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.run(cmd, stdin, stdout, stderr, func(ctx context.Context, c *cli.CLI) error {
				return c.OpenGist(ctx, at(args, 0))
			})
		},
	}
}

func newCloneCommand(o *options, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "clone <gist> [dir]",
		Short: "clone a gist with git",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.run(cmd, stdin, stdout, stderr, func(ctx context.Context, c *cli.CLI) error {
				return c.Clone(ctx, args[0], at(args, 1))
			})
		},
	}
}
