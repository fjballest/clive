/*
	sort lines in input
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
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

type addr struct {
	from, to int
	kind     sKind
	rev      bool
	all      bool
}

type xSort struct {
	lines []string
	keys  [][]interface{} // field or line keys to sort
	revs  []bool          // which addr is reverse order?
}


var (
	opts = opt.New("{file}")
	one, uniq, xflag bool
	seps             string
	addrs            []addr
	kargs []string
)

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
	defer cmd.Dprintf("\t< %v %v -> %v\t\t%v\n", ki, kj, res, x.revs)
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

func parseKeys() error {
	for _, r := range kargs {
		rev := false
		kind := sStr
		if len(r) > 0 && r[len(r)-1] == 'r' {
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
			addrs = append(addrs, a)
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
		addrs = append(addrs, a)
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
			if fldnb >= 1 && fldnb <= len(fields) {
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
					cmd.Warn("non numeric field '%s'", fld)
				}
				nb = float64(n)
			}
			x.keys[i] = append(x.keys[i], nb)
		case sTime:
			t, err := opt.ParseTime(fld)
			if err != nil {
				cmd.Warn("non time field '%s'", fld)
			}
			x.keys[i] = append(x.keys[i], t)
		default:
			x.keys[i] = append(x.keys[i], fld)
		}
	}
}

func (x *xSort) sort() error {
	x.keys = make([][]interface{}, len(x.lines))
	for _, a := range addrs {
		if a.from < 0 {
			a.from = len(x.lines) - (-a.from) + 1
		}
		if a.to < 0 {
			a.to = len(x.lines) - (-a.to) + 1
		}
		for i := a.from; i <= a.to; i++ {
			x.initKey(a.kind, i, a.rev, a.all, one, seps)
		}
	}
	cmd.Dprintf("%d lines %d keys %d revs:\n", len(x.lines), len(x.keys), len(x.revs))
	for _, r := range x.revs {
		cmd.Dprintf("\t%v", r)
	}
	cmd.Dprintf("\n")
	for _, ks := range x.keys {
		for _, k := range ks {
			cmd.Dprintf("\t%v", k)
		}
		cmd.Dprintf("\n")
	}

	sort.Stable(x)

	last := ""
	for i, ln := range x.lines {
		ln := ln
		if uniq && i > 0 && last == ln {
			continue
		}
		if _, err := cmd.Printf("%s\n", ln); err != nil {
			return err
		}
		last = ln
	}
	*x = xSort{}
	return nil
}

func sortFiles(in <-chan interface{}) error {
	out := cmd.Out("out")
	x := &xSort{}
	for m := range in {
		switch m := m.(type) {
		case []byte:
			s := string(m)
			if len(s) > 0 && s[len(s)-1] == '\n' {
				s = s[:len(s)-1]
			}
			x.lines = append(x.lines, s)
		default:
			cmd.Dprintf("got %T\n", m)
			if xflag {
				// else we sort all files in input and
				// it's not meaningful to fwd dirs and other msgs.
				if err := x.sort(); err != nil {
					close(in, err)
				}
				if ok := out <- m; !ok {
					close(in, cerror(out))
				}
			}
		}
	}
	if err := x.sort(); err != nil {
		return err
	}
	return cerror(in)
}

func setSep() {
	if one {
		if seps == "" {
			seps = "\t"
		}
	} else {
		if seps == "" {
			seps = " \t"
		}
	}
}

// Run print lines in the current app context.
func main() {
	c := cmd.AppCtx()
	cmd.UnixIO("err")
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("d", "do not print dup lines", &uniq)
	opts.NewFlag("r", "key: use this field range as the sort key(s)", &kargs)
	opts.NewFlag("F", "sep: input field delimiter character(s) (or string under -1)", &seps)
	opts.NewFlag("1", "fields separated by 1 run of the field delimiter string", &one)
	opts.NewFlag("x", "sort each extracted text on its own (eg. out from gr -x)", &xflag)
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
	if len(kargs) == 0 {
		kargs = append(kargs, ",")
	}
	if err := parseKeys(); err != nil {
		cmd.Fatal(err)
	}
	setSep()
	err = sortFiles(cmd.Lines(cmd.In("in")))
	if err != nil {
		cmd.Fatal(err)
	}
}
