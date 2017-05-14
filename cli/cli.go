package cli

import (
	"errors"

	"github.com/b4b4r07/gist/api"
)

func NewGist() (*api.Gist, error) {
	return api.NewGist(api.Config{
		Token:            Conf.Gist.Token,
		OpenStarredItems: Conf.Flag.OpenStarredItems,
		ShowIndicator:    Conf.Flag.ShowIndicator,
		NewPrivate:       Conf.Flag.NewPrivate,
		Dir:              Conf.Gist.Dir,
		BaseURL:          Conf.Core.BaseURL,
		Editor:           Conf.Core.Editor,
	})
}

func Edit(g *api.Gist, fname string) error {
	if err := g.Sync(fname); err != nil {
		return err
	}

	editor := g.Config.Editor
	if editor == "" {
		return errors.New("$EDITOR: not set")
	}

	if err := Run(editor, fname); err != nil {
		return err
	}

	return g.Sync(fname)
}
