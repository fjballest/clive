/*
	Ql builtin and external srt command.
	sort lines
*/
package srt

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/dbg"
	"clive/nchan"
	"clive/zx"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type sKind int

const (
	sStr  sKind = iota // sort as string
	sNum               // sort as a number (integer or float)
	sTime              // sort as a time
)

type addr  {
	from, to int
	kind     sKind
	rev      bool
	all      bool
}

type xSort  {
	lines   []string
	keys    [][]interface{} // field or line keys to sort
	revs    []bool          // which addr is reverse order?
	dprintf dbg.PrintFunc
}

type xCmd  {
	*cmd.Ctx
	*opt.Flags
	debug     bool
	keys      []string
	xs        *xSort
	one, uniq bool
	seps      string
	addrs     []addr
	dprintf   dbg.PrintFunc
}

func (x *xCmd) parseKeys() error {
	for _, r := range x.keys {
		rev := false
		kind := sStr
		if len(r)>0 && r[len(r)-1]=='r' {
			rev = true
			r = r[:len(r)-1]
		}
		if len(r) > 0 {
			switch r[len(r)-1] {
			case 's':
				r = r[:len(r)-1]
			case 'n':
				kind = sNum
				r = r[:len(r)-1]
			case 't':
				kind = sTime
				r = r[:len(r)-1]
			}
		}
		if r == "" {
			a := addr{0, 0, kind, rev, true}
			x.addrs = append(x.addrs, a)
			continue
		}
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
		a := addr{from, to, kind, rev, false}
		x.addrs = append(x.addrs, a)
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

func (x *xSort) Len() int {
	return len(x.lines)
}

func (x *xSort) Swap(i, j int) {
	x.lines[i], x.lines[j] = x.lines[j], x.lines[i]
	x.keys[i], x.keys[j] = x.keys[j], x.keys[i]
}

func (x *xSort) Less(i, j int) (res bool) {
	ki := x.keys[i]
	kj := x.keys[j]
	defer x.dprintf("\t< %v %v -> %v\t\t%v\n", ki, kj, res, x.revs)
	for n := 0; n < len(ki); n++ {
		rev := x.revs[n]
		switch vi := ki[n].(type) {
		case float64:
			vj := kj[n].(float64)
			if rev {
				if vi > vj {
					return true
				}
				if vi < vj {
					return false
				}
				continue
			}
			if vi < vj {
				return true
			}
			if vi > vj {
				return false
			}
		case string:
			vj := kj[n].(string)
			if rev {
				if vi > vj {
					return true
				}
				if vi < vj {
					return false
				}
				continue
			}
			if vi < vj {
				return true
			}
			if vi > vj {
				return false
			}
		case time.Time:
			vj := kj[n].(time.Time)
			if rev {
				if vi.After(vj) {
					return true
				}
				if vi.Before(vj) {
					return false
				}
				continue
			}
			if vi.Before(vj) {
				return true
			}
			if vi.After(vj) {
				return false
			}
		}
	}
	return false
}

// time formats understood
var tfmts = []string{
	time.ANSIC,
	time.UnixDate,
	time.RFC822,
	time.RFC850,
	time.Kitchen,
	"01/02",
	"01/02/06",
	"01/02/2006",
	"2006/0102",
	"15:04:05",
	"15:04",
	"3pm",
	"3:04pm",
	"01/02 15:04",
	"01/02/06 15:04",
	"01/02/2006 15:04",
	"2006/0102 15:04 ",
}

func (x *xSort) initKey(k sKind, fldnb int, rev bool, all bool, one bool, seps string) {
	x.revs = append(x.revs, rev)
	for i := 0; i < len(x.lines); i++ {
		ln := x.lines[i]
		fld := ln
		if !all {
			var fields []string
			if one {
				fields = strings.Split(ln, seps)
			} else {
				fields = strings.FieldsFunc(ln, func(r rune) bool {
					return strings.ContainsRune(seps, r)
				})
			}
			if fldnb>=1 && fldnb<=len(fields) {
				fld = fields[fldnb-1]
			} else {
				fld = ""
			}
		}
		switch k {
		case sNum:
			nb, err := strconv.ParseFloat(fld, 64)
			if err != nil {
				n, _ := strconv.Atoi(fld)
				nb = float64(n)
			}
			x.keys[i] = append(x.keys[i], nb)
		case sTime:
			var err error
			var t time.Time
			for ti := 0; ti < len(tfmts); ti++ {
				t, err = time.Parse(tfmts[ti], fld)
				if err == nil {
					break
				}
			}
			x.keys[i] = append(x.keys[i], t)
		default:
			x.keys[i] = append(x.keys[i], fld)
		}
	}
}

func (x *xCmd) RunFile(d zx.Dir, dc <-chan []byte) error {
	if dc == nil {
		return nil
	}
	xs := x.xs
	rc := nchan.Lines(dc, '\n')
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
		xs.lines = append(xs.lines, s)
	}
	return cerror(rc)
}

func (x *xCmd) sort() error {
	xs := x.xs
	if x.one {
		if x.seps == "" {
			x.seps = "\t"
		}
	} else {
		if x.seps == "" {
			x.seps = " \t"
		}
	}

	xs.keys = make([][]interface{}, len(xs.lines))
	for _, a := range x.addrs {
		if a.from < 0 {
			a.from = len(xs.lines) - (-a.from) + 1
		}
		if a.to < 0 {
			a.to = len(xs.lines) - (-a.to) + 1
		}
		for i := a.from; i <= a.to; i++ {
			xs.initKey(a.kind, i, a.rev, a.all, x.one, x.seps)
		}
	}
	x.dprintf("%d lines %d keys %d revs\n", len(xs.lines), len(xs.keys[0]), len(xs.revs))
	for _, r := range xs.revs {
		x.dprintf("\t%v", r)
	}
	x.dprintf("\n")
	for _, ks := range xs.keys {
		for _, k := range ks {
			x.dprintf("\t%v", k)
		}
		x.dprintf("\n")
	}

	sort.Stable(xs)

	last := ""
	for i, ln := range xs.lines {
		if x.uniq && i>0 && last==ln {
			continue
		}
		x.Printf("%s\n", ln)
		last = ln
	}
	return nil
}

func (x *xCmd) usage() {
	x.Usage(x.Stderr)
	x.Eprintf("\tkey is a range with optional type and optional reverse indication\n")
	x.Eprintf("\trange is addr,addr or addr\n")
	x.Eprintf("\taddr is fieldnb, or +fieldnb, or -fieldnb\n")
	x.Eprintf("\tsort types are: 's' (string, default), 'n' (number), 't' (time)\n")
	x.Eprintf("\ta reverse indication is a final 'r' character.\n")
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("{file}")
	x.Argv0 = argv[0]
	x.dprintf = dbg.FlagPrintf(x.Stderr, &x.debug)
	x.NewFlag("D", "debug", &x.debug)
	x.NewFlag("u", "(unique) do not print dup lines", &x.uniq)
	x.NewFlag("r", "key: use this field range as the sort key(s)", &x.keys)
	x.NewFlag("F", "sep: input field delimiter character(s) (or string under -1)", &x.seps)
	x.NewFlag("1", "fields separated by 1 run of the field delimiter string", &x.one)
	args, err := x.Parse(argv)
	if err != nil {
		x.usage()
		return err
	}
	if len(x.keys) == 0 {
		x.keys = append(x.keys, ",")
	}
	if err := x.parseKeys(); err != nil {
		return err
	}
	if len(args) > 1 {
		x.usage()
		return errors.New("usage")
	}
	if cmd.Ns == nil {
		cmd.MkNS()
	}
	x.xs = &xSort{
		lines:   []string{},
		dprintf: x.dprintf,
	}
	if err := cmd.RunFiles(x, args...); err != nil {
		return err
	}
	return x.sort()
}
