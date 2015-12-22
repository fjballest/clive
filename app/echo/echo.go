/*
	echo command
*/
package echo

import (
	"bytes"
	"clive/app"
	"clive/app/opt"
)

type xCmd struct {
	*opt.Flags
	*app.Ctx
	nflag bool
}

// Run echo in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("{arg}")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("g", "don't add a final newline", &x.nflag)
	args, err := x.Parse(x.Args)
	if err != nil {
		x.Usage()
		app.Exits("usage")
	}
	var b bytes.Buffer
	for i, arg := range args {
		b.WriteString(arg)
		if i < len(args)-1 {
			b.WriteString(" ")
		}
	}
	if !x.nflag {
		b.WriteString("\n")
	}
	out := app.Out()
	ok := out <- b.Bytes()
	if !ok {
		app.Warn("stdout: %s", cerror(out))
		app.Exits(cerror(out))
	}
	app.Exits(nil)
}
