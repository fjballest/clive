/*
	move multiple files command
*/
package mvm

import (
	"clive/app"
	"clive/app/opt"
	"clive/dbg"
	"clive/zx"
	"fmt"
	"strings"
)

type xCmd struct {
	*opt.Flags
	*app.Ctx

	dirs []zx.Dir

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

func (x *xCmd) mvm() error {
	dsts := map[string]bool{}
	var dpref string
	var dtree zx.RWTree
	var dspaths, sspaths []string
	for _, d := range x.dirs {
		dp := d["dp"]
		if dsts[dp] {
			app.Fatal("multiple moves into %s", dp)
		}
		dsts[dp] = true
		pref, trs, paths, err := app.ResolveTree(dp)
		if err != nil {
			app.Fatal(err)
		}
		if dtree != nil && dpref != pref {
			app.Fatal("cross device move at", d["up"])
		}
		p := paths[0]
		if p == "" || p == "/" {
			app.Fatal("won't move to / in server for '%s'", d["up"])
		}
		dpref = pref
		dtree = trs[0]
		dspaths = append(dspaths, paths[0])
		pref, _, paths, err = app.ResolveTree(d["path"])
		if err != nil {
			app.Fatal(err)
		}
		if dpref != pref {
			app.Fatal("cross device move at", d["up"])
		}
		sp := paths[0]
		if sp == "" || sp == "/" {
			app.Fatal("won't move from / in server for '%s'", d["up"])
		}
		sspaths = append(sspaths, sp)
		if zx.HasPrefix(p, sp) {
			app.Fatal("can't move %s into %s", sp, p)
		}
	}
	var sts error
	for i := range dspaths {
		x.vprintf("mvf %s %s\n", sspaths[i], dspaths[i])
		if !x.nflag {
			if err := <-dtree.Move(sspaths[i], dspaths[i]); err != nil {
				app.Warn("%s", err)
				sts = err
			}
		}
	}
	return sts
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

// Run mvm in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("[dst]")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("v", "report operations made", &x.verb)
	x.NewFlag("c", "collect all files into the destination dir ", &x.cflag)
	x.NewFlag("n", "dry run", &x.nflag)
	x.vprintf = app.FlagEprintf(&x.verb)
	args, err := x.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}
	x.mflag = x.Args[0] != "mvf"
	x.verb = x.verb || x.nflag
	if len(args) == 0 {
		if x.mflag {
			args = append(args, ".")
		}
	}
	if len(args) != 1 {
		x.Usage()
		app.Exits("usage")
	}
	dst := x.setdst(args[0])
	in := app.In()
	var sts error
	first := true
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
		case zx.Dir:
			app.Dprintf("got %T %s\n", m, m["path"])
			if !first && !x.mflag {
				app.Fatal("multiple files in input")
			}
			first = false
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
			m["dp"] = dp
			m["up"] = up
			x.dirs = append(x.dirs, m)
		default:
			// ignored
			app.Dprintf("got %T\n", m)
		}
	}
	if err := x.mvm(); err != nil {
		sts = err
	} else if sts == nil {
		sts = cerror(in)
	}
	app.Exits(sts)
}
