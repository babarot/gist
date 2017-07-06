package cli

import (
	"log"
	"os"
	"time"

	"github.com/briandowns/spinner"
)

var (
	SpinnerSymbol int = 14
)

type Spinner struct {
	*spinner.Spinner

	text string
}

func NewSpinner(text string) *Spinner {
	return &Spinner{
		Spinner: spinner.New(spinner.CharSets[SpinnerSymbol], 100*time.Millisecond),
		text:    text,
	}
}

func (s *Spinner) Start() {
	s.Spinner.Writer = os.Stderr
	s.Spinner.Prefix = "\r"
	if len(s.text) > 0 {
		s.Suffix = " " + s.text
	}
	s.Spinner.Start()
}

func (s *Spinner) Stop() {
	s.Spinner.Stop()
}

func ErrorLog(err error) {
	log.Printf("%v\n", err)
}
