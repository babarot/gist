package cli

import (
	"bytes"
	"errors"
	"strings"

	gist "github.com/b4b4r07/gist/api"
)

type (
	Screen struct {
		Items gist.Items
		Lines []string
	}
	Lines []string
	// Line  string
	// Lines []Line
)

func NewScreen() (s *Screen, err error) {
	gist, err := gist.NewGist(Conf.Gist.Token)
	if err != nil {
		return
	}
	items, err := gist.List()
	if err != nil {
		return
	}
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

func (s *Screen) Select() (items gist.Items, err error) {
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
			// TODO: log
			continue
		}
		items = append(items, item)
	}

	return
}

func (s *Screen) parse(line string) (gist.Item, error) {
	l := strings.Split(line, "\t")
	idx := func() int {
		for i, v := range Conf.Screen.Columns {
			if v == "{{.ID}}" {
				return i
			}
		}
		return -1
	}()
	if idx == -1 {
		// default
		idx = 0
	}
	for _, item := range s.Items {
		if item.ID == l[idx] {
			return item, nil
		}
	}
	return gist.Item{}, errors.New("not found")
}

/*
type Line struct {
	ID          string
	ShortID     string
	Description string
	Filename    string
	Path        string
	URL         string
	Public      bool
}

type Lines []Line

func (s *Screen) parseLine(line string) (*Line, error) {
	trimDirSymbol := func(id string) string {
		id = strings.TrimSpace(id)
		id = strings.TrimLeft(id, " | ")
		id = strings.TrimLeft(id, " + ")
		return id
	}
	trimPrivateSymbol := func(filename string) string {
		filename = strings.TrimSpace(filename)
		filename = strings.TrimLeft(filename, "* ")
		return filename
	}
	l := strings.Split(line, "\t")
	var (
		shortID  = trimDirSymbol(l[0])
		filename = trimPrivateSymbol(l[1])
		desc     = l[2]

		longID string
		err    error
	)

	longID, err = s.Gist.ExpandID(shortID)
	if err != nil {
		if Conf.Gist.UseCache && util.Exists(s.Cache.Path) {
			files, err := s.Cache.Load()
			if err != nil {
				return &Line{}, err
			}
			for _, file := range files {
				if file.ShortID == shortID {
					longID = file.ID
				}
			}
		} else {
			return &Line{}, err
		}
	}

	baseURL := Conf.Gist.BaseURL
	if baseURL == "" {
		baseURL = "https://gist.github.com"
	}

	return &Line{
		ID:          longID,
		ShortID:     shortID,
		Filename:    filename,
		Description: desc,
		Path:        filepath.Join(Conf.Gist.Dir, longID, filename),
		URL:         path.Join(baseURL, longID),
	}, nil
}

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
