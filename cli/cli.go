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

func Sync(gist *api.Gist, fname string) error {
	kind, content, err := gist.Compare(fname)
	if err != nil {
		return err
	}
	switch kind {
	case "local":
		err = gist.UpdateRemote(fname, content)
	case "remote":
		err = gist.UpdateLocal(fname, content)
	case "equal":
	case "":
		// Locally but not remote
	default:
	}
	return err
}
