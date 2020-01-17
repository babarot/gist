package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/b4b4r07/gist/pkg/gist"
	"github.com/caarlos0/spin"
	"github.com/dustin/go-humanize"
	"github.com/manifoldco/promptui"
	"golang.org/x/crypto/ssh/terminal"
)

type meta struct {
	gist  gist.Gist
	Files []gist.File

	cache *gist.Cache
}

func (m *meta) init(args []string) error {
	workDir := filepath.Join(os.Getenv("HOME"), ".gist")
	cache := gist.NewCache(filepath.Join(workDir, "cache.json"))

	token := os.Getenv("GITHUB_TOKEN")
	user := os.Getenv("USER")

	// load cache
	cache.Open()

	var pages []gist.Page
	switch len(cache.Pages) {
	case 0:
		s := spin.New("%s Fetching pages...")
		s.Start()
		client := gist.NewClient(token)
		results, err := client.List(user)
		s.Stop()
		if err != nil {
			panic(err)
		}
		pages = results
	default:
		pages = cache.Pages
	}
	cache.Save(pages)

	gist := gist.Gist{
		User:    user,
		WorkDir: workDir,
		Pages:   pages,
	}

	s := spin.New("%s Checking pages...")
	s.Start()
	gist.Update()
	s.Stop()

	m.cache = cache
	m.gist = gist
	m.Files = gist.Files()
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
			content += "  ...\n"
			break
		}
		content += "  " + wrap(lines[i]) + "\n"
	}
	return content
}

func (m *meta) prompt() (gist.File, error) {
	funcMap := promptui.FuncMap
	funcMap["head"] = head
	funcMap["time"] = humanize.Time
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   promptui.IconSelect + " {{ .Name | cyan }}",
		Inactive: "  {{ .Name | faint }}",
		Selected: promptui.IconGood + " {{ .Name }}",
		Details: `
{{ "ID:" | faint }}	{{ .Gist.ID }}
{{ "Description:" | faint }}	{{ .Gist.Description }}
{{ "Private:" | faint }}	{{ not .Gist.Public }}
{{ "Last modified:" | faint }}	{{ .Gist.UpdatedAt | time }}
{{ "Content:" | faint }}	{{ .Content | head }}
		`,
		FuncMap: funcMap,
	}

	searcher := func(input string, index int) bool {
		file := m.Files[index]
		name := strings.Replace(strings.ToLower(file.Name), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)
		return strings.Contains(name, input)
	}

	prompt := promptui.Select{
		Label:             "Select a page",
		Items:             m.Files,
		Templates:         templates,
		Searcher:          searcher,
		StartInSearchMode: true,
		HideSelected:      true,
	}
	i, _, err := prompt.Run()
	return m.Files[i], err
}
