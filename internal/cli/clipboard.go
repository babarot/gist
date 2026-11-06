package cli

import (
	"encoding/base64"
	"errors"
	"fmt"
)

// copyURL has the terminal put s on the clipboard, with OSC 52. That goes
// through stderr so stdout stays the data.
func (c *CLI) copyURL(s string) error {
	if !c.StdoutTTY {
		return errors.New("not copied: there is no terminal to ask")
	}
	_, err := fmt.Fprintf(c.Stderr, "\x1b]52;c;%s\x07", base64.StdEncoding.EncodeToString([]byte(s)))
	return err
}
