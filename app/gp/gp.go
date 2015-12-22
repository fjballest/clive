/*
	grep for predicates files in the input
*/
package gp

import (
	"clive/app"
	"clive/app/opt"
	"clive/dbg"
	"clive/zx"
	"clive/zx/pred"
	"errors"
)

type xCmd struct {
	*opt.Flags
	*app.Ctx

	preds []*pred.Pred
}

func (x *xCmd) gp(in, out chan interface{}) error {
	matched := true
	some := false
	doselect {
	case s := <-x.Sig:
		app.Dprintf("got sig %s\n", s)
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-in:
		if !ok {
			app.Dprintf("eof\n")
			break
		}
		switch d := m.(type) {
		case zx.Dir:
			app.Dprintf("got %T %s\n", m, d["path"])
			matched = false
			depth := len(zx.Elems(d["rpath"]))
			if depth > 0 {
				depth--
			}
			var err error
			for _, p := range x.preds {
				matched, _, err = p.EvalAt(d, depth)
				app.Dprintf("%s match=%v\n",
					d.TestFmt(), matched)
				if matched {
					break
				}
			}
			if err != nil {
				app.Warn("%s", err)
			}
			if matched {
				some = true
				ok = out <- d
			}
		default:
			app.Dprintf("got %T\n", d)
			if matched {
				some = true
				ok = out <- d
			}
		}
		if !ok {
			app.Exits(cerror(out))
		}
	}
	if !some {
		return errors.New("no match")
	}
	return cerror(in)
}

// Run gp in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("{pred}")
	x.NewFlag("D", "debug", &x.Debug)
	args, err := x.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}
	if len(args) == 0 {
		app.Warn("missing predicate")
		x.Usage()
		app.Exits("usage")
	}
	for _, a := range args {
		p, err := pred.New(a)
		if err != nil {
			app.Fatal("%s", err)
		}
		x.preds = append(x.preds, p)
	}
	in := app.In()
	out := app.Out()
	err = x.gp(in, out)
	app.Exits(err)
}
