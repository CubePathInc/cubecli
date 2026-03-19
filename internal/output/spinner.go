package output

import (
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
)

type Spinner struct {
	s *spinner.Spinner
}

func NewSpinner(msg string) *Spinner {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " " + msg
	s.Color("green")
	return &Spinner{s: s}
}

func (sp *Spinner) Start() {
	sp.s.Start()
}

func (sp *Spinner) Stop() {
	sp.s.Stop()
}

func (sp *Spinner) StopWithSuccess(msg string) {
	sp.s.Stop()
	color.New(color.FgGreen).Printf("✓ %s\n", msg)
}

func (sp *Spinner) StopWithError(msg string) {
	sp.s.Stop()
	color.New(color.FgRed).Printf("✗ %s\n", msg)
}
