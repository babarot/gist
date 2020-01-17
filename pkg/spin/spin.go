package spin

import (
	"io/ioutil"

	"github.com/b4b4r07/gist/pkg/logging"
	"github.com/b4b4r07/spin"
)

func New(text string) *spin.Spinner {
	isLogSet := len(string(logging.LogLevel())) > 0
	if isLogSet {
		return spin.New(text, spin.WithWriter(ioutil.Discard))
	}
	return spin.New(text)
}
