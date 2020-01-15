package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/b4b4r07/gist/pkg/gist"
	"github.com/manifoldco/promptui"
	"golang.org/x/crypto/ssh/terminal"
)

type meta struct {
	files []gist.File

	gist gist.Gist
}

func (m *meta) init(args []string) error {
	user := os.Getenv("USER")
	base := filepath.Join(os.Getenv("HOME"), ".gist")
	m.gist = gist.New(user, base)

	files, err := m.gist.List()
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
	wrap := func(line string) string {
		line = strings.ReplaceAll(line, "\t", "  ")
		id := int(os.Stdout.Fd())
		width, _, _ := terminal.GetSize(id)
		if width < 10 {
			return line
		}
		if len(line) < width-10 {
			return line
		}
		return line[:width-10] + "..."
	}
	lines := strings.Split(content, "\n")
	content = "\n"
	for i := 0; i < len(lines); i++ {
		if i > 4 {
			break
		}
		content += "  " + wrap(lines[i]) + "\n"
	}
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
