/*
	sort lines in input
*/
package srt

import (
	"clive/dbg"
	"clive/app"
	"clive/app/opt"
	"time"
	"strconv"
	"strings"
	"fmt"
	"sort"
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
}

type xCmd {
	*opt.Flags
	*app.Ctx
	one, uniq, xflag bool
	seps      string
	addrs     []addr
	*xSort
	kargs      []string
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
	defer app.Dprintf("\t< %v %v -> %v\t\t%v\n", ki, kj, res, x.revs)
	for n := 0; n < len(ki); n++ {
		rev := x.revs[n]
		switch vi := ki[n].(type) {
		case float64:
			vj := kj[n].(float64)
			if rev {
				vi, vj = vj, vi
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
				vi, vj = vj, vi
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
				vi, vj = vj, vi
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

func (x *xCmd) parseKeys() error {
	for _, r := range x.kargs {
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
				n, err := strconv.Atoi(fld)
				if err != nil {
					app.Warn("non numeric field '%s'", fld)
				}
				nb = float64(n)
			}
			x.keys[i] = append(x.keys[i], nb)
		case sTime:
			t, err := opt.ParseTime(fld)
			if err != nil {
				app.Warn("non time field '%s'", fld)
			}
			x.keys[i] = append(x.keys[i], t)
		default:
			x.keys[i] = append(x.keys[i], fld)
		}
	}
}

func (x *xCmd) sort() error {
	x.keys = make([][]interface{}, len(x.lines))
	for _, a := range x.addrs {
		if a.from < 0 {
			a.from = len(x.lines) - (-a.from) + 1
		}
		if a.to < 0 {
			a.to = len(x.lines) - (-a.to) + 1
		}
		for i := a.from; i <= a.to; i++ {
			x.initKey(a.kind, i, a.rev, a.all, x.one, x.seps)
		}
	}
	app.Dprintf("%d lines %d keys %d revs:\n", len(x.lines), len(x.keys), len(x.revs))
	for _, r := range x.revs {
		app.Dprintf("\t%v", r)
	}
	app.Dprintf("\n")
	for _, ks := range x.keys {
		for _, k := range ks {
			app.Dprintf("\t%v", k)
		}
		app.Dprintf("\n")
	}

	sort.Stable(x)

	last := ""
	for i, ln := range x.lines {
		if x.uniq && i>0 && last==ln {
			continue
		}
		if err := app.Printf("%s\n", ln); err != nil {
			return err
		}
		last = ln
	}
	x.xSort = &xSort{}
	return nil
}

func (x *xCmd) getFiles(in chan interface{}) error {
	out := app.Out()
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
			if len(s)>0 && s[len(s)-1]=='\n' {
				s = s[:len(s)-1]
			}
			x.lines = append(x.lines, s)
		default:
			app.Dprintf("got %T\n", m)
			if x.xflag {
				// else we sort all files in input and
				// it's not meaningful to fwd dirs et al.
				if err := x.sort(); err != nil {
					return err
				}
				out <- m
			}
		}
	}
	if err := x.sort(); err != nil {
		return err
	}
	return cerror(in)
}

func (x *xCmd) setSep() {
	if x.one {
		if x.seps == "" {
			x.seps = "\t"
		}
	} else {
		if x.seps == "" {
			x.seps = " \t"
		}
	}
}

// Run print lines in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx(), xSort: &xSort{}, }
	x.Flags = opt.New("{file}")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("u", "(unique) do not print dup lines", &x.uniq)
	x.NewFlag("r", "key: use this field range as the sort key(s)", &x.kargs)
	x.NewFlag("F", "sep: input field delimiter character(s) (or string under -1)", &x.seps)
	x.NewFlag("1", "fields separated by 1 run of the field delimiter string", &x.one)
	x.NewFlag("x", "sort each extracted text on its own (eg. out from gr -x)", &x.xflag)
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
	if len(x.kargs) == 0 {
		x.kargs = append(x.kargs, ",")
	}
	if err := x.parseKeys(); err != nil {
		app.Fatal(err)
	}
	x.setSep()
	err = x.getFiles(app.Lines(app.In()))
	app.Exits(err)
}
