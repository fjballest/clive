/*
	print dirs/files in input
*/
package pf

import (
	"clive/app"
	"clive/dbg"
	"clive/app/nsutil"
	"clive/app/opt"
	"path"
	"clive/zx"
	"clive/app/gr"
	"errors"
	"bytes"
)

type xCmd {
	*opt.Flags
	*app.Ctx

	lflag, llflag, pflag, vflag, dflag, fflag,
		xflag, sflag, Sflag, aflag, wflag, dryw bool
}

func (x *xCmd) pd(d zx.Dir) error {
	if x.pflag {
		d["path"] = path.Base(d["path"])
	}
	switch {
	case x.vflag:
		return app.Printf("%s\n", d)
	case x.lflag:
		return app.Printf("%s\n", d.Long())
	case x.llflag:
		return app.Printf("%s\n", d.LongLong())
	case x.pflag:
		return app.Printf("%s\n", d["path"])
	default:
		return app.Printf("%s\n", d["upath"])
	}
	return nil
}

func (x *xCmd) pf(in, out chan interface{}) error {
	addrs := ""
	doselect {
	case s := <-x.Sig:
		app.Dprintf("got sig %s\n", s)
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-in:
		if x.aflag {
			if _, ok := m.(gr.Addr); !ok && addrs != "" {
				out <- []byte(addrs)
				addrs = ""
			}
		}
		if !ok {
			app.Dprintf("eof\n")
			break
		}
		switch m := m.(type) {
		case []byte:
			app.Dprintf("got %T [%d]\n", m, len(m))
			if x.dflag {
				continue
			}
			ok = out <- m
		case zx.Dir:
			app.Dprintf("got %T %s\n", m, m["path"])
			if x.fflag {
				continue
			}
			ok = x.pd(m) == nil
		case gr.Addr:
			if x.xflag {
				ok = app.Printf("%s:\n", m) == nil
			} else if x.aflag {
				addrs = m.String() + "\n"
			}
		case string:
			if x.sflag {
				ok = app.Printf("%s", m) == nil
			} else if x.Sflag {
				ok = app.Printf("[%s]", m) == nil
			}
		default:
			// ignored
			app.Dprintf("got %T\n", m)
			ok = out <- m
		}
		if !ok {
			app.Exits(cerror(out))
		}
	}
	return cerror(in)
}

func (x *xCmd) write(bd zx.Dir, b *bytes.Buffer) error {
	var err error
	if bd == nil {
		return err
	}
	if x.vflag || x.dryw {
		app.Printf("wr '%s' type %s %d bytes\n", bd["upath"], bd["type"], b.Len())
	} else {
		app.Dprintf("wr %s:\n[%s]\n", bd["upath"], b)
	}
	if bd["path"] == "" || bd["mode"] == "" {
		app.Warn("no path or no mode in dir: %v", bd)
		err = errors.New("wrong dir in input")
	} else if !x.dryw {
		err = nsutil.PutAll(bd["path"], zx.Dir{"mode": bd["mode"]}, b.Bytes())
		if err != nil {
			app.Warn("%s", err)
		}
	}
	b.Reset()
	return  err
}

// -w requires storing all the data in memory so that the entire source file
// has been processed by the pipe before we write it back to disk.
func (x *xCmd) wf(in, out chan interface{}) error {
	addrs := ""
	var bd zx.Dir
	b := &bytes.Buffer{}
	var sts error
	doselect {
	case s := <-x.Sig:
		app.Dprintf("got sig %s\n", s)
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-in:
		if x.aflag {
			if _, ok := m.(gr.Addr); !ok && addrs != "" {
				out <- []byte(addrs)
				addrs = ""
			}
		}
		if !ok {
			if bd != nil {
				if err := x.write(bd, b); err != nil {
					sts = err
				}
				bd = nil
			}
			app.Dprintf("eof\n")
			break
		}
		switch m := m.(type) {
		case []byte:
			app.Dprintf("got %T [%d]\n", m, len(m))
			b.Write(m)
		case string:
			b.WriteString(m)
		case zx.Dir:
			if bd != nil {
				if err := x.write(bd, b); err != nil {
					sts = err
				}
				bd = nil
			}
			app.Dprintf("got %T %s\n", m, m["path"])
			if m["type"] == "-" {
				bd = m
			}
		case gr.Addr:
			if x.xflag {
				app.Printf("%s:\n", m)
			} else if x.aflag {
				addrs = m.String() + "\n"
			}
		default:
			// ignored
			app.Dprintf("got %T\n", m)
		}
		if !ok {
			app.Exits(cerror(out))
		}
	}
	if sts == nil {
		sts = cerror(in)
	}
	return sts
}

// update ql/builtin.go bltin table if new aliases are added or some are removed.
var aliases = map[string]string {
	"wf": "-w",
}

func (x *xCmd) aliases() {
	if len(x.Args) == 0 {
		return
	}
	if v, ok := aliases[x.Args[0]]; ok {
		// old argv0 + "-aliased flags"  + "all other args"
		nargs := []string{x.Args[0]}
		x.Args[0] = v
		x.Args = append(nargs, x.Args...)
	}
}

// Run the print dirs/files in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("{file}")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("l", "long list dirs", &x.lflag)
	x.NewFlag("L", "longer than long list dirs", &x.llflag)
	x.NewFlag("p", "print abs paths", &x.pflag)
	x.NewFlag("v", "verbose list dirs; or verbose report of files written back", &x.vflag)
	x.NewFlag("d", "do not print file data", &x.dflag)
	x.NewFlag("f", "do not print dir data", &x.fflag)
	x.NewFlag("x", "print all gr addresses", &x.xflag)
	x.NewFlag("a", "print last gr addresses", &x.aflag)
	x.NewFlag("s", "print strings", &x.sflag)
	x.NewFlag("S", "print strings enclosed in [], for debugging", &x.Sflag)
	x.NewFlag("w", "write back input files (eg. after gr -x pipes)", &x.wflag)
	x.NewFlag("n", "dry run for -w, implies -w", &x.dryw)
	x.aliases()
	args, err := x.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}
	x.wflag = x.wflag || x.dryw

	in := app.In()
	out := app.Out()
	if x.wflag {
		err = x.wf(in, out)
	} else {
		err = x.pf(in, out)
	}
	sts := err
	in = app.Files(args...)
	if x.wflag {
		err = x.wf(in, out)
	} else {
		err = x.pf(in, out)
	}
	if err != nil {
		sts = err
	}
	app.Exits(sts)
}
