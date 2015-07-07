/*
	Ql builtin and external flds command.
	print fields of zx files
*/
package flds

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

type addr  {
	from, to int
}

type xCmd  {
	*cmd.Ctx
	*opt.Flags
	debug   bool
	ranges  []string
	one     bool
	seps    string
	osep    string
	addrs   []addr
	all     bool
	dprintf dbg.PrintFunc
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
		if from==1 && to==-1 {
			x.all = true
		}
	}
	return nil
}

func (a addr) match(fno, nfld int) bool {
	if a.from>0 && fno<a.from {
		return false
	}
	if a.to>0 && fno>a.to {
		return false
	}
	negfno := nfld - fno + 1
	if a.from<0 && negfno>-a.from {
		return false
	}
	if a.to<0 && negfno< -a.to {
		return false
	}
	return true
}

func (x *xCmd) RunFile(d zx.Dir, dc <-chan []byte) error {
	if dc == nil {
		return nil
	}
	rc := nchan.Lines(dc, '\n')
	osep := "\t"
	if x.osep != "" {
		osep = x.osep
	} else {
		if x.seps == "" {
			osep = "\t"
		} else {
			osep = x.seps
		}
		if !x.one {
			osep = osep[:1]
		}
	}
	doselect {
	case <-x.Intrc:
		close(rc, "interrupted")
		return errors.New("interrupted")
	case s, ok := <-rc:
		if !ok {
			break
		}
		if len(s)>0 && s[len(s)-1]=='\n' {
			s = s[:len(s)-1]
		}
		var fields []string
		if x.one {
			if x.seps == "" {
				x.seps = "\t"
			}
			fields = strings.Split(s, x.seps)
		} else {
			if x.seps == "" {
				fields = strings.Fields(s)
			} else {
				fields = strings.FieldsFunc(s, func(r rune) bool {
					return strings.ContainsRune(x.seps, r)
				})
			}
		}
		if x.all {
			x.Printf("%s\n", strings.Join(fields, osep))
			continue
		}
		sep := ""
		for _, a := range x.addrs {
			for i, fld := range fields {
				nfld := i + 1
				x.dprintf("tl match %d of %d in %d,%d\n", nfld, len(fields), a.from, a.to)
				if a.match(nfld, len(fields)) {
					x.Printf("%s%s", sep, fld)
					sep = osep
				}
			}
		}
		x.Printf("\n")
	}
	if err := cerror(rc); err != nil {
		return err
	}
	return nil
}

func (x *xCmd) usage() {
	x.Usage(x.Stderr)
	x.Eprintf("\trange is addr,addr or addr\n")
	x.Eprintf("\taddr is fieldnb, or +fieldnb, or -fieldnb\n")
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("{file}")
	x.Argv0 = argv[0]
	x.dprintf = dbg.FlagPrintf(x.Stderr, &x.debug)
	x.NewFlag("D", "debug", &x.debug)
	x.NewFlag("r", "range: print this field range", &x.ranges)
	x.NewFlag("F", "sep: input field delimiter character(s) (or string under -1)", &x.seps)
	x.NewFlag("o", "sep: output field delimiter string", &x.osep)
	x.NewFlag("1", "fields separated by 1 run of the field delimiter string", &x.one)
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
