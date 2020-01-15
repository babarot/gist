package cmd

import (
	"errors"
	"strings"

	"github.com/b4b4r07/gist/pkg/gist"
	"github.com/manifoldco/promptui"
)

type meta struct {
	files []gist.File
}

func (m *meta) init(args []string) error {
	user := "b4b4r07"
	files, err := gist.List(user, "/Users/b4b4r07/.gist")
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("unknown error when meta.init")
	}
	m.files = files
	return nil
}

func head(content string) string {
	lines := strings.Split(content, "\n")
	content = "\n"
	content += "  " + lines[0] + "\n"
	content += "  " + lines[1] + "\n"
	content += "  " + lines[2] + "\n"
	content += "  " + lines[3] + "\n"
	content += "  " + lines[4] + "\n"
	return content
}

func (m *meta) prompt() (gist.File, error) {
	funcMap := promptui.FuncMap
	funcMap["head"] = head
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   promptui.IconSelect + " {{ .Name | cyan }}",
		Inactive: "  {{ .Name | faint }}",
		Selected: promptui.IconGood + " {{ .Name }}",
		Details: `
{{ "ID:" | faint }}	{{ .Gist.ID }}
{{ "Description:" | faint }}	{{ .Gist.Description }}
{{ "Public:" | faint }}	{{ .Gist.Public }}
{{ "Content:" | faint }}	{{ .Content | head }}
		`,
		FuncMap: funcMap,
	}

	searcher := func(input string, index int) bool {
		file := m.files[index]
		name := strings.Replace(strings.ToLower(file.Name), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)
		return strings.Contains(name, input)
	}

	prompt := promptui.Select{
		Label:             "Select a page",
		Items:             m.files,
		Templates:         templates,
		Searcher:          searcher,
		StartInSearchMode: true,
		HideSelected:      true,
	}
	i, _, err := prompt.Run()
	return m.files[i], err
}
