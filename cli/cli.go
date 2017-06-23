package cli

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"

	"github.com/b4b4r07/gist/api"
	"github.com/pkg/browser"
)

func NewGist() (*api.Gist, error) {
	return api.NewGist(Conf.Gist.Token)
}

// TODO
var (
	ErrConfigEditor = errors.New("config editor not set")
)

func Edit(gist *api.Gist, fname string) error {
	return nil
	// if err := gist.Sync(fname); err != nil {
	// 	return err
	// }
	//
	// editor := Conf.Core.Editor
	// if editor == "" {
	// 	return ErrConfigEditor
	// }
	//
	// if err := Run(editor, fname); err != nil {
	// 	return err
	// }
	//
	// return gist.Sync(fname)
}

func Open(link string) error {
	_, err := url.ParseRequestURI(link)
	if err != nil {
		return err
	}
	return browser.OpenURL(link)
}

func GetPath(id string) (path string, err error) {
	path = filepath.Join(Conf.Gist.Dir, id)
	_, err = os.Stat(path)
	return
}
