package cli

import (
	"errors"

	"github.com/b4b4r07/gist/api"
)

func NewGist() (*api.Gist, error) {
	return api.NewGist(api.Config{
		Token:      Conf.Gist.Token,
		BaseURL:    Conf.Gist.BaseURL,
		NewPrivate: Conf.Flag.NewPrivate,
		ClonePath:  Conf.Gist.Dir,
	})
}

func Edit(g *api.Gist, fname string) error {
	if err := g.Sync(fname); err != nil {
		return err
	}

	editor := Conf.Core.Editor
	if editor == "" {
		return errors.New("$EDITOR: not set")
	}

	if err := Run(editor, fname); err != nil {
		return err
	}

	return g.Sync(fname)
}
