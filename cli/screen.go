package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/b4b4r07/gist/api"
	"github.com/b4b4r07/gist/util"
	runewidth "github.com/mattn/go-runewidth"
	"golang.org/x/crypto/ssh/terminal"
)

var IDLength int = api.IDLength

type Screen struct {
	Gist  *api.Gist
	Text  string
	Lines []string
}

func NewScreen() (s *Screen, err error) {
	spn := util.NewSpinner("Fetching...")
	spn.Start()
	defer spn.Stop()

	gist, err := NewGist()
	if err != nil {
		return s, err
	}

	cache := filepath.Join(Conf.Gist.Dir, "cache.json")
	var files api.Files
	if Conf.Gist.UseCache {
		if !util.Exists(cache) {
			files, err := filesFromAPI(gist)
			if err != nil {
				return s, err
			}
			err = makeCache(files)
		}
		files, err = filesFromCache(cache)
		if err != nil {
			return s, err
		}
	} else {
		// sync files in background
		syncFiles(gist)
		files, err = filesFromAPI(gist)
		if err != nil {
			return s, err
		}
		err = makeCache(files)
		if err != nil {
			return s, err
		}
	}

	lines := renderLines(files)
	return &Screen{
		Gist:  gist,
		Text:  strings.Join(lines, "\n"),
		Lines: lines,
	}, nil
}

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
		cache := filepath.Join(Conf.Gist.Dir, "cache.json")
		if Conf.Gist.UseCache && util.Exists(cache) {
			files, err := filesFromCache(cache)
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
		parsedLine, err := s.parseLine(line)
		if err != nil {
			// TODO: log
			continue
		}
		lines = append(lines, *parsedLine)
	}

	if len(lines) == 0 {
		err = errors.New("no lines selected")
		return
	}

	return
}

func getSize() (int, error) {
	w, _, err := terminal.GetSize(int(os.Stdout.Fd()))
	return w, err
}

func renderLines(files api.Files) (lines []string) {
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

func filesFromAPI(gist *api.Gist) (files api.Files, err error) {
	if Conf.Flag.OpenStarredItems {
		err = gist.ListStarred()
	} else {
		err = gist.List()
	}
	if err != nil {
		return
	}

	for _, item := range gist.Items {
		if err := gist.Clone(item); err != nil {
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

func makeCache(files api.Files) error {
	cache := filepath.Join(Conf.Gist.Dir, "cache.json")
	f, err := os.Create(cache)
	if err != nil {
		return err
	}
	return json.NewEncoder(f).Encode(&files)
}

func filesFromCache(cache string) (files api.Files, err error) {
	f, err := os.Open(cache)
	if err != nil {
		return
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(&files)
	if err != nil {
		return
	}
	return
}
