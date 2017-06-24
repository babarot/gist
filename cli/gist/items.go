package gist

import (
	"bytes"
	"fmt"
	"path"
	"path/filepath"
	tt "text/template"

	"github.com/b4b4r07/gist/api"
	runewidth "github.com/mattn/go-runewidth"
)

func convertItem(data api.Item) Item {
	var files Files
	for _, file := range data.Files {
		files = append(files, File{
			Filename: file.Filename,
			Content:  file.Content,
			// original field
			Path: filepath.Join(data.ID, file.Filename),
		})
	}
	return Item{
		ID:          data.ID,
		ShortID:     data.ShortID,
		Description: data.Description,
		Public:      data.Public,
		Files:       files,
		// original field
		URL: path.Join(BaseURL, data.ID),
	}
}

func convertItems(data api.Items) Items {
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

func ShortenID(id string) string {
	return runewidth.Truncate(id, api.IDLength, "")
}
