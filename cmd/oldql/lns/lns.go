/*
	Ql builtin and external lz command.
	print lines of zx files
*/
package lns

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/dbg"
	"clive/nchan"
	"clive/zx"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type addr struct {
	from, to int
}

type xCmd struct {
	*cmd.Ctx
	*opt.Flags
	debug           bool
	ranges          []string
	addrs           []addr
	all             bool
	nflag           bool
	nhd, ntl, nfrom int
	dprintf         dbg.PrintFunc
}

func (x *xCmd) parseRanges() error {
	for _, r := range x.ranges {
		toks := strings.SplitN(r, ",", 2)
		if len(toks) == 1 {
			toks = append(toks, toks[0])
		}
		if len(toks[0]) == 0 {
			toks[0] = "1"
		}
		if len(toks[1]) == 0 {
			toks[1] = "-1"
		}
		from, err := strconv.Atoi(toks[0])
		if err != nil {
			return fmt.Errorf("%s: %s", r, err)
		}
		to, err := strconv.Atoi(toks[1])
		if err != nil {
			return fmt.Errorf("%s: %s", r, err)
		}
		a := addr{from, to}
		x.addrs = append(x.addrs, a)
		if from > 0 && x.nhd < from {
			x.nhd = from
		}
		if to > 0 && x.nhd < to {
			x.nhd = to
		}
		if from < 0 && x.ntl < -from {
			x.ntl = -from
		}
		if to < 0 && x.ntl < -to {
			x.ntl = -to
		}
		if from > 0 && to < 0 {
			x.nfrom = from
		}
		if from < 0 && to > 0 {
			x.nfrom = to
		}
		if from == 1 && to == -1 {
			x.all = true
		}
	}
	return nil
}

func (a addr) match(lno, nln int) bool {
	if nln == 0 && (a.from < 0 || a.to < 0) { // need to wait for nlines
		return false
	}
	if a.from > 0 && lno < a.from {
		return false
	}
	if a.to > 0 && lno > a.to {
		return false
	}
	neglno := nln - lno + 1
	if a.from < 0 && neglno > -a.from {
		return false
	}
	if a.to < 0 && neglno < -a.to {
		return false
	}
	return true
}

// addresses a, -b require storing all lines starting at +a
// matching requires waiting until all lines have been read
// addresses -b, -c require storing the last -b lines
// matching requires waiting until all lines have been read
// addresses a, b don't require any store.
func (x *xCmd) RunFile(d zx.Dir, dc <-chan []byte) error {
	if dc == nil {
		return nil
	}
	rc := nchan.Lines(dc, '\n')
	last := []string{}
	nln := 0
	x.dprintf("nhd %d ntl %d nfrom %d\n", x.nhd, x.ntl, x.nfrom)
	doselect {
	case <-x.Intrc:
		close(rc, "interrupted")
		return errors.New("interrupted")
	case s, ok := <-rc:
		if !ok {
			break
		}
		lout := false
		nln++
		if x.all {
			if x.nflag {
				x.Printf("%-5d %s", nln, s)
			} else {
				x.Printf("%s", s)
			}
			continue
		}
		if x.ntl == 0 && x.nfrom == 0 && x.nhd > 0 && nln > x.nhd {
			close(rc, "done")
			return nil
		}
		for _, a := range x.addrs {
			x.dprintf("tl match %d of ? in %d,%d\n", nln, a.from, a.to)
			if a.match(nln, 0) {
				lout = true
				if x.nflag {
					x.Printf("%-5d %s", nln, s)
				} else {
					x.Printf("%s", s)
				}
				break
			}
		}
		if nln >= x.nfrom || x.ntl > 0 {
			if lout {
				s = "" /*already there */
			}
			if nln >= x.nfrom || x.ntl > 0 && len(last) < x.ntl {
				last = append(last, s)
			} else {
				copy(last, last[1:])
				last[len(last)-1] = s
			}
		}

	}
	if !x.all && (x.ntl > 0 || x.nfrom > 0) {
		// if len(last) == 3 and nln is 10
		// last[0] is -3 or 10-2
		// last[1] is -2 or 10-1
		// last[2] is -1 or 10
		for i := 0; i < len(last); i++ {
			for _, a := range x.addrs {
				if a.from > 0 && a.to > 0 { // done already
					continue
				}
				x.dprintf("tl match %d of %d in %d,%d\n",
					nln-len(last)+1+i, nln, a.from, a.to)
				if a.match(nln-len(last)+1+i, nln) && last[i] != "" {
					if x.nflag {
						x.Printf("%-5d %s", nln-len(last)+1+i, last[i])
					} else {
						x.Printf("%s", last[i])
					}
					last[i] = "" /* I can do this because if empty it still contains \n */
					break
				}
			}
		}
	}
	if err := cerror(rc); err != nil {
		return err
	}
	return nil
}

func (x *xCmd) usage() {
	x.Usage(x.Stderr)
	x.Eprintf("\trange is addr,addr or addr\n")
	x.Eprintf("\taddr is linenb, or +linenb, or -linenb\n")
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("{file}")
	x.Argv0 = argv[0]
	x.dprintf = dbg.FlagPrintf(x.Stderr, &x.debug)
	x.NewFlag("D", "debug", &x.debug)
	x.NewFlag("r", "range: print this range", &x.ranges)
	x.NewFlag("n", "print line numbers", &x.nflag)
	args, err := x.Parse(argv)
	if err != nil {
		x.usage()
		return err
	}
	if len(x.ranges) == 0 {
		x.ranges = append(x.ranges, ",")
	}
	if err := x.parseRanges(); err != nil {
		return err
	}
	if cmd.Ns == nil {
		cmd.MkNS()
	}
	return cmd.RunFiles(x, args...)
}
