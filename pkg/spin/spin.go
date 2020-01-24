package spin

import (
	"io/ioutil"

	clilog "github.com/b4b4r07/go-cli-log"
	"github.com/caarlos0/spin"
)

func New(text string) *spin.Spinner {
	if clilog.IsEnabled() {
		return spin.New(text, spin.WithWriter(ioutil.Discard))
	}
	return spin.New(text)
}
