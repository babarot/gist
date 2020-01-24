package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/b4b4r07/gist/pkg/gist"
	"github.com/b4b4r07/gist/pkg/spin"
	"github.com/dustin/go-humanize"
	"github.com/manifoldco/promptui"
	"golang.org/x/crypto/ssh/terminal"
)

type meta struct {
	gist  gist.Gist
	files []gist.File

	cache *gist.Cache
}

func (m *meta) init(args []string) error {
	workDir := filepath.Join(os.Getenv("HOME"), ".gist")
	cache := gist.NewCache(filepath.Join(workDir, "cache.json"))
	// load cache
	cache.Open()
	m.cache = cache

	user := os.Getenv("GIST_USER")
	if user == "" {
		user = os.Getenv("USER")
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	token, err := m.githubToken()
	if err != nil {
		return err
	}
	client := gist.NewClient(token)

	var pages []gist.Page
	switch len(cache.Pages) {
	case 0:
		s := spin.New("%s Fetching pages...")
		s.Start()
		results, err := client.List(user)
		s.Stop()
		if err != nil {
			return err
		}
		pages = results
	default:
		pages = cache.Pages
	}

	// update cache
	cache.Save(pages)

	gist := gist.Gist{
		User:    user,
		Token:   token,
		Editor:  editor,
		Client:  client,
		WorkDir: workDir,
		Pages:   pages,
	}

	s := spin.New("%s Checking pages...")
	s.Start()
	defer s.Stop()
	gist.Checkout()

	m.gist = gist
	m.files = gist.Files()
	return nil
}

func (m *meta) UpdateCache(file gist.File) {
	if file.ID == "" {
		return
	}
	var pages []gist.Page
	for _, page := range m.cache.Pages {
		if page.ID == file.ID {
			page.UpdatedAt = time.Now()
		}
		pages = append(pages, page)
	}
	m.cache.Save(pages)
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
{{ "ID:" | faint }}	{{ .Page.ID }}
{{ "Description:" | faint }}	{{ .Page.Description }}
{{ "Private:" | faint }}	{{ not .Page.Public }}
{{ "Last modified:" | faint }}	{{ .Page.UpdatedAt | time }}
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

func (m *meta) githubToken() (string, error) {
	var token string
	token = os.Getenv("GITHUB_TOKEN")
	if token != "" {
		return token, nil
	}
	if m.cache == nil {
		return "", errors.New("cache is nil")
	}
	token = m.cache.Token
	if token != "" {
		return token, nil
	}
	prompt := promptui.Prompt{
		Label: "GITHUB_TOKEN",
		Mask:  '*',
	}
	token, err := prompt.Run()
	if err != nil {
		return "", err
	}
	m.cache.Token = token
	return token, nil
}
