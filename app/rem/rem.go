/*
	remove files command
*/
package rem

import (
	"clive/app"
	"clive/app/opt"
	"clive/dbg"
	"clive/zx"
)

type xCmd struct {
	*opt.Flags
	*app.Ctx

	aflag, fflag, dry, verb bool
	dirs                    []zx.Dir
	vprintf                 app.PrintFunc
}

var astr = map[bool]string{true: "-r "}

func (x *xCmd) rem(d zx.Dir) error {
	_, trs, spaths, err := app.ResolveTree(d["path"])
	if err != nil {
		return err
	}
	p := spaths[0]
	if p == "" || p == "/" {
		app.Fatal("won't remove / in server for '%s'", d["upath"])
	}
	x.vprintf("rem %s%s\n", astr[x.aflag], d["upath"])
	if x.dry {
		return nil
	}
	if x.aflag {
		return <-trs[0].RemoveAll(p)
	}
	return <-trs[0].Remove(p)
}

// Run rem in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("{file}")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("a", "remove all", &x.aflag)
	x.NewFlag("f", "quiet, called 'force' in unix", &x.fflag)
	x.NewFlag("n", "dry run; report removes but do not do them", &x.dry)
	x.NewFlag("v", "verbose; print the calls made in the order they are made.", &x.verb)
	x.vprintf = app.FlagEprintf(&x.verb)
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
	x.verb = x.verb || x.dry
	in := app.In()
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
			app.Dprintf("got %T %s\n", d, d["upath"])
			x.dirs = append(x.dirs, d)
		default:
			// ignored
			app.Dprintf("got %T\n", m)
		}
	}
	for i := len(x.dirs) - 1; i >= 0; i-- {
		if cerr := x.rem(x.dirs[i]); cerr != nil {
			if !x.fflag {
				app.Warn("%s", cerr)
				err = cerr
			}
		}
	}
	if err == nil {
		err = cerror(in)
	}
	app.Exits(err)
}
