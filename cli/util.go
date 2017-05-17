package cli

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/b4b4r07/gist/api"
)

func listFilesRecursively(dir string) (files []string, err error) {
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			// skip recursive
			if strings.HasPrefix(filepath.Base(path), ".") {
				return filepath.SkipDir
			}
			// skip
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

func syncFiles(gist *api.Gist) {
	files, err := listFilesRecursively(Conf.Gist.Dir)
	if err != nil {
		// as not error
		return
	}
	if len(files) == 0 {
		return
	}
	var wg sync.WaitGroup
	for _, file := range files {
		wg.Add(1)
		go func(file string) {
			defer wg.Done()
			if err := Sync(gist, file); err != nil {
				// do nothing for now
			}
		}(file)
	}
	wg.Wait()
}
