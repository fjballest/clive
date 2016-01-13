/*
	format input text
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/cmd/wr/frmt"
	"strconv"
	"strings"
)

var (
	opts = opt.New("{file}")

	wid = 80
	tabwid int
	right       bool
)

func tabsOf(s []byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] != '\t' {
			return i
		}
	}
	return 0
}

type par struct {
	lnc   <-chan []rune
	ntabs int
	x     interface{}
}

func fmt(parc chan par) {
	in := cmd.Lines(cmd.In("in"))
	var rawc chan<- string
	var wordc <-chan []rune
	ntabs := 0
	for m := range in {
		switch m := m.(type) {
		case []byte:
			cmd.Dprintf("got %T [%d]\n", m, len(m))
			if t := tabsOf(m); t != ntabs {
				close(rawc)
				rawc = nil
				ntabs = t
			}
			s := string(m)
			if rawc == nil {
				wid := wid - ntabs*tabwid
				if wid < 5 {
					wid = 5
				}
				rawc, wordc = frmt.Words()
				lnc := frmt.Fmt(wordc, wid, right, frmt.OneBlankLine)
				p := par{ntabs: ntabs, lnc: lnc}
				if ok := parc <- p; !ok {
					cmd.Dprintf("parc1 done\n")
					close(lnc, cerror(rawc))
					close(in, cerror(parc))
					continue
				}
			}
			if ok := rawc <- s; !ok {
				cmd.Dprintf("rawc done\n")
				close(in, cerror(rawc))
				close(parc, cerror(rawc))
				continue
			}
		default:
			cmd.Dprintf("got %T\n", m)
			close(rawc)
			rawc = nil
			p := par{x: m}
			if ok := parc <- p; !ok {
				cmd.Dprintf("parc done\n")
				close(in, cerror(parc))
			}
		}
	}
	close(rawc, cerror(in))
	close(parc, cerror(in))
}

// Run the print dirs/files in the current app context.
func main() {
	cmd.UnixIO("err")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("w", "wid: set max line width (default is 80)", &wid)
	opts.NewFlag("r", "right justify", &right)
	opts.NewFlag("t", "tabwid: set tab width (default $tabstop or 8)", &tabwid)
	ux := false
	opts.NewFlag("u", "use unix out", &ux)
	args, err := opts.Parse()
	if err != nil {
		cmd.Warn("%s", err)
		opts.Usage()
	}
	if ux {
		cmd.UnixIO("out")
	}
	if len(args) != 0 {
		cmd.SetIn("in", cmd.Files(args...))
	}

	ts := cmd.GetEnv("tabstop")
	if ts != "" {
		tabwid, _ = strconv.Atoi(ts)
	}
	if tabwid == 0 {
		tabwid = 8
	}
	parc := make(chan par)
	out := cmd.Out("out")
	go fmt(parc)
	for p := range parc {
		if p.x != nil {
			if ok := out <- p.x; !ok {
				err := cerror(out)
				close(p.lnc, err)
				close(parc, err)
				cmd.Exit(err)
			}
		}
		if p.lnc == nil {
			continue
		}
		pref := strings.Repeat("\t", p.ntabs)
		for ln := range p.lnc {
			oln := []byte(pref + string(ln) + "\n")
			if ok := out <- oln; !ok {
				err := cerror(out)
				close(p.lnc, err)
				close(parc, err)
				cmd.Exit(err)
			}
		}
	}
	cmd.Exit(cerror(parc))
}
