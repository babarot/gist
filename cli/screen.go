package cli

import (
	"bytes"
	"errors"
	"strings"

	"github.com/b4b4r07/gist/api"
	"github.com/b4b4r07/gist/cli/gist"
)

var IDLength = api.IDLength

type (
	Screen struct {
		Items gist.Items
		Lines []string
	}
)

func NewScreen() (s *Screen, err error) {
	client, err := gist.NewClient(Conf.Gist.Token)
	if err != nil {
		return
	}
	var items gist.Items
	cache := NewCache()
	if cache.Available() && !Conf.Flag.StarredItems {
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
	s.Lines = items.Render(Conf.Screen.Columns)
	return
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

func (s *Screen) parse(line string) (gist.Item, error) {
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
	return gist.Item{}, errors.New("not found")
}
