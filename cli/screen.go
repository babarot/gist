package cli

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/b4b4r07/gist/api"
	"github.com/b4b4r07/gist/util"
	"github.com/mattn/go-runewidth"
)

type Screen struct {
	Gist  *api.Gist
	Files api.Files
	Text  string
	id    func(string) string
}

func NewScreen() (s *Screen, err error) {
	spn := util.NewSpinner("Fetching...")
	spn.Start()
	defer spn.Stop()

	// // fetch remote files
	// if g.Config.OpenStarredItems {
	// 	err = g.getStarredItems()
	// } else {
	// 	err = g.getItems()
	// }
	gist, err := api.NewGist(Conf.Gist.Token)
	if err != nil {
		return s, err
	}

	if err := gist.Get(); err != nil {
		return s, err
	}

	var files api.Files
	for _, item := range gist.Items {
		desc := ""
		if item.Description != nil {
			desc = *item.Description
		}
		for _, file := range item.Files {
			files = append(files, api.File{
				ID:          *item.ID,
				ShortID:     shortenID(*item.ID),
				Filename:    *file.Filename,
				Path:        filepath.Join(*item.ID, *file.Filename),
				Description: desc,
				Public:      *item.Public,
			})
		}
	}

	id := func(id string) string {
		for _, item := range gist.Items {
			if id == shortenID(*item.ID) {
				return *item.ID
			}
		}
		return ""
	}

	// var files Files
	// for _, item := range g.Items {
	// 	if err := g.cloneGist(item); err != nil {
	// 		continue
	// 	}
	// 	desc := ""
	// 	if item.Description != nil {
	// 		desc = *item.Description
	// 	}
	// 	for _, f := range item.Files {
	// 		files = append(files, File{
	// 			ID:          *item.ID,
	// 			ShortID:     shortenID(*item.ID),
	// 			Filename:    *f.Filename,
	// 			Path:        filepath.Join(*item.ID, *f.Filename),
	// 			Description: desc,
	// 			Public:      *item.Public,
	// 		})
	// 	}
	// }
	//
	// var text string
	// var length int
	// max := len(files) - 1
	// prefixes := make([]string, max+1)
	// var previous, current, next string
	// for i, file := range files {
	// 	if len(file.Filename) > length {
	// 		length = len(file.Filename)
	// 	}
	// 	current = files[i].ID
	// 	switch {
	// 	case i == 0:
	// 		previous = ""
	// 		next = files[i+1].ID
	// 	case 0 < i && i < max:
	// 		previous = files[i-1].ID
	// 		next = files[i+1].ID
	// 	case i == max:
	// 		previous = files[i-1].ID
	// 		next = ""
	// 	}
	// 	prefixes[i] = " "
	// 	if current == previous {
	// 		prefixes[i] = "|"
	// 		if current != next {
	// 			prefixes[i] = "+"
	// 		}
	// 	}
	// 	if current == next {
	// 		prefixes[i] = "|"
	// 		if current != previous {
	// 			prefixes[i] = "+"
	// 		}
	// 	}
	// }
	//
	// format := fmt.Sprintf("%%-%ds\t%%-%ds\t%%s\n", IDLength, length)
	// width, _ := getSize()
	// if g.Config.ShowIndicator {
	// 	format = fmt.Sprintf(" %%s %%-%ds\t%%-%ds\t%%s\n", IDLength, length)
	// }
	// width = width - IDLength - length
	// // TODO
	// if width > 50 {
	// 	width -= 10
	// }
	// for i, file := range files {
	// 	filename := file.Filename
	// 	if g.Config.ShowPrivateSymbol {
	// 		if file.Public {
	// 			filename = "  " + file.Filename
	// 		} else {
	// 			filename = "* " + file.Filename
	// 		}
	// 	}
	// 	desc := runewidth.Truncate(strings.Replace(file.Description, "\n", " ", -1), width-3, "...")
	// 	if g.Config.ShowIndicator {
	// 		text += fmt.Sprintf(format, prefixes[i], file.ShortID, filename, desc)
	// 	} else {
	// 		text += fmt.Sprintf(format, file.ShortID, filename, desc)
	// 	}
	// }

	text := ""
	for _, file := range files {
		filename := file.Filename
		desc := runewidth.Truncate(strings.Replace(file.Description, "\n", " ", -1), 80, "...")
		text += fmt.Sprintf("%s\t%s\t%s\n", file.ShortID, filename, desc)
	}

	return &Screen{
		Gist:  gist,
		Files: files,
		Text:  text,
		id:    id,
	}, nil
}

type Line struct {
	Line     string
	ID       string
	ShortID  string
	Filename string
	Desc     string
	Path     string
	URL      string
}

type Lines []Line

func (s *Screen) parseLine(line string) *Line {
	l := strings.Split(line, "\t")
	return &Line{
		Line:     line,
		ID:       s.id(l[0]),
		ShortID:  l[0],
		Filename: l[1],
		Desc:     l[2],
		Path:     filepath.Join(Conf.Gist.Dir, s.id(l[0]), l[1]),
		URL:      path.Join(Conf.Core.BaseURL, s.id(l[0])),
	}
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

func (s *Screen) Select() (lines Lines, err error) {
	if s.Text == "" {
		err = errors.New("no text to display")
		return
	}
	selectcmd := Conf.Core.SelectCmd
	if selectcmd == "" {
		err = errors.New("no selectcmd specified")
		return
	}

	var buf bytes.Buffer
	err = runFilter(selectcmd, strings.NewReader(s.Text), &buf)
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
		parsedLine := s.parseLine(line)
		lines = append(lines, *parsedLine)
	}

	if len(lines) == 0 {
		err = errors.New("no lines selected")
		return
	}

	return
}

var IDLength int = 9

func shortenID(id string) string {
	return runewidth.Truncate(id, IDLength, "")
}
