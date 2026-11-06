// recording runs gist against a made-up GitHub, for the recording in the README:
// mise run demo records it.
package main

import (
	"fmt"
	"os"

	"github.com/babarot/gist/v2/demo/internal/recording"
)

func main() {
	if err := recording.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "recording:", err)
		os.Exit(1)
	}
}
