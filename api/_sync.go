package api

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/b4b4r07/gist/util"
	"github.com/google/go-github/github"
)

func (g *Gist) Compare(fname string) (kind, content string, err error) {
	if len(g.Items) == 0 {
		err = g.List()
		if err != nil {
			return
		}
	}

	fi, err := os.Stat(fname)
	if err != nil {
		// TODO:
		// case -> there is a dir but file has already deleted
		// item := g.Items.Filter(func(i Item) bool {
		// 	return *i.ID == getID(fname)
		// }).One()
		// TODO
		// err = g.Clone(item)
		// err = errors.Wrapf(err, "%s: no such file or directory", fname)
		return
	}

	item := g.Items.Filter(func(i Item) bool {
		return *i.ID == getID(fname)
	}).One()
	if item == nil {
		err = fmt.Errorf("%s: not found in gist", getID(fname))
		return
	}

	ctx := context.Background()
	gist, _, err := g.Client.Gists.Get(ctx, *item.ID)
	if err != nil {
		return
	}
	var (
		remoteContent, localContent string
	)
	localContent, _ = util.FileContent(fname)
	for _, file := range gist.Files {
		if *file.Filename != filepath.Base(fname) {
			return "", "", fmt.Errorf("%s: not found on cloud", filepath.Base(fname))
		}
		remoteContent = *file.Content
	}
	if remoteContent == localContent {
		return "equal", "", nil
	}

	local := fi.ModTime().UTC()
	remote := item.UpdatedAt.UTC()

	switch {
	case local.After(remote):
		return "local", localContent, nil
	case remote.After(local):
		return "remote", remoteContent, nil
	default:
	}

	return "equal", "", nil
}

func (g *Gist) UpdateLocal(fname, content string) error {
	return ioutil.WriteFile(fname, []byte(content), os.ModePerm)
}

func (g *Gist) UpdateRemote(fname, content string) error {
	var (
		gist = func(fname string) github.Gist {
			return github.Gist{
				Files: map[github.GistFilename]github.GistFile{
					github.GistFilename(filepath.Base(fname)): github.GistFile{
						Content: github.String(content),
					},
				},
			}
		}(fname)
		id = getID(fname)
	)
	ctx := context.Background()
	_, _, err := g.Client.Gists.Edit(ctx, id, &gist)
	return err
}

func (g *Gist) sync(file string) (msg string, err error) {
	kind, content, err := g.Compare(file)
	if err != nil {
		return "", err
	}
	switch kind {
	case "local":
		err = g.UpdateRemote(file, content)
		msg = "Uploaded"
	case "remote":
		err = g.UpdateLocal(file, content)
		msg = "Downloaded"
	case "equal":
		// Do nothing
	case "":
		// Locally but not remote
	default:
	}
	return msg, err
}

func (g *Gist) Sync(file string) (err error) {
	var msg string
	spn := util.NewSpinner("Checking...")
	spn.Start()
	defer func() {
		spn.Stop()
		util.Underline(msg, path.Join("https://gist.github.com", getID(file)))
	}()
	msg, err = g.sync(file)
	return err
}

func (g *Gist) SyncAll(files []string) {
	var wg sync.WaitGroup
	for _, file := range files {
		wg.Add(1)
		go func(file string) {
			defer wg.Done()
			// ignore error for now
			g.sync(file)
		}(file)
	}
	wg.Wait()
}
