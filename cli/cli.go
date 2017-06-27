package cli

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/pkg/browser"
)

var (
	ErrConfigEditor = errors.New("config.core.editor is not set")
)

func Open(link string) error {
	_, err := url.ParseRequestURI(link)
	if err != nil {
		return err
	}
	return browser.OpenURL(link)
}

func Underline(message, target string) {
	if message == "" || target == "" {
		return
	}
	link := color.New(color.Underline).SprintFunc()
	fmt.Printf("%s %s\n", message, link(target))
}

func TempFile(filename string) (*os.File, error) {
	return os.Create(filepath.Join(os.TempDir(), filename))
}

var (
	ScanDefaultString string
	ScanAllowEmpty    bool
)

func Scan(message string, allowEmpty bool) (string, error) {
	tmp := "/tmp"
	if runtime.GOOS == "windows" {
		tmp = os.Getenv("TEMP")
	}
	l, err := readline.NewEx(&readline.Config{
		Prompt:            message,
		HistoryFile:       filepath.Join(tmp, "gist.txt"),
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	})
	if err != nil {
		return "", err
	}
	defer l.Close()

	var line string
	for {
		if ScanDefaultString == "" {
			line, err = l.Readline()
		} else {
			line, err = l.ReadlineWithDefault(ScanDefaultString)
		}
		if err == readline.ErrInterrupt {
			if len(line) <= len(ScanDefaultString) {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" && allowEmpty {
			continue
		}
		return line, nil
	}
	return "", errors.New("canceled")
}

func FileContent(file string) (content string, err error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	return string(data), err
}
