package wr

import (
	"testing"
	"clive/app"
)

func TestPars(t *testing.T) {
	c := app.New()
	defer app.Exiting()
	c.Debug = testing.Verbose()
	in := app.Files("example")
	app.SetIO(in, 0)
//	c.Args = []string{"wr.test", "-Sp", "-o", "example.tex"}
	c.Args = []string{"wr.test", "-o", "example.html", "-c1"}
//	c.Args = []string{"wr.test", "-ho", "example.html"}
	Run()
}
