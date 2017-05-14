package cli

import (
	"errors"

	"github.com/b4b4r07/gist/api"
)

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
