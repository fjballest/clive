package frmt

import (
	"testing"
//	"clive/app/lf"
	"clive/app"
	"clive/dbg"
	"os"
	"clive/zx"
)

var (
	debug bool
	dprintf = dbg.FuncPrintf(os.Stdout, testing.Verbose)

	testpar =`
// When true, Load() reads .bib files containing "bib2ref ok"
// in the first line (with this line being discarded).

// field is described in a single line. 
		%A	Frederick G. Sayward



		%T	User's Guide for the EXPER Mutation Analysis system
		%O	(Yale university, memo)


	// Parsing of the bibtex entries is naive and assumes that each

`
	testout = `
//  When  true, Load() reads .bib files containing
"bib2ref  ok" // in the first line (with this line
being                                  discarded).

// field is described in a single line.
		%A Frederick G. Sayward

		%T  User's  Guide  for  the  EXPER
		Mutation  Analysis system %O (Yale
		university, memo)

	//  Parsing of the bibtex entries is naive
	and assumes that each

`
)


func TestFrmt(t *testing.T) {
	app.New()	// prevent app.Fatal from calling dbg.Fatal
	//app.Debug = testing.Verbose()
	// gf
	pipe := make(chan interface{}, 2)
	pipe <- zx.Dir{"upath": "foo", "path": "foo"}
	pipe <- []byte(testpar)
	close(pipe)
	out := make(chan interface{})
	/*
	go func() {
		c := app.New()
		defer app.Exiting()
		c.Args = []string {"gf", "frmt,"}
		app.DupIO()
		app.SetIO(pipe, 1)
		app.Cd("/zx/sys/src/clive/app")
		lf.Run()
	}()
	*/
	
	// frmt
	go func() {
		c := app.New()
		//c.Debug = testing.Verbose()
		defer app.Exiting()
		c.Args = []string{"frmt", "-w", "50", "-r"}
		app.DupIO()
		app.SetIO(pipe, 0)
		app.SetIO(out, 1)
		app.Cd("/zx/sys/src/clive/app")
		Run()
	}()
	outs := ""
	for x := range out {
		switch x := x.(type) {
		case zx.Dir:
			dprintf("xgot %T %s\n", x, x["upath"])
		case []byte:
			if len(x) > 0 {
				x = x[:len(x)-1]
			}
			dprintf("[%s]\n", x)
			outs += string(x)+"\n"
		case error:
			dprintf("xgot %T %v\n", x, x)
		default:
			dprintf("xgot %T %v\n", x, x)
		}
	}
	dprintf("outs = `%s`\n", outs)
	err := cerror(out)
	dprintf("got sts %v\n", err)
	if outs != testout {
		t.Fatalf("bad output")
	}
}
