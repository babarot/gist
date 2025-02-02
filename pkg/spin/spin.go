package spin

import (
	"github.com/caarlos0/spin"
)

func New(text string) *spin.Spinner {
	// if clilog.IsEnabled() {
	// 	return spin.New(text, spin.WithWriter(ioutil.Discard))
	// }
	return spin.New(text)
}
