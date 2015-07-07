/*
	pwd command
*/
package pwd

import (
	"clive/app"
	"clive/app/opt"
)

type xCmd {
	*opt.Flags
	*app.Ctx
	nflag bool
}

// Run pwd in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("")
	args, err := x.Parse(x.Args)
	if err != nil || len(args) != 0 {
		x.Usage()
		app.Exits("usage")
	}
	err = app.Printf("%s\n", app.Dot())
	app.Exits(err)
}
