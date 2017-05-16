package cli

import (
	"errors"
	"fmt"

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

func Edit(gist *api.Gist, fname string) error {
	if err := gist.Sync(fname); err != nil {
		return err
	}

	editor := Conf.Core.Editor
	if editor == "" {
		return errors.New("$EDITOR: not set")
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
		fmt.Printf("Uploaded\t%s\n", fname)
	case "remote":
		err = gist.UpdateLocal(fname, content)
		fmt.Printf("Downloaded\t%s\n", fname)
	case "equal":
		fmt.Printf("Not changed\t%s\n", fname)
	case "":
		// Locally but not remote
	default:
	}
	return err
}
