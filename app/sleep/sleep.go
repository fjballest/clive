/*
	sleep command
*/
package sleep

import (
	"clive/app"
	"clive/app/opt"
	"time"
	"clive/dbg"
)

type xCmd {
	*opt.Flags
	*app.Ctx
	nflag bool
}

// Run echo in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("ival")
	x.NewFlag("D", "debug", &x.Debug)
	args, err := x.Parse(x.Args)
	if err != nil || len(args) != 1 || len(args[0]) == 0 {
		x.Usage()
		app.Exits("usage")
	}
	s := args[0]
	if last := s[len(s)-1]; last >= '0' && last <= '9' {
		s += "s"
	}
	ival, err := time.ParseDuration(s)
	if err != nil {
		x.Usage()
		app.Exits("usage")
	}
	select {
	case <-time.After(ival):
	case <-x.Sig:
		app.Fatal(dbg.ErrIntr)
	}
	app.Exits(nil)
}
