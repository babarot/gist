package gist

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	tt "text/template"

	"github.com/b4b4r07/gist/api"
	"github.com/b4b4r07/gist/cli"
	"github.com/b4b4r07/gist/cli/config"
	runewidth "github.com/mattn/go-runewidth"
)

func convertItem(item api.Item) Item {
	var files Files
	for _, file := range item.Files {
		files = append(files, File{
			ItemID:   item.ID,
			Filename: file.Filename,
			Content:  file.Content,
			// original field
			Path: filepath.Join(Dir, item.ID, file.Filename),
		})
	}
	return Item{
		ID:          item.ID,
		ShortID:     item.ShortID,
		Description: item.Description,
		Public:      item.Public,
		Files:       files,
		// original field
		URL: func() string {
			u, _ := url.Parse(BaseURL)
			u.Path = path.Join(u.Path, item.ID)
			return u.String()
		}(),
		Path: filepath.Join(Dir, item.ID),
	}
}

func convertItems(data api.Items) Items {
	var items Items
	for _, d := range data {
		items = append(items, convertItem(d))
	}
	return items
}

func (items *Items) Render() []string {
	var (
		lines []string

		columns = config.Conf.Screen.Columns
		max     = 0
	)
	for _, item := range *items {
		for _, file := range item.Files {
			if len(file.Filename) > max {
				max = len(file.Filename)
			}
		}
	}
	for _, item := range *items {
		var line string
		var tmpl *tt.Template
		if len(columns) == 0 {
			columns = []string{"{{.ID}}"}
		}
		fnfmt := fmt.Sprintf("%%-%ds", max)
		for _, file := range item.Files {
			format := columns[0]
			for _, v := range columns[1:] {
				format += "\t" + v
			}
			t, err := tt.New("format").Parse(format)
			if err != nil {
				return []string{}
			}
			tmpl = t
			if tmpl != nil {
				var b bytes.Buffer
				err := tmpl.Execute(&b, map[string]interface{}{
					"ID":          item.ID,
					"ShortID":     item.ShortID,
					"Description": item.Description,
					"Filename":    fmt.Sprintf(fnfmt, file.Filename),
					"PrivateMark": func() string {
						if item.Public {
							return " "
						}
						return "*"
					}(),
				})
				if err != nil {
					return []string{}
				}
				line = b.String()
			}
			lines = append(lines, line)
		}
	}
	return lines
}

func (i *Items) Unique() Items {
	items := make(Items, 0)
	encountered := map[string]bool{}
	for _, item := range *i {
		if !encountered[item.ID] {
			encountered[item.ID] = true
			items = append(items, item)
		}
	}
	return items
}

func (i *Items) Filter(fn func(Item) bool) *Items {
	items := make(Items, 0)
	for _, item := range *i {
		if fn(item) {
			items = append(items, item)
		}
	}
	return &items
}

func (i *Items) One() Item {
	var item Item
	if len(*i) > 0 {
		return (*i)[0]
	}
	return item
}

func ShortenID(id string) string {
	return runewidth.Truncate(id, api.IDLength, "")
}

func (f *File) Exist() bool {
	_, err := os.Stat(f.Path)
	return err == nil
}

func (i *Item) Exist() bool {
	_, err := os.Stat(i.Path)
	return err == nil
}

func (i *Item) Exists() bool {
	if !i.Exist() {
		return false
	}
	for _, file := range i.Files {
		if !file.Exist() {
			return false
		}
	}
	return true
}

func (i *Item) Clone() error {
	if i.Exists() {
		return nil
	}
	os.RemoveAll(i.Path)
	cwd, _ := os.Getwd()
	os.Chdir(config.Conf.Gist.Dir)
	defer os.Chdir(cwd)
	return exec.Command("git", "clone", i.URL).Run()
}

func (i *Items) CloneAll() {
	s := cli.NewSpinner("Cloning...")
	s.Start()
	defer s.Stop()
	var wg sync.WaitGroup
	for _, item := range *i {
		wg.Add(1)
		go func(item Item) {
			defer wg.Done()
			item.Clone()
		}(item)
	}
	wg.Wait()
}

func inSlice(slice []string, e string) bool {
	for _, s := range slice {
		if s == e {
			return true
		}
	}
	return false
}

func collect(vs []string, f func(string) string) []string {
	vsm := make([]string, len(vs))
	for i, v := range vs {
		vsm[i] = f(v)
	}
	return vsm
}

func (f *File) Runnable() bool {
	return inSlice(collect(config.Conf.Gist.RunnableExt, func(ext string) string {
		return strings.TrimPrefix(ext, ".")
	}), strings.TrimPrefix(filepath.Ext(f.Path), "."))
}

func (f *File) Run(args []string) error {
	fi, err := os.Stat(f.Path)
	if err != nil {
		return err
	}
	if !f.Runnable() {
		return fmt.Errorf("%s: not in config.gist.runnable_ext", filepath.Ext(f.Filename))
	}
	var (
		origPerm = fi.Mode().Perm()
		execPerm = os.FileMode(0755).Perm()
	)
	if origPerm != execPerm {
		os.Chmod(f.Path, execPerm)
		defer os.Chmod(f.Path, origPerm)
	}
	cmd := exec.Command(f.Path, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (f *File) GetContent() string {
	if len(f.Content) > 0 {
		return f.Content
	}
	data, _ := ioutil.ReadFile(f.Path)
	return string(data)
}
