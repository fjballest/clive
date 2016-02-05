/*
	Echo command arguments.

	Doubles as a testing tool that
	sends output to a named chan and/or gets input from a named chan.
*/
package main

import (
	"bytes"
	"clive/cmd"
	"clive/cmd/opt"
)

var (
	nflag, mflag bool
	ux bool
	oname = "out"
	iname string
	opts = opt.New("{arg}")
)

// Run echo in the current app context.
func main() {
	cmd.UnixIO("err")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("n", "don't add a final newline", &nflag)
	opts.NewFlag("m", "issue one message per arg", &mflag)
	opts.NewFlag("u", "use unix out", &ux)
	opts.NewFlag("o", "chan: output to this chan (for testing other tools)", &oname)
	opts.NewFlag("i", "chan: echo input from this chan (for testing other tools)", &iname)
	args := opts.Parse()
	if ux {
		cmd.UnixIO(oname)
	}
	var b bytes.Buffer
	out := cmd.Out(oname)
	if out == nil {
		cmd.Fatal("no output chan '%s'", oname)
	}
	mflag = mflag || iname != ""
	for i, arg := range args {
		if mflag {
			ok := out <- []byte(arg)
			if !ok {
				cmd.Fatal("out: %s", cerror(out))
			}
		} else {
			b.WriteString(arg)
			if i < len(args)-1 {
				b.WriteString(" ")
			}
		}
	}
	if iname != "" {
		for x := range cmd.In(iname) {
			x := x
			if b, ok := x.([]byte); ok {
				ok := out <- []byte(b)
				if !ok {
					cmd.Fatal("out: %s", cerror(out))
				}
			}
		}
	}
	if mflag {
		cmd.Exit(nil)
	}
	if !nflag {
		b.WriteString("\n")
	}
	ok := out <- b.Bytes()
	if !ok {
		cmd.Fatal("out: %s", cerror(out))
	}
}
