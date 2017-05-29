package util

import (
	"os"
	"time"

	"github.com/briandowns/spinner"
)

var (
	SpinnerSymbol int = 14
)

type Spinner struct {
	*spinner.Spinner

	Text string
}

func NewSpinner(text string) *Spinner {
	return &Spinner{
		Spinner: spinner.New(spinner.CharSets[SpinnerSymbol], 100*time.Millisecond),
		Text:    text,
	}
}

func (s *Spinner) Start() {
	s.Spinner.Writer = os.Stderr
	if len(s.Text) > 0 {
		s.Suffix = " " + s.Text
	}
	s.Spinner.Start()
}

func (s *Spinner) Stop() {
	s.Spinner.Stop()
}
