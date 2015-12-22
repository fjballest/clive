/*
	join records in input
*/
package jn

import (
	"clive/app"
	"clive/app/opt"
	"clive/dbg"
	"clive/zx"
	"errors"
	"sort"
	"strconv"
	"strings"
)

type line []string
type file struct {
	lines   map[string]line
	nfields int
}

type xCmd struct {
	*opt.Flags
	*app.Ctx

	fld1, fld2 int
	one        bool
	seps       string
	osep       string
	files      []*file
	keys       map[string]bool
	blanks     []string
}

func (x *xCmd) setSep() {
	if x.osep == "" {
		if x.seps == "" {
			x.osep = "\t"
		} else {
			x.osep = x.seps
		}
		if !x.one {
			x.osep = x.osep[:1]
		}
	}
}

func (x *xCmd) fields(s string) []string {
	if x.one {
		if x.seps == "" {
			x.seps = "\t"
		}
		return strings.Split(s, x.seps)
	}
	if x.seps == "" {
		return strings.Fields(s)
	}
	return strings.FieldsFunc(s, func(r rune) bool {
		return strings.ContainsRune(x.seps, r)
	})
}

func (x *xCmd) getFiles(in chan interface{}) error {
	var f *file
	nfields := 0
	fldno := x.fld1
	name := "stdin"
	var err error
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
		case zx.Dir:
			name = m["upath"]
			if f != nil {
				x.files = append(x.files, f)
				f = nil
				fldno = x.fld2
			}
		case []byte:
			if f == nil {
				f = &file{lines: map[string]line{}}
			}
			s := string(m)
			if len(s) > 0 && s[len(s)-1] == '\n' {
				s = s[:len(s)-1]
			}
			fields := x.fields(s)
			if len(fields) == 0 {
				continue
			}
			if len(fields) > nfields {
				nfields = len(fields)
			}
			if fldno < 1 || fldno > len(fields) {
				app.Warn("%s: wrong number of fields in '%s'", name, s)
				err = errors.New("wrong number of fields")
				continue
			}
			fld := fields[fldno-1]
			x.keys[fld] = true
			f.nfields = nfields
			if len(x.files) > 0 {
				// remove key from line
				f.nfields--
				var nflds line
				nflds = append(nflds, fields[0:fldno-1]...)
				nflds = append(nflds, fields[fldno:]...)
				fields = nflds
			}
			if f.lines[fld] != nil {
				app.Warn("%s: dup lines for key %s", name, fld)
			}
			f.lines[fld] = fields
		default:
			app.Dprintf("ignored %T\n", m)
		}
	}
	if f != nil {
		x.files = append(x.files, f)
	}
	if err == nil {
		err = cerror(in)
	}
	return err
}

type asNumbers []string

func (x asNumbers) Len() int      { return len(x) }
func (x asNumbers) Swap(i, j int) { x[i], x[j] = x[j], x[i] }
func (x asNumbers) Less(i, j int) bool {
	// could convert first and then sort, but this suffices by now.
	n1, _ := strconv.ParseFloat(x[i], 64)
	n2, _ := strconv.ParseFloat(x[j], 64)
	return n1 < n2
}

func (x *xCmd) keyList() []string {
	ks := []string{}
	numbers := true
	for k := range x.keys {
		if numbers && k != "" && k != "-" {
			_, err := strconv.ParseFloat(k, 64)
			numbers = err == nil
		}
		ks = append(ks, k)
	}
	if numbers {
		sort.Sort(asNumbers(ks))
	} else {
		sort.Sort(sort.StringSlice(ks))
	}
	return ks
}

func (f file) getLine(k string) line {
	ln := f.lines[k]
	for i := len(ln); i < f.nfields; i++ {
		ln = append(ln, "-")
	}
	return ln
}

func (f file) fakeLine(k string, fno int) line {
	var ln line
	for i := 0; i < f.nfields; i++ {
		if i+1 == fno {
			ln = append(ln, k)
		} else {
			ln = append(ln, "-")
		}
	}
	return ln
}

func (x *xCmd) join() error {
	for _, k := range x.keyList() {
		var ln line
		for i, f := range x.files {
			var fln line
			if f.lines[k] == nil && i == 0 {
				fln = f.fakeLine(k, x.fld1)
			} else {
				fln = f.getLine(k)
			}
			ln = append(ln, fln...)
		}
		if err := app.Printf("%s\n", strings.Join(ln, x.osep)); err != nil {
			return err
		}
	}
	return nil
}

// Run print lines in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("{file}")
	x.NewFlag("D", "debug", &x.Debug)
	x.fld1 = 1
	x.NewFlag("k1", "nb: join on this field nb. for 1st file", &x.fld1)
	x.fld2 = 1
	x.NewFlag("k2", "nb: join on this field nb. for 2nd and following files", &x.fld2)
	x.NewFlag("i", "isep: input field separator character(s) or string under -1", &x.seps)
	x.NewFlag("1", "fields are separated by one run of the separator string", &x.one)
	x.NewFlag("o", "osep: output field delimiter string", &x.osep)
	args, err := x.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}
	x.keys = map[string]bool{}
	x.setSep()
	if err := x.getFiles(app.Lines(app.In())); err != nil {
		app.Exits(err)
	}
	in := app.Lines(app.Files(args...))
	if err := x.getFiles(app.Lines(in)); err != nil {
		app.Exits(err)
	}
	app.Exits(x.join())
}
