package util

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/b4b4r07/gist/config"
	"github.com/b4b4r07/go-colon"
	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/pkg/browser"
)

const LengthID = 9

func Open(target string) error {
	_, err := url.ParseRequestURI(target)
	if err != nil {
		return err
	}
	return browser.OpenURL(target)
}

func Underline(message, target string) {
	if message == "" || target == "" {
		return
	}
	link := color.New(color.Underline).SprintFunc()
	fmt.Printf("%s %s\n", message, link(target))
}

func GetID(file string) string {
	return filepath.Base(filepath.Dir(file))
}

func FileContent(fname string) string {
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func Exists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func TempFile(filename string) (*os.File, error) {
	return os.Create(filepath.Join(os.TempDir(), filename))
}

func UniqueArray(args []string) []string {
	ret := make([]string, 0, len(args))
	encountered := map[string]bool{}
	for _, arg := range args {
		if !encountered[arg] {
			encountered[arg] = true
			ret = append(ret, arg)
		}
	}
	return ret
}

func Filter(text string) ([]string, error) {
	var (
		selectedLines []string
		buf           bytes.Buffer
		err           error
	)
	if text == "" {
		return selectedLines, errors.New("No input")
	}
	err = runFilter(config.Conf.Core.SelectCmd, strings.NewReader(text), &buf)
	if err != nil {
		return selectedLines, err
	}
	if buf.Len() == 0 {
		return selectedLines, errors.New("no lines selected")
	}
	selectedLines = strings.Split(buf.String(), "\n")
	return selectedLines, nil
}

func runFilter(command string, r io.Reader, w io.Writer) error {
	command = os.Expand(command, os.Getenv)
	result, err := colon.Parse(command)
	if err != nil {
		return err
	}
	first, err := result.Executable().First()
	if err != nil {
		return err
	}
	command = first.Item
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	cmd.Stderr = os.Stderr
	cmd.Stdout = w
	cmd.Stdin = r
	return cmd.Run()
}

func RunCommand(command string, args ...string) error {
	if command == "" {
		return errors.New("command not found")
	}
	command += " " + strings.Join(args, " ")
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func ShortenID(id string) string {
	var ret string
	for pos, str := range strings.Split(id, "") {
		if pos <= LengthID {
			ret += str
		}
	}
	return ret
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
			if len(line) == 0 {
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
