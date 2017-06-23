package api

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/b4b4r07/gist/util"
	"github.com/google/go-github/github"
	runewidth "github.com/mattn/go-runewidth"
)

func (g *Gist) Clone(dir string, item *github.Gist) error {
	if util.Exists(*item.ID) {
		return nil
	}

	oldwd, _ := os.Getwd()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.Mkdir(dir, 0700)
	}
	os.Chdir(dir)
	defer os.Chdir(oldwd)

	// TODO: Start()
	return exec.Command("git", "clone", *item.GitPullURL).Start()
}

// func (i *Items) Filter(fn func(Item) bool) *Items {
// 	items := make(Items, 0)
// 	for _, item := range *i {
// 		if fn(item) {
// 			items = append(items, item)
// 		}
// 	}
// 	return &items
// }
//
// func (i *Items) One() Item {
// 	var item Item
// 	if len(*i) > 0 {
// 		return (*i)[0]
// 	}
// 	return item
// }

func getID(file string) string {
	switch strings.Count(file, "/") {
	case 0:
		return file
	case 1:
		return filepath.Base(file)
	default:
		id := filepath.Base(filepath.Dir(file))
		if id == "files" {
			id = filepath.Base(file)
		}
		return id
	}
}

func (g *Gist) ExpandID(shortID string) (longID string, err error) {
	if len(g.Items) == 0 {
		return "", errors.New("no gist items")
	}
	for _, item := range g.Items {
		longID = *item.ID
		if shortID == ShortenID(longID) {
			return longID, nil
		}
	}
	return "", errors.New("no matched ID")
}

func ShortenID(longID string) string {
	return runewidth.Truncate(longID, IDLength, "")
}
