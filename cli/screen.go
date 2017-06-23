package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	tt "text/template"

	gist "github.com/b4b4r07/gist/api"
)

var IDLength = gist.IDLength

type (
	Screen struct {
		Items Items
		Lines []string
	}
	Lines []string
	// Line  string
	// Lines []Line

	Item struct {
		ID          string
		ShortID     string
		Description string
		Public      bool
		Files       []File
	}
	Items []Item
	File  struct {
		Filename string
		Content  string
	}
	Files []File
)

func NewScreen() (s *Screen, err error) {
	gist, err := gist.NewGist(Conf.Gist.Token)
	if err != nil {
		return
	}
	resp, err := gist.List()
	if err != nil {
		return
	}
	items := convertItems(resp)
	lines := items.Render(Conf.Screen.Columns)

	// for screen cache
	// cache := NewCache()

	// var files Files
	// if cache.Available() && !Conf.Flag.StarredItems {
	// 	files, err = cache.Load()
	// 	if err != nil {
	// 		return s, err
	// 	}
	// } else {
	// 	files, err = Load(gist)
	// 	if err != nil {
	// 		return s, err
	// 	}
	// 	// sync files in background
	// 	lfs, _ := getLocalFiles()
	// 	gist.SyncAll(lfs)
	// 	if !Conf.Flag.StarredItems {
	// 		cache.Cache(files)
	// 	}
	// }

	return &Screen{
		Items: items,
		Lines: lines,
	}, nil
}

func (s *Screen) Select() (items Items, err error) {
	if len(s.Lines) == 0 {
		err = errors.New("no text to display")
		return
	}
	selectcmd := Conf.Core.SelectCmd
	if selectcmd == "" {
		err = errors.New("no selectcmd specified")
		return
	}

	text := strings.NewReader(strings.Join(s.Lines, "\n"))
	var buf bytes.Buffer
	err = runFilter(selectcmd, text, &buf)
	if err != nil {
		return
	}

	if buf.Len() == 0 {
		err = errors.New("no lines selected")
		return
	}

	selectedLines := strings.Split(buf.String(), "\n")
	for _, line := range selectedLines {
		if line == "" {
			continue
		}
		item, err := s.parse(line)
		if err != nil {
			continue
		}
		items = append(items, item)
	}

	return
}

func containsIndex(s string) int {
	for i, v := range Conf.Screen.Columns {
		if strings.Contains(v, s) {
			return i
		}
	}
	return -1
}

func (s *Screen) parse(line string) (Item, error) {
	l := strings.Split(line, "\t")
	var (
		id  = containsIndex("{{.ID}}")
		sid = containsIndex("{{.ShortID}}")
	)
	for _, item := range s.Items {
		// Strictly do not compare
		if id >= 0 && len(item.ID) >= IDLength && strings.Contains(l[id], item.ID) {
			return item, nil
		}
		if sid >= 0 && len(item.ShortID) >= IDLength && strings.Contains(l[sid], item.ShortID) {
			return item, nil
		}
	}
	return Item{}, errors.New("not found")
}

func convertItem(data gist.Item) Item {
	var files Files
	for _, file := range data.Files {
		files = append(files, File{
			Filename: file.Filename,
			Content:  file.Content,
		})
	}
	return Item{
		ID:          data.ID,
		ShortID:     data.ShortID,
		Description: data.Description,
		Public:      data.Public,
		Files:       files,
	}
}

func convertItems(data gist.Items) Items {
	var items Items
	for _, d := range data {
		items = append(items, convertItem(d))
	}
	return items
}

func (items *Items) Render(columns []string) []string {
	var lines []string
	max := 0
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

/*
func (l *Lines) Filter(fn func(Line) bool) *Lines {
	lines := make(Lines, 0)
	for _, line := range *l {
		if fn(line) {
			lines = append(lines, line)
		}
	}
	return &lines
}

func (l *Lines) Uniq() Lines {
	lines := make(Lines, 0)
	encountered := map[string]bool{}
	for _, line := range *l {
		if !encountered[line.ID] {
			encountered[line.ID] = true
			lines = append(lines, line)
		}
	}
	return lines
}

func getSize() (int, error) {
	w, _, err := terminal.GetSize(int(os.Stdout.Fd()))
	return w, err
}

func renderLines(files Files) (lines []string) {
	var line string
	var length int
	max := len(files) - 1
	prefixes := make([]string, max+1)
	var previous, current, next string
	for i, file := range files {
		if len(file.Filename) > length {
			length = len(file.Filename)
		}
		current = files[i].ID
		switch {
		case max == 0:
			break
		case i == 0:
			previous = ""
			next = files[i+1].ID
		case 0 < i && i < max:
			previous = files[i-1].ID
			next = files[i+1].ID
		case i == max:
			previous = files[i-1].ID
			next = ""
		}
		prefixes[i] = " "
		if current == previous {
			prefixes[i] = "|"
			if current != next {
				prefixes[i] = "+"
			}
		}
		if current == next {
			prefixes[i] = "|"
			if current != previous {
				prefixes[i] = "+"
			}
		}
	}

	format := fmt.Sprintf("%%-%ds\t%%-%ds\t%%s", IDLength, length)
	width, _ := getSize()
	if Conf.Flag.ShowIndicator {
		format = fmt.Sprintf(" %%s %%-%ds\t%%-%ds\t%%s", IDLength, length)
	}
	width = width - IDLength - length
	// TODO
	if width > 50 {
		width -= 10
	}

	for i, file := range files {
		filename := file.Filename
		if Conf.Flag.ShowPrivateSymbol {
			if file.Public {
				filename = "  " + file.Filename
			} else {
				filename = "* " + file.Filename
			}
		}
		desc := runewidth.Truncate(strings.Replace(file.Description, "\n", " ", -1), width-3, "...")
		if Conf.Flag.ShowIndicator {
			line = fmt.Sprintf(format, prefixes[i], file.ShortID, filename, desc)
		} else {
			line = fmt.Sprintf(format, file.ShortID, filename, desc)
		}
		lines = append(lines, line)
	}
	return lines
}

func Load(gist *api.Gist) (files Files, err error) {
	if Conf.Flag.StarredItems {
		err = gist.ListStarred()
	} else {
		err = gist.List()
	}
	if err != nil {
		return
	}

	for _, item := range gist.Items {
		if err := gist.Clone(Conf.Gist.Dir, item); err != nil {
			continue
		}
		desc := ""
		if item.Description != nil {
			desc = *item.Description
		}
		for _, file := range item.Files {
			files = append(files, api.File{
				ID:          *item.ID,
				ShortID:     api.ShortenID(*item.ID),
				Description: desc,
				Filename:    *file.Filename,
				Path:        filepath.Join(Conf.Gist.Dir, *item.ID, *file.Filename),
				Public:      *item.Public,
			})
		}
	}
	return files, nil
}

func getLocalFiles() (files []string, err error) {
	err = filepath.Walk(Conf.Gist.Dir, func(path string, info os.FileInfo, err error) error {
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
*/
