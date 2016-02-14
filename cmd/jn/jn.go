/*
	join records in input
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx"
	"errors"
	"sort"
	"strconv"
	"strings"
)

type line []string
struct file {
	lines   map[string]line
	nfields int
}

var (
	opts       = opt.New("{file}")
	ux         bool
	fld1, fld2 = 1, 1
	one        bool
	seps       string
	osep       string
	files      []*file
	keys       map[string]bool
	blanks     []string
)

func setSep() {
	if osep == "" {
		if seps == "" {
			osep = "\t"
		} else {
			osep = seps
		}
		if !one {
			osep = osep[:1]
		}
	}
}

func fields(s string) []string {
	if one {
		if seps == "" {
			seps = "\t"
		}
		return strings.Split(s, seps)
	}
	if seps == "" {
		return strings.Fields(s)
	}
	return strings.FieldsFunc(s, func(r rune) bool {
		return strings.ContainsRune(seps, r)
	})
}

func getFiles(in <-chan face{}) error {
	var f *file
	nfields := 0
	fldno := fld1
	name := "stdin"
	var err error
	for m := range in {
		switch m := m.(type) {
		case zx.Dir:
			name = m["Upath"]
			if name == "" {
				name = m["path"]
			}
			if f != nil {
				files = append(files, f)
				f = nil
				fldno = fld2
			}
		case []byte:
			if f == nil {
				f = &file{lines: map[string]line{}}
			}
			s := string(m)
			if len(s) > 0 && s[len(s)-1] == '\n' {
				s = s[:len(s)-1]
			}
			fields := fields(s)
			if len(fields) == 0 {
				continue
			}
			if len(fields) > nfields {
				nfields = len(fields)
			}
			if fldno < 1 || fldno > len(fields) {
				cmd.Warn("%s: wrong number of fields in '%s'", name, s)
				err = errors.New("wrong number of fields")
				continue
			}
			fld := fields[fldno-1]
			keys[fld] = true
			f.nfields = nfields
			if len(files) > 0 {
				// remove key from line
				f.nfields--
				var nflds line
				nflds = append(nflds, fields[0:fldno-1]...)
				nflds = append(nflds, fields[fldno:]...)
				fields = nflds
			}
			if f.lines[fld] != nil {
				cmd.Warn("%s: dup lines for key %s", name, fld)
			}
			f.lines[fld] = fields
		default:
			cmd.Dprintf("ignored %T\n", m)
		}
	}
	if f != nil {
		files = append(files, f)
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

func keyList() []string {
	ks := []string{}
	numbers := true
	for k := range keys {
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

func join() error {
	for _, k := range keyList() {
		var ln line
		for i, f := range files {
			var fln line
			if f.lines[k] == nil && i == 0 {
				fln = f.fakeLine(k, fld1)
			} else {
				fln = f.getLine(k)
			}
			ln = append(ln, fln...)
		}
		if _, err := cmd.Printf("%s\n", strings.Join(ln, osep)); err != nil {
			return err
		}
	}
	return nil
}

// Run print lines in the current app context.
func main() {
	cmd.UnixIO("err")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("u", "unix IO", &ux)
	opts.NewFlag("k1", "nb: join on this field nb. for 1st file", &fld1)
	opts.NewFlag("k2", "nb: join on this field nb. for 2nd and following files", &fld2)
	opts.NewFlag("i", "isep: input field separator character(s) or string under -1", &seps)
	opts.NewFlag("1", "fields are separated by one run of the separator string", &one)
	opts.NewFlag("o", "osep: output field delimiter string", &osep)
	args := opts.Parse()
	if ux {
		cmd.UnixIO("out")
	}
	keys = map[string]bool{}
	setSep()
	if err := getFiles(cmd.Lines(cmd.In("in"))); err != nil {
		cmd.Fatal(err)
	}
	if len(args) > 0 {
		in := cmd.Lines(cmd.Files(args...))
		if err := getFiles(cmd.Lines(in)); err != nil {
			cmd.Fatal(err)
		}
	}
	if err := join(); err != nil {
		cmd.Fatal(err)
	}
}
