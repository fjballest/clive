/*
	Ql builtin and external command.
	count lines, words, runes, and bytes.
*/
package cnt

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/dbg"
	"clive/nchan"
	"clive/zx"
	"errors"
	"unicode"
)

type cnt struct {
	name                       string
	lines, words, runes, bytes int64
}

type xCmd struct {
	*cmd.Ctx
	*opt.Flags
	lflag, wflag, rflag, bflag, nflag bool

	tots    []*cnt
	debug   bool
	dprintf dbg.PrintFunc
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
	if x.lflag {
		x.Printf("%8d%s\n", c.lines, s)
		return
	}
	if x.wflag {
		x.Printf("%8d%s\n", c.words, s)
		return
	}
	if x.rflag {
		x.Printf("%8d%s\n", c.runes, s)
		return
	}
	if x.bflag {
		x.Printf("%8d%s\n", c.bytes, s)
		return
	}
	x.Printf("%8d %8d %8d %8d%s\n", c.lines, c.words, c.runes, c.bytes, s)
}

func (x *xCmd) RunFile(d zx.Dir, dc <-chan []byte) error {
	name := d["path"]
	if dc == nil {
		return nil
	}
	x.dprintf("cns %s\n", name)
	rc := nchan.Lines(dc, '\n')
	c := &cnt{name: name}
	doselect {
	case <-x.Intrc:
		close(rc, "interrupted")
		return errors.New("interrupted")
	case ln, ok := <-rc:
		if !ok {
			break
		}
		c.lines++
		inword := false
		for _, r := range ln {
			c.runes++
			if !isword(r) && inword {
				inword = false
			} else if isword(r) && !inword {
				c.words++
				inword = true
			}
		}
		c.bytes += int64(len(ln))
	}
	x.tots = append(x.tots, c)
	if !x.nflag {
		x.report(c)
	}
	return cerror(dc)
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("{file}")
	x.Argv0 = argv[0]
	x.dprintf = dbg.FlagPrintf(x.Stderr, &x.debug)
	x.NewFlag("D", "debug", &x.debug)
	x.NewFlag("l", "count just lines", &x.lflag)
	x.NewFlag("w", "count just words", &x.wflag)
	x.NewFlag("r", "count just runes", &x.rflag)
	x.NewFlag("b", "count just bytes", &x.bflag)
	x.NewFlag("c", "count just bytes (characters)", &x.bflag)
	x.NewFlag("n", "print just totals", &x.nflag)
	args, err := x.Parse(argv)
	if err != nil {
		x.Usage(x.Stderr)
		return err
	}
	if cmd.Ns == nil {
		cmd.MkNS()
	}
	x.nflag = x.nflag || len(args) == 0
	err = cmd.RunFiles(x, args...)
	tot := &cnt{name: "total"}
	for i := 0; i < len(x.tots); i++ {
		tot.lines += x.tots[i].lines
		tot.words += x.tots[i].words
		tot.runes += x.tots[i].runes
		tot.bytes += x.tots[i].bytes
	}
	if len(x.tots) > 1 || x.nflag {
		x.report(tot)
	}
	return err
}
