package cli

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	colon "github.com/b4b4r07/go-colon"
	"github.com/kballard/go-shellquote"
)

func expandPath(s string) string {
	if len(s) >= 2 && s[0] == '~' && os.IsPathSeparator(s[1]) {
		if runtime.GOOS == "windows" {
			s = filepath.Join(os.Getenv("USERPROFILE"), s[2:])
		} else {
			s = filepath.Join(os.Getenv("HOME"), s[2:])
		}
	}
	return os.Expand(s, os.Getenv)
}

func runFilter(command string, r io.Reader, w io.Writer) error {
	command = expandPath(command)
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

func escape(command string, args []string) string {
	for _, arg := range args {
		command = shellquote.Join(command, arg)
	}
	return command
}

func Run(command string, args ...string) error {
	if command == "" {
		return errors.New("command not found")
	}
	command = escape(command, args)
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

func Runnable(file string) error {
	fi, err := os.Stat(file)
	if err != nil {
		return err
	}
	var (
		origPerm = fi.Mode().Perm()
		execPerm = os.FileMode(0755).Perm()
	)
	if origPerm != execPerm {
		os.Chmod(file, execPerm)
		defer os.Chmod(file, origPerm)
	}
	return nil
}
