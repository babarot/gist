package screen

import (
	"bytes"
	"errors"
	"strings"

	"github.com/b4b4r07/gist/api"
	"github.com/b4b4r07/gist/cli"
	"github.com/b4b4r07/gist/cli/config"
	"github.com/b4b4r07/gist/cli/gist"
)

var IDLength = api.IDLength

type (
	Screen struct {
		Items gist.Items
		Lines []string
	}
	Row struct {
		ID          string
		ShortID     string
		Description string
		Public      bool
		URL         string
		File        gist.File
	}
)

func New() (s *Screen, err error) {
	gist.Dir = config.Conf.Gist.Dir
	client, err := gist.NewClient(config.Conf.Gist.Token)
	if err != nil {
		return
	}
	var items gist.Items
	cache := cli.NewCache()
	if cache.Available() && !config.Conf.Flag.StarredItems {
		items, err = cache.Load()
		if err != nil {
			return s, err
		}
	} else {
		items, err = client.List()
		if err != nil {
			return s, err
		}
		cache.Cache(items)
	}
	s = &Screen{}
	s.Items = items
	s.Lines = items.Render(config.Conf.Screen.Columns)
	return
}

func (s *Screen) Select() (rows []Row, err error) {
	if len(s.Lines) == 0 {
		err = errors.New("no text to display")
		return
	}
	selectcmd := config.Conf.Core.SelectCmd
	if selectcmd == "" {
		err = errors.New("no selectcmd specified")
		return
	}

	text := strings.NewReader(strings.Join(s.Lines, "\n"))
	var buf bytes.Buffer
	err = cli.Filter(selectcmd, text, &buf)
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
		row, err := s.parse(line)
		if err != nil {
			continue
		}
		rows = append(rows, row)
	}

	return
}

func containsIndex(s string) int {
	for i, v := range config.Conf.Screen.Columns {
		if strings.Contains(v, s) {
			return i
		}
	}
	return -1
}

func (s *Screen) parse(line string) (row Row, err error) {
	l := strings.Split(line, "\t")
	var (
		id  = containsIndex("{{.ID}}")
		sid = containsIndex("{{.ShortID}}")
	)
	for _, item := range s.Items {
		row = Row{
			ID:          item.ID,
			ShortID:     item.ShortID,
			Description: item.Description,
			Public:      item.Public,
			URL:         item.URL,
			File:        item.Files[0],
		}
		// Strictly do not compare
		if id >= 0 && len(item.ID) >= IDLength && strings.Contains(l[id], item.ID) {
			return
		}
		if sid >= 0 && len(item.ShortID) >= IDLength && strings.Contains(l[sid], item.ShortID) {
			return
		}
	}
	return Row{}, errors.New("failed to parse selected line")
}
