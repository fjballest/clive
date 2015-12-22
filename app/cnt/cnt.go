/*
	cnt command
*/
package cnt

import (
	"clive/app"
	"clive/app/opt"
	"clive/dbg"
	"clive/zx"
	"unicode"
	"unicode/utf8"
)

type cnt struct {
	name string

	msgs, lines, words, runes, bytes int64
}

type xCmd struct {
	*opt.Flags
	*app.Ctx

	lflag, wflag, rflag, bflag, nflag, mflag bool
	tots                                     []*cnt
}

func isword(r rune) bool {
	iscon := unicode.Is(unicode.Pc, r) || unicode.Is(unicode.Pd, r)
	return !unicode.IsControl(r) && !unicode.IsSpace(r) &&
		(!unicode.IsPunct(r) || iscon)
}

func (x *xCmd) report(c *cnt) {
	s := "  " + c.name
	if x.nflag {
		s = ""
	}
	if x.mflag {
		app.Printf("%8d%s\n", c.msgs, s)
		return
	}
	if x.lflag {
		app.Printf("%8d%s\n", c.lines, s)
		return
	}
	if x.wflag {
		app.Printf("%8d%s\n", c.words, s)
		return
	}
	if x.rflag {
		app.Printf("%8d%s\n", c.runes, s)
		return
	}
	if x.bflag {
		app.Printf("%8d%s\n", c.bytes, s)
		return
	}
	app.Printf("%8d %8d %8d %8d %8d%s\n", c.msgs, c.lines, c.words, c.runes, c.bytes, s)
}

func (x *xCmd) add(c *cnt) {
	x.tots = append(x.tots, c)
	if !x.nflag {
		x.report(c)
	}
}

func (x *xCmd) cnt(in chan interface{}) {
	var c *cnt
	var saved []byte
	inword := false
	doselect {
	case <-x.Sig:
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-in:
		if !ok {
			break
		}
		switch m := m.(type) {
		case zx.Dir:
			app.Dprintf("got %T %s\n", m, m["path"])
			inword = false
			saved = nil
			if c != nil {
				x.add(c)
			}
			c = &cnt{name: m["upath"]}
			if c.name == "" {
				c.name = m["path"]
			}
			if m["type"] == "d" {
				c = nil
			}
		case []byte:
			app.Dprintf("got %T\n", m)
			if c == nil {
				c = &cnt{name: "stdin"}
			}
			c.msgs++
			c.bytes += int64(len(m))
			if len(saved) > 0 {
				m = append(saved, m...)
				saved = nil
			}
			for len(m) > 0 {
				if !utf8.FullRune(m) {
					saved = m
					break
				}
				r, n := utf8.DecodeRune(m)
				m = m[n:]
				c.runes++
				if unicode.IsSpace(r) {
					inword = false
				} else if !inword {
					inword = true
					c.words++
				}
				if r == '\n' {
					c.lines++
				}
			}
		default:
			app.Dprintf("got %T\n", m)
			c.msgs++
		}
	}
	if c != nil {
		x.add(c)
	}
}

// Run cnt in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("{file}")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("m", "count just msgs", &x.mflag)
	x.NewFlag("l", "count just lines", &x.lflag)
	x.NewFlag("w", "count just words", &x.wflag)
	x.NewFlag("r", "count just runes", &x.rflag)
	x.NewFlag("b", "count just bytes", &x.bflag)
	x.NewFlag("c", "count just characters", &x.bflag)
	x.NewFlag("n", "print just totals", &x.nflag)
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
	// x.nflag = x.nflag || len(args)==0

	in := app.In()
	x.cnt(in)

	tot := &cnt{name: "total"}
	for i := 0; i < len(x.tots); i++ {
		tot.msgs += x.tots[i].msgs
		tot.lines += x.tots[i].lines
		tot.words += x.tots[i].words
		tot.runes += x.tots[i].runes
		tot.bytes += x.tots[i].bytes
	}
	if len(x.tots) > 1 || x.nflag {
		x.report(tot)
	}
	app.Exits(cerror(in))
}
