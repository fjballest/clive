/*
	diffs command
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx"
	"errors"
	"fmt"
	"bytes"
)

type hash struct {
	lines  map[string]int
	hlines []string
}

type file struct {
	name  string
	lines []int
}

const (
	oEq = iota
	oAdd
	oDel
)

type rep struct {
	what, i, j int
}

type xFiles struct {
	rpath  string
	f      [2]file
	h      hash
	prefix []int
	suffix []int
	lcs    [][]int
	repc   chan rep
}

var (
	opts = opt.New("file [file]")
	lflag, qflag bool

	errDiffs = errors.New("diffs")
)

func (f file) String() string {
	return fmt.Sprintf("%s[%d]", f.name, len(f.lines))
}

func prefix(ln1, ln2 []int) ([]int, []int, []int) {
	ni := len(ln1)
	nj := len(ln2)
	i := 0
	for i < ni && i < nj {
		if ln1[i] != ln2[i] {
			break
		}
		i++
	}
	return ln1[:i], ln1[i:], ln2[i:]
}

func suffix(ln1, ln2 []int) ([]int, []int, []int) {
	ni := len(ln1)
	nj := len(ln2)
	i := 0
	for i < ni && i < nj {
		if ln1[ni-i-1] != ln2[nj-i-1] {
			break
		}
		i++
	}
	return ln1[ni-i:], ln1[:ni-i], ln2[:nj-i]
}

func (x *xFiles) addLine(ln []byte, fno int) {
	switch fno {
	case 0:
		return
	case 1, 2:
		fno--
		s := string(ln)
		if x.h.lines == nil {
			x.h.lines = map[string]int{}
		}
		id, ok := x.h.lines[s]
		if !ok {
			id = len(x.h.hlines)
			x.h.lines[s] = id
			x.h.hlines = append(x.h.hlines, s)
		}
		x.f[fno].lines = append(x.f[fno].lines, id)
	}
}

func (x *xFiles) report(i, j int) {
	ln1 := x.f[0].lines
	ln2 := x.f[1].lines
	if i > 0 && j > 0 && ln1[i-1] == ln2[j-1] {
		x.report(i-1, j-1)
		x.repc <- rep{oEq, i - 1, j - 1}
	} else if j > 0 && (i == 0 || x.lcs[i][j-1] >= x.lcs[i-1][j]) {
		x.report(i, j-1)
		x.repc <- rep{oAdd, i, j - 1}
	} else if i > 0 && (j == 0 || x.lcs[i][j-1] < x.lcs[i-1][j]) {
		x.report(i-1, j)
		x.repc <- rep{oDel, i - 1, j}
	}
}

func (x *xFiles) showreport() error {
	var err, oerr error
	if lflag {
		for _, ln := range x.prefix {
			if _, oerr = cmd.Printf("  %s", x.h.hlines[ln]); oerr != nil {
				err = oerr
			}
		}
		cmd.Dprintf("---\n")
	}
	f1 := x.f[0]
	f2 := x.f[1]
	a1 := zx.Addr{Name: f1.name}
	a2 := zx.Addr{Name: f2.name}
	var n1, n2 int
	buf := &bytes.Buffer{}
	out := cmd.Out("out")
	var r rep
	for r = range x.repc {
		switch r.what {
		case oEq:
			if buf.Len() > 0 {
				if a1.Ln0 == 0 {
					a1.Ln0 = r.i+1+len(x.prefix)
					n1 = 1
				}
				if a2.Ln0 == 0 {
					a2.Ln0 = r.j+1+len(x.prefix)
					n2 = 1
				}
				a1.Ln1 = a1.Ln0+n1-1
				a2.Ln1 = a2.Ln0+n2-1
				out <- []byte(fmt.Sprintf("#diff\t%s\t%s\n", a1, a2))
				
				if ok := out <- buf.Bytes(); !ok {
					err = cerror(out)
				}
				buf = &bytes.Buffer{}
			}
			a1 = zx.Addr{Name: f1.name}
			a2 = zx.Addr{Name: f2.name}
			n1, n2 = 0, 0
			if lflag {
				_, oerr = cmd.Printf("  %s", x.h.hlines[f1.lines[r.i]])
			}
		case oAdd:
			if a2.Ln0 == 0 {
				a2.Ln0 = r.j+1+len(x.prefix)
				n2++
			} else {
				n2++
			}
			fmt.Fprintf(buf, "+ %s", x.h.hlines[f2.lines[r.j]])
			err = errDiffs
		case oDel:
			if a1.Ln0 == 0 {
				a1.Ln0= r.i+1+len(x.prefix)
				n1++
			} else {
				n1++
			}
			fmt.Fprintf(buf, "- %s", x.h.hlines[f1.lines[r.i]])
			err = errDiffs
		}
	}
	if buf.Len() > 0 {
		if a1.Ln0 == 0 {
			a1.Ln0 = r.i+len(x.prefix)
			n1 = 1
		}
		if a2.Ln0 == 0 {
			a2.Ln0 = r.j+len(x.prefix)
			n2 = 1
		}
		a1.Ln1 = a1.Ln0+n1-1
		a2.Ln1 = a2.Ln0+n2-1
		cmd.Printf("#diff\t%s\t%s\n", a1, a2)
		if ok := out <- buf.Bytes(); !ok {
			return err
		}
	}
	if lflag {
		cmd.Dprintf("---\n")
		for _, ln := range x.suffix {
			if _, err = cmd.Printf("  %s", x.h.hlines[ln]); err != nil {
				return err
			}
		}
	}
	return err
}

func (x *xFiles) diff() error {
	f1, f2 := x.f[0], x.f[1]
	x.prefix, f1.lines, f2.lines = prefix(f1.lines, f2.lines)
	x.suffix, f1.lines, f2.lines = suffix(f1.lines, f2.lines)
	x.f[0], x.f[1] = f1, f2
	if len(f1.lines) == 0 && len(f2.lines) == 0 {
		return nil
	}
	if _, err := cmd.Printf("#diffs\t%s\t%s\n", f1.name, f2.name); err != nil {
		return err
	}
	if qflag {
		return errDiffs
	}
	ni := len(f1.lines) + 1
	nj := len(f2.lines) + 1
	x.lcs = make([][]int, ni)
	for i := 0; i < ni; i++ {
		x.lcs[i] = make([]int, nj)
	}
	for i := 1; i < ni; i++ {
		for j := 1; j < nj; j++ {
			if f1.lines[i-1] == f2.lines[j-1] {
				x.lcs[i][j] = x.lcs[i-1][j-1] + 1
			} else if x.lcs[i][j-1] > x.lcs[i-1][j] {
				x.lcs[i][j] = x.lcs[i][j-1]
			} else {
				x.lcs[i][j] = x.lcs[i-1][j]
			}
		}
	}
	x.repc = make(chan rep)
	rdonec := make(chan bool)
	go func() {
		x.report(ni-1, nj-1)
		close(x.repc)
		rdonec <- true
	}()
	err := x.showreport()
	<-rdonec
	return err
}

func (x *xFiles) getFile(in <-chan interface{}, fno int) zx.Dir {
	cmd.Dprintf("getfile %d\n", fno)
	for m := range in {
		if e, ok := m.(error); ok {
			cmd.Warn("%s", e)
			continue
		}
		if d, ok := m.(zx.Dir); ok {
			return d
		}
		if b, ok := m.([]byte); ok {
			x.addLine(b, fno)
		}
	}
	if err := cerror(in); err != nil {
		cmd.Warn("%s", err)
	}
	return nil
}

func (x *xFiles) getFiles(i1, i2 <-chan interface{}) (zx.Dir, zx.Dir) {
	cmd.Dprintf("getfiles\n")
	doselect {
	case m, ok := <-i1:
		if !ok {
			if err := cerror(i1); err != nil {
				cmd.Warn("%s", err)
			}
			return nil, x.getFile(i2, 2)
		}
		if e, ok := m.(error); ok {
			cmd.Warn("%s", e)
			continue
		}
		if d, ok := m.(zx.Dir); ok {
			return d, x.getFile(i2, 2)
		}
		if b, ok := m.([]byte); ok {
			x.addLine(b, 1)
		}
	case m, ok := <-i2:
		if !ok {
			if err := cerror(i2); err != nil {
				cmd.Warn("%s", err)
			}
			return x.getFile(i1, 1), nil
		}
		if e, ok := m.(error); ok {
			cmd.Warn("%s", e)
			continue
		}
		if d, ok := m.(zx.Dir); ok {
			return x.getFile(i1, 1), d
		}
		if b, ok := m.([]byte); ok {
			x.addLine(b, 2)
		}
	}
}

func (x *xFiles) getDir(in <-chan interface{}) zx.Dir {
	for m := range in {
		if e, ok := m.(error); ok {
			cmd.Warn("%s", e)
			continue
		}
		if d, ok := m.(zx.Dir); ok {
			return d
		}
	}
	if err := cerror(in); err != nil {
		cmd.Warn("%s", err)
	}
	return nil
}

func (x *xFiles) getDirs(i1, i2 <-chan interface{}) (zx.Dir, zx.Dir) {
	cmd.Dprintf("getdirs\n")
	var d1, d2 zx.Dir
	doselect {
	case m1, ok := <-i1:
		if !ok {
			if err := cerror(i1); err != nil {
				cmd.Warn("%s", err)
			}
			cmd.Dprintf("getdir 2\n")
			return nil, x.getDir(i2)
		}
		if e, ok := m1.(error); ok {
			cmd.Warn("%s", e)
			continue
		}
		d1, ok = m1.(zx.Dir)
		if !ok {
			continue
		}
		cmd.Dprintf("getdir 2\n")
		return d1, x.getDir(i2)
	case m2, ok := <-i2:
		if !ok {
			if err := cerror(i2); err != nil {
				cmd.Warn("%s", err)
			}
			cmd.Dprintf("getdir 1\n")
			return x.getDir(i1), nil
		}
		if e, ok := m2.(error); ok {
			cmd.Warn("%s", e)
			continue
		}
		d2, ok = m2.(zx.Dir)
		if !ok {
			continue
		}
		cmd.Dprintf("getdir 1\n")
		return x.getDir(i1), d2
	}
}

// Run diffs in the current app context.
func main() {
	cmd.UnixIO("err")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("l", "long output", &lflag)
	opts.NewFlag("q", "quiet output (report which files differ)", &qflag)
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
	var i1, i2 <-chan interface{}
	switch len(args) {
	case 1:
		i1 = cmd.In("in")
		i2 = cmd.Files(args[0])
	case 2:
		i1 = cmd.Files(args[0])
		i2 = cmd.Files(args[1])
	default:
		cmd.Warn("wrong number of arguments")
		opts.Usage()
	}
	i1 = cmd.Lines(i1)
	i2 = cmd.Lines(i2)

	var sts error
	x := &xFiles{}
	d1, d2 := x.getDirs(i1, i2)
	for d1 != nil || d2 != nil {
		cmd.Dprintf("loop d1 %s d2 %s\n", d1["Rpath"], d2["Rpath"])
		switch cmp := zx.PathCmp(d1["Rpath"], d2["Rpath"]); {
		case d2 == nil || (d1 != nil && cmp < 0):
			d1["path"] = d1["Upath"]
			_, err := cmd.Printf("#only %s\n", d1.Fmt())
			if err != nil {
				close(i1, err)
				close(i2, err)
				cmd.Fatal(err)
			}
			d1 = x.getDir(i1)
			sts = errDiffs
			continue
		case d1 == nil || (d2 != nil && cmp > 0):
			d2["path"] = d2["Upath"]
			_, err := cmd.Printf("#only %s\n",  d2.Fmt())
			if err != nil {
				close(i1, err)
				close(i2, err)
				cmd.Fatal(err)
			}
			d2 = x.getDir(i2)
			sts = errDiffs
			continue
		default:
			if d1["type"] != d2["type"] {
				d1["path"] = d1["Upath"]
				cmd.Printf("#only %s\n", d1.Fmt())
				d2["path"] = d2["Upath"]
				cmd.Printf("#only %s\n", d2.Fmt())
				sts = errDiffs
			}
		}
		o1, o2 := d1, d2
		x.rpath = d1["Rpath"]
		x.f[0] = file{name: d1["Upath"]}
		x.f[1] = file{name: d2["Upath"]}
		cmd.Dprintf("getfiles %s %s\n", x.f[0], x.f[1])
		d1, d2 = x.getFiles(i1, i2)
		if o1["type"] == "-" && o2["type"] == "-" {
			cmd.Dprintf("run diff %s %s\n", x.f[0], x.f[1])
			if err := x.diff(); err != nil {
				sts = err
			}
			x = &xFiles{}
		}
	}
	if sts != errDiffs {
		cmd.Fatal(sts)
	}
	cmd.Exit(sts)
}
