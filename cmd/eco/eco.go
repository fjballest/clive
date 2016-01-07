/*
	echo command
*/
package main

import (
	"bytes"
	"clive/cmd"
	"clive/cmd/opt"
)

var (
	nflag bool
	opts = opt.New("{arg}")
	ux bool
)

// Run echo in the current app context.
func main() {
	c := cmd.AppCtx()
	cmd.UnixIO("err")
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("n", "don't add a final newline", &nflag)
	opts.NewFlag("u", "use unix out", &ux)
	args, err := opts.Parse()
	if err != nil {
		opts.Usage()
	}
	if ux {
		cmd.UnixIO("out")
	}
	var b bytes.Buffer
	for i, arg := range args {
		b.WriteString(arg)
		if i < len(args)-1 {
			b.WriteString(" ")
		}
	}
	if !nflag {
		b.WriteString("\n")
	}
	out := cmd.IO("out")
	ok := out <- b.Bytes()
	if !ok {
		cmd.Fatal("out: %s", cerror(out))
	}
}
