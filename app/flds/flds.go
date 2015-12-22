/*
	print fields in input
*/
package flds

import (
	"bytes"
	"clive/app"
	"clive/app/opt"
	"clive/dbg"
	"fmt"
	"strings"
)

type xCmd struct {
	*opt.Flags
	*app.Ctx
	ranges []string
	one    bool
	seps   string
	osep   string
	addrs  []opt.Range
	all    bool
}

func (x *xCmd) parseRanges() error {
	for _, r := range x.ranges {
		a, err := opt.ParseRange(r)
		if err != nil {
			return err
		}
		if a.P0 == 1 && a.P1 == -1 {
			x.all = true
		}
		x.addrs = append(x.addrs, a)
	}
	return nil
}

func (x *xCmd) flds(in, out chan interface{}) {
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
	case <-x.Sig:
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-in:
		if !ok {
			break
		}
		dat, ok := m.([]byte)
		if !ok {
			app.Dprintf("got %T\n", m)
			out <- m
			continue
		}
		s := string(dat)
		if len(s) > 0 && s[len(s)-1] == '\n' {
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
			app.Printf("%s\n", strings.Join(fields, osep))
			continue
		}
		b := &bytes.Buffer{}
		sep := ""
		for na, a := range x.addrs {
			for i, fld := range fields {
				nfld := i + 1
				app.Dprintf("tl match %d of %d in a%d %s\n", nfld, len(fields), na, a)
				if a.Matches(nfld, len(fields)) {
					fmt.Fprintf(b, "%s%s", sep, fld)
					sep = osep
				}
			}
		}
		fmt.Fprintf(b, "\n")
		if ok := out <- b.Bytes(); !ok {
			app.Fatal(cerror(out))
		}
	}
}

// Run print fields in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("{file}")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("r", "range: print this range", &x.ranges)
	x.NewFlag("F", "sep: input field delimiter character(s) (or string under -1)", &x.seps)
	x.NewFlag("o", "sep: output field delimiter string", &x.osep)
	x.NewFlag("1", "fields separated by 1 run of the field delimiter string", &x.one)
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
	if len(x.ranges) == 0 {
		x.ranges = append(x.ranges, ",")
	}
	if err := x.parseRanges(); err != nil {
		app.Fatal(err)
	}
	in := app.Lines(app.In())
	x.flds(in, app.Out())
	app.Exits(cerror(in))
}
