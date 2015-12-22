/*
	list files command
*/
package lf

import (
	"clive/app"
	"clive/app/opt"
	"clive/dbg"
	"clive/zx"
	"errors"
)

type xCmd struct {
	*opt.Flags
	*app.Ctx
	gflag bool
}

// Run list files in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("{file}")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("g", "get file contents", &x.gflag)
	args, err := x.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}
	if x.Args[0] == "gf" {
		x.gflag = true
	}
	if len(args) == 0 {
		args = append(args, ".,1")
	}
	find := app.Dirs
	if x.gflag {
		find = app.Files
	}
	dc := find(args...)
	var sts error
	out := app.Out()
	doselect {
	case s := <-x.Sig:
		app.Dprintf("got sig %s\n", s)
		close(dc, dbg.ErrIntr)
		app.Fatal(dbg.ErrIntr)
	case m := <-dc:
		if m == nil {
			break
		}
		var ok bool
		switch m := m.(type) {
		case error:
			app.Warn("%s", m.Error())
			sts = m
			continue
		default:
			app.Dprintf("got %T %v\n", m, m)
			ok = out <- m
		case zx.Dir:
			app.Dprintf("got %T %s\n", m, m["upath"])
			if m["err"] != "" {
				if m["err"] != "pruned" {
					sts = errors.New(m["err"])
					app.Warn("%s", m["err"])
					continue
				}
			}
			ok = out <- m
		}
		if !ok {
			err := cerror(out)
			close(dc, err)
			app.Fatal(err)
		}
	}
	if sts == nil {
		sts = cerror(dc)
	}
	app.Exits(sts)
}
