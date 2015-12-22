/*
	translate expressions in input
*/
package trex

import (
	"clive/app"
	"clive/app/opt"
	"clive/dbg"
	"clive/sre"
	"strings"
)

type xCmd struct {
	*opt.Flags
	*app.Ctx

	res        []*sre.ReProg
	froms, tos []string
	all        bool

	sflag, fflag, gflag, tflag, lflag, uflag, rflag, xflag bool
}

func replre(s string, re *sre.ReProg, to string, glob bool) string {
	rfrom := []rune(s)
	rto := []rune(to)
	nrefs := 0
	for i := 0; i < len(rto)-1; i++ {
		if nb := rto[i+1]; rto[i] == '\\' {
			if nb >= '0' && nb <= '9' {
				nb -= '0'
				rto[i] = nb
				nrefs++
				n := copy(rto[i+1:], rto[i+2:])
				rto = rto[:i+1+n]
			} else if nb == '\\' {
				n := copy(rto[i+1:], rto[i+2:])
				rto = rto[:i+1+n]
			}
		}
	}
	st := 0
	for {
		app.Dprintf("re match [%d:%d]\n", st, len(rfrom))
		rg := re.ExecRunes(rfrom, st, len(rfrom))
		if len(rg) == 0 {
			break
		}
		r0 := rg[0]
		var ns []rune
		ns = append(ns, rfrom[:r0.P0]...)
		if nrefs == 0 {
			ns = append(ns, rto...)
		} else {
			for _, r := range rto {
				if r > 10 {
					ns = append(ns, r)
					continue
				}
				if r < rune(len(rg)) {
					t := rfrom[rg[r].P0:rg[r].P1]
					ns = append(ns, t...)
				}
			}
		}
		st = len(ns)
		ns = append(ns, rfrom[r0.P1:]...)
		rfrom = ns
		if !glob {
			break
		}
	}
	return string(rfrom)
}

func replset(s string, from, to string) string {
	rfrom := []rune(from)
	rto := []rune(to)
	if len(rfrom) != len(rto) && len(rto) > 0 {
		app.Fatal("incompatible replacement '%s' to '%s'", from, to)
	}
	rs := []rune(s)
Loop:
	for i := 0; i < len(rs); {
		for n := 0; n < len(rfrom); n++ {
			if n < len(rfrom)-2 && rfrom[n+1] == '-' {
				if rs[i] >= rfrom[n] && rs[i] <= rfrom[n+2] {
					if len(rto) > 0 {
						rs[i] = rto[n] + rs[i] - rfrom[n]
					} else {
						n := copy(rs[i:], rs[i+1:])
						rs = rs[:i+n]
						continue Loop
					}
				}
				n += 2
			} else if rs[i] == rfrom[n] {
				if len(rto) > 0 {
					rs[i] = rto[n]
				} else {
					n := copy(rs[i:], rs[i+1:])
					rs = rs[:i+n]
					continue Loop
				}
			}
		}
		i++
	}
	return string(rs)
}

func (x *xCmd) trex(in chan interface{}) error {
	nrepl := 1
	if x.gflag {
		nrepl = -1
	}
	out := app.Out()
	doall := false
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
		case []byte:
			s := string(m)
			app.Dprintf("got '%s'\n", s)
			if x.all {
				doall = true
				continue
			}
			if x.uflag {
				s = strings.ToUpper(s)
			} else if x.lflag {
				s = strings.ToLower(s)
			} else if x.tflag {
				s = strings.ToTitle(s)
			}
			for i := 0; i < len(x.froms); i++ {
				if x.res != nil {
					s = replre(s, x.res[i], x.tos[i], x.gflag)
				} else if x.rflag {
					s = replset(s, x.froms[i], x.tos[i])
				} else {
					s = strings.Replace(s, x.froms[i], x.tos[i], nrepl)
				}
			}
			app.Printf("%s", s)
		default:
			if x.all && doall {
				doall = false
				out <- []byte(x.tos[0])
			}
			// ignored
			app.Dprintf("got %T\n", m)
			ok = out <- m
		}
		if !ok {
			app.Exits(cerror(out))
		}
	}
	if x.all && doall {
		m := []byte(x.tos[0])
		if ok := out <- m; !ok {
			app.Exits(cerror(out))
		}
	}
	return cerror(in)
}

// Run trex in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("to | {from to}")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("s", "handle args as strings and not rexps.", &x.sflag)
	x.NewFlag("g", "change globally (as many times as can be done per line)", &x.gflag)
	x.NewFlag("f", "match expressions in full files", &x.fflag)
	x.NewFlag("u", "translate to upper case", &x.uflag)
	x.NewFlag("l", "translate to lower case", &x.lflag)
	x.NewFlag("t", "translate to title case", &x.tflag)
	x.NewFlag("r", "interpret replacements as rune sets", &x.rflag)
	x.NewFlag("x", "match against each extracted text (eg., out from gr -x)", &x.xflag)
	args, err := x.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}
	if len(args) == 1 {
		args = []string{".*", args[0]}
		x.all = true
	}
	if len(args)%2 != 0 {
		app.Warn("wrong number of arguments")
		x.Usage()
		app.Exits("usage")
	}
	if x.rflag && x.gflag || x.rflag && x.sflag || x.gflag && x.sflag {
		app.Warn("incompatible flags given")
		x.Usage()
		app.Exits("usage")
	}
	for i := 0; i < len(args); i += 2 {
		x.froms = append(x.froms, args[0])
		x.tos = append(x.tos, args[1])
		if !x.sflag && !x.rflag {
			re, err := sre.CompileStr(args[0], sre.Fwd)
			if err != nil {
				app.Fatal("rexp: %s", err)
			}
			x.res = append(x.res, re)
		}
	}
	if x.xflag {
		app.Exits(x.trex(app.In()))
	}
	if x.fflag {
		app.Exits(x.trex(app.FullFiles(app.In())))
	}
	app.Exits(x.trex(app.Lines(app.In())))
}
