// gist edits your GitHub Gists from the terminal.
package main

import (
	"os"

	"github.com/babarot/gist/v2/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
