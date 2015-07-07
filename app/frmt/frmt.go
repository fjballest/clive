/*
	format input text
*/
package frmt

import (
	"clive/app"
	"clive/dbg"
	"clive/app/opt"
	"clive/app/wr/frmt"
	"strconv"
	"strings"
)

type xCmd {
	*opt.Flags
	*app.Ctx

	wid, tabwid int
	right bool
}

func tabsOf(s []byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] != '\t' {
			return i
		}
	}
	return 0
}

type par {
	lnc <-chan []rune
	ntabs int
	x interface{}
}

func (x *xCmd) frmt(parc chan par) {
	in := app.Lines(app.In())
	var rawc chan<- string
	var wordc <-chan []rune
	ntabs := 0
	doselect {
	case s := <-x.Sig:
		app.Dprintf("got sig %s\n", s)
		close(rawc, dbg.ErrIntr)
		close(parc, cerror(in))
		return
	case m, ok := <-in:
		if !ok {
			app.Dprintf("got eof\n")
			close(rawc, cerror(in))
			close(parc, cerror(in))
			return
		}
		switch m := m.(type) {
		case []byte:
			app.Dprintf("got %T [%d]\n", m, len(m))
			if t := tabsOf(m); t != ntabs {
				close(rawc)
				rawc = nil
				ntabs = t
			}
			s := string(m)
			if rawc == nil {
				wid := x.wid - ntabs * x.tabwid
				if wid < 5 { wid = 5 }
				rawc, wordc = frmt.Words()
				lnc := frmt.Fmt(wordc, wid, x.right, frmt.OneBlankLine)
				p := par{ntabs: ntabs, lnc: lnc}
				if ok := parc <- p; !ok {
					app.Dprintf("parc1 done\n")
					close(rawc, cerror(parc))
					return
				}
			}
			if ok := rawc <- s; !ok {
				app.Dprintf("rawc done\n")
				return
			}
		default:
			app.Dprintf("got %T\n", m)
			close(rawc)
			rawc = nil
			p := par{x: m}
			if ok := parc <- p; !ok {
				app.Dprintf("parc done\n")
				return
			}
		}
	}
}

// Run the print dirs/files in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("{file}")
	x.NewFlag("D", "debug", &x.Debug)
	x.wid = 80
	x.NewFlag("w", "wid: set max line width (default is 80)", &x.wid)
	x.NewFlag("r", "right justify", &x.right)
	x.NewFlag("t", "tabwid: set tab width (default $tabstop or 8)", &x.tabwid)
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

	ts := app.GetEnv("tabstop")
	if ts != "" {
		x.tabwid, _ = strconv.Atoi(ts)
	}
	if x.tabwid == 0 {
		x.tabwid = 8
	}
	parc := make(chan par)
	go x.frmt(parc)
	out := app.Out()
	for p := range parc {
		if p.x != nil {
			if ok := out <- p.x; !ok {
				err := cerror(out)
				close(p.lnc, err)
				close(parc, err)
				app.Exits(err)
			}
			continue
		}
		if p.lnc == nil {
			break
		}
		pref := strings.Repeat("\t", p.ntabs)
		for ln := range p.lnc {
			oln := []byte(pref + string(ln) + "\n")
			if ok := out <- oln; !ok {
				err := cerror(out)
				close(p.lnc, err)
				close(parc, err)
				app.Exits(err)
			}
		}
	}
	app.Exits(nil)
}

