// recording runs gist against a made-up GitHub, for the recording in the README:
// mise run demo records it. It shows the version in internal/version, and its one
// argument is the CHANGELOG.md the notes of that version are read from.
package main

import (
	"fmt"
	"os"

	"github.com/babarot/gist/v2/demo/internal/recording"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: recording CHANGELOG.md")
		os.Exit(2)
	}
	if err := recording.Run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "recording:", err)
		os.Exit(1)
	}
}
