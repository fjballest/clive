/*
	Ql builtin and external jn command.
	join records of files
*/
package jn

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/nchan"
	"clive/zx"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type addr  {
	from, to int
}

type flines map[string][]string

type xCmd  {
	*cmd.Ctx
	*opt.Flags
	debug      bool
	fld1, fld2 int
	one        bool
	seps       string
	osep       string
	files      []flines
	nf1fields  int
	blanks     []string
}

func (x *xCmd) RunFile(d zx.Dir, dc <-chan []byte) error {
	name := d["path"]
	if dc == nil {
		return nil
	}
	rc := nchan.Lines(dc, '\n')
	lines := map[string][]string{}
	fldno := x.fld1
	if len(x.files) > 0 {
		fldno = x.fld2
	}
	nfields := 0
	var err error
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
		if len(fields) > nfields {
			nfields = len(fields)
		}
		if len(fields) == 0 {
			continue
		}
		if fldno<1 || fldno>len(fields) {
			x.Warn("%s: wrong number of fields in '%s'", name, s)
			err = errors.New("wrong number of fields")
			continue
		}
		fld := fields[fldno-1]
		if len(x.files) > 0 {
			// remove key from line
			nflds := []string{}
			nflds = append(nflds, fields[0:fldno-1]...)
			nflds = append(nflds, fields[fldno:]...)
			joins := strings.Join(nflds, x.osep)
			lines[fld] = append(lines[fld], joins)
		} else {
			joins := strings.Join(fields, x.osep)
			lines[fld] = append(lines[fld], joins)
		}
	}
	if len(x.files) > 0 {
		nfields-- // key was removed
	} else {
		x.nf1fields = nfields
	}
	x.files = append(x.files, lines)
	ef := x.osep + "-"
	if nfields > 0 {
		x.blanks = append(x.blanks, "-"+strings.Repeat(ef, nfields-1))
	}
	return err
}

func (x *xCmd) printLine(key string, ln string, files []flines, blanks []string) {
	if len(files) == 0 {
		x.Printf("%s\n", ln)
		return
	}
	f1 := files[0]
	blank := blanks[0]
	files = files[1:]
	blanks = blanks[1:]
	if len(ln) > 0 {
		ln += x.osep
	}
	if len(f1[key]) == 0 {
		ln += blank
		x.printLine(key, ln, files, blanks)
		return
	}
	for _, f1ln := range f1[key] {
		x.printLine(key, ln+f1ln, files, blanks)
	}
}

func (x *xCmd) join() error {
	keys := []string{}
	for k := range x.files[0] {
		keys = append(keys, k)
	}
	f1fmts := []string{}
	for i := 0; i < x.nf1fields; i++ {
		if i+1 == x.fld1 {
			f1fmts = append(f1fmts, "%s")
		} else {
			f1fmts = append(f1fmts, "-")
		}
	}
	f1fmt := strings.Join(f1fmts, x.osep)
	for _, f := range x.files[1:] {
		for k := range f {
			if _, ok := x.files[0][k]; !ok {
				keys = append(keys, k)
				x.files[0][k] = []string{fmt.Sprintf(f1fmt, k)}
			}
		}
	}
	sort.Sort(sort.StringSlice(keys))
	for _, k := range keys {
		x.printLine(k, "", x.files, x.blanks)
	}
	return nil
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("file {file}")
	x.Argv0 = argv[0]
	x.NewFlag("D", "debug", &x.debug)
	x.fld1 = 1
	x.NewFlag("k1", "join on this field for 1st file", &x.fld1)
	x.fld2 = 1
	x.NewFlag("k2", "join on this field for 2nd and following files", &x.fld2)
	x.NewFlag("F", "sep: input field delimiter character(s) (or string under -1)", &x.seps)
	x.NewFlag("1", "fields separated by 1 run of the field delimiter string", &x.one)
	x.NewFlag("o", "sep: output field delimiter string", &x.osep)
	args, err := x.Parse(argv)
	if err != nil {
		x.Usage(x.Stderr)
		return err
	}
	if len(args) == 0 {
		x.Usage(x.Stderr)
		return errors.New("missing file name")
	}
	if cmd.Ns == nil {
		cmd.MkNS()
	}
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
	if len(args) == 1 {
		dc := make(chan []byte)
		go func() {
			_, _, err := nchan.ReadBytesFrom(x.Stdin, dc)
			close(dc, err)
		}()
		if err := x.RunFile(zx.Dir{"path": "-"}, dc); err != nil {
			x.Warn("stdin: %s", err)
			return errors.New("errors")
		}

	}
	if err := cmd.RunFiles(x, args...); err != nil {
		return err
	}
	return x.join()
}
