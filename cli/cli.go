package cli

import (
	"errors"

	"github.com/b4b4r07/gist/api"
)

func NewGist() (*api.Gist, error) {
	return api.NewGist(Conf.Gist.Token)
}

// TODO
var (
	ErrConfigEditor = errors.New("config editor not set")
)

func Edit(gist *api.Gist, fname string) error {
	if err := gist.Sync(fname); err != nil {
		return err
	}

	editor := Conf.Core.Editor
	if editor == "" {
		return ErrConfigEditor
	}

	if err := Run(editor, fname); err != nil {
		return err
	}

	return gist.Sync(fname)
}
