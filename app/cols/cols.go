/*
	columnate text
*/
package cols

import (
	"clive/app"
	"clive/app/opt"
	"clive/app/tty"
	"clive/dbg"
	"clive/zx"
	"fmt"
	"strings"
)

type xCmd struct {
	*opt.Flags
	*app.Ctx
	wid, ncols int
	words      []string
	maxwid     int
}

func (x *xCmd) col() {
	if x.wid == 0 {
		x.wid, _ = tty.Cols()
		if x.wid == 0 {
			x.wid = 70
		}
	}
	colwid := x.maxwid + 1
	if x.ncols == 0 {
		x.ncols = x.wid / colwid
	}
	if x.ncols == 0 {
		x.ncols = 1
	}
	fmts := fmt.Sprintf("%%-%ds ", x.maxwid)
	for i, w := range x.words {
		app.Printf(fmts, w)
		if (i+1)%x.ncols == 0 {
			app.Printf("\n")
		}
	}
	if len(x.words)%x.ncols != 0 {
		app.Printf("\n")
	}
}

func (x *xCmd) add(ws ...string) {
	for _, w := range ws {
		if len(w) > x.maxwid {
			x.maxwid = len(w)
		}
	}
	x.words = append(x.words, ws...)
}

// Run cols in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("{file}")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("w", "wid: set max line width", &x.wid)
	x.NewFlag("n", "ncols: set number of columns", &x.ncols)
	args, err := x.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}
	if len(args) != 0 {
		in := app.Files(args...)
		app.SetIO(in, 0)
	}
	in := app.In()
	out := app.Out()
	doselect {
	case s := <-x.Sig:
		app.Dprintf("got sig %s\n", s)
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-in:
		if !ok {
			app.Dprintf("eof\n")
			break
		}
		switch m := m.(type) {
		default:
			// ignored & forwarded
			app.Dprintf("got %T\n", m)
			ok = out <- m
			continue
		case zx.Dir:
			app.Dprintf("got %T %s\n", m, m["upath"])
			x.add(m["name"])
		case error:
			if m != nil {
				err = m
				app.Warn("%s", err)
			}
		case []byte:
			app.Dprintf("got %T [%d]\n", m, len(m))
			ok = out <- m
			words := strings.Fields(strings.TrimSpace(string(m)))
			x.add(words...)
		}
	}
	x.col()
	app.Exits(cerror(in))
}
