/*
	copy dirs/files in input into a destination dir or name

*/
package cpm

import (
	"clive/app"
	"clive/app/nsutil"
	"clive/app/opt"
	"clive/dbg"
	"clive/zx"
	"errors"
	"fmt"
	"strings"
)

type xCmd struct {
	*opt.Flags
	*app.Ctx

	verb, aflag, cflag, nflag, mflag, nodst bool
	vprintf                                 app.PrintFunc
}

func (x *xCmd) setdst(dst string) zx.Dir {
	if !strings.ContainsRune(dst, ',') {
		return zx.Dir{"path": app.AbsPath(dst), "upath": dst}
	}
	dc := app.Dirs(dst)
	var r zx.Dir
	doselect {
	case s := <-x.Sig:
		app.Dprintf("got sig %s\n", s)
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-dc:
		if m == nil || !ok {
			if !ok {
				if err := cerror(dc); err != nil {
					app.Fatal(err)
				}
			}
			break
		}
		switch m := m.(type) {
		case error:
			close(dc, m)
			app.Fatal("%s", m.Error())
		case zx.Dir:
			app.Dprintf("got %T %s\n", m, m["upath"])
			if m["err"] != "" {
				if m["err"] != "pruned" {
					app.Fatal(m["err"])
				}
				continue
			}
			if r != nil {
				close(dc, "too many")
				app.Fatal("too many destinations")
			}
			r = m
		}
	}
	if r == nil {
		r = zx.Dir{"path": app.AbsPath(dst), "upath": dst}
	}
	return r
}

func (x *xCmd) paths(dst, m zx.Dir) (dp string, up string, err error) {
	switch {
	case x.mflag && x.cflag:
		dp = zx.Path(dst["path"], m["name"])
		up = zx.Path(dst["upath"], m["name"])
	case x.mflag:
		if m["rpath"] == "/" && m["type"] != "d" {
			dp = zx.Path(dst["path"], m["name"])
			up = zx.Path(dst["upath"], m["name"])
		} else {
			dp = zx.Path(dst["path"], m["rpath"])
			up = zx.Path(dst["upath"], m["rpath"])
		}
	case dst == nil:
		up = m["name"]
		dp = app.AbsPath(up)
	default:
		dp = dst["path"]
		up = dst["upath"]
	}
	if dp == m["path"] {
		return dp, up, fmt.Errorf("%s: same file", dp)
	}
	return dp, up, nil
}

// Run copy dirs/files in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("[dst]")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("v", "report operations made", &x.verb)
	x.NewFlag("c", "collect all files into the destination dir ", &x.cflag)
	x.NewFlag("a", "preserve attributes", &x.aflag)
	x.NewFlag("n", "dry run", &x.nflag)
	x.vprintf = app.FlagEprintf(&x.verb)
	args, err := x.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}
	x.mflag = x.Args[0] != "cpf"
	x.verb = x.verb || x.nflag
	if len(args) == 0 {
		if x.mflag {
			args = append(args, ".")
		}
	} else if len(args) != 1 {
		x.Usage()
		app.Exits("usage")
	}
	dst := x.setdst(args[0])
	in := app.In()
	var putc chan []byte
	var putrc chan zx.Dir
	var sts error

	first := true
	doselect {
	case s := <-x.Sig:
		app.Dprintf("got sig %s\n", s)
		close(putc, dbg.ErrIntr)
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-in:
		if !ok {
			app.Dprintf("eof\n")
			break
		}
		switch m := m.(type) {
		case zx.Dir:
			app.Dprintf("got %T %s\n", m, m["path"])
			close(putc)
			if putrc != nil {
				<-putrc
				if err := cerror(putrc); err != nil {
					sts = err
					app.Warn("%s", err)
				}
			}
			if !first && !x.mflag {
				app.Fatal("multiple files in input")
			}
			first = false
			if m["err"] != "" {
				sts = errors.New(m["err"])
				app.Warn("%s", m["err"])
				continue
			}
			if m["rpath"] == "" {
				app.Warn("no rpath in %s", m["upath"])
				continue
			}
			if x.mflag && m["rpath"] == "/" && m["type"] == "d" {
				// must be a no-op
				continue
			}
			dp, up, err := x.paths(dst, m)
			if err != nil {
				app.Warn("%s", err)
				sts = err
				continue
			}
			x.vprintf("cpf %s %s\n", m["upath"], up)
			if x.nflag {
				continue
			}
			dd := zx.Dir{"mode": m["mode"]}
			if x.aflag {
				for k, v := range m.UsrAttrs() {
					if k != "size" {
						dd[k] = v
					}
				}
			}
			if m["type"] == "d" {
				if err := nsutil.Mkdir(dp, dd); err != nil {
					app.Warn("%s", err)
					sts = err
				}
				continue
			}
			putc = make(chan []byte)
			putrc = nsutil.Put(dp, dd, 0, putc, "")
		case []byte:
			if putc == nil {
				continue // errors in output, perhaps
			}
			if ok := putc <- m; !ok {
				sts = cerror(putc)
				app.Warn("%s", sts)
				putc = nil
				putrc = nil
			}
		default:
			// ignored
			app.Dprintf("got %T\n", m)
		}
	}
	if putc != nil {
		close(putc)
		if putrc != nil {
			<-putrc
			if err := cerror(putrc); err != nil {
				app.Warn("%s", err)
				sts = err
			}
		}
	}
	if sts != nil {
		app.Exits(sts)
	}
	app.Exits(cerror(in))
}
