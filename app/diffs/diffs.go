/*
	diffs command
*/
package diffs

import (
	"clive/dbg"
	"clive/app"
	"clive/app/opt"
	"clive/zx"
	"errors"
	"fmt"
)

type hash  {
	lines  map[string]int
	hlines []string
}

type file  {
	name  string
	lines []int
}

const (
	oEq = iota
	oAdd
	oDel
)

type rep  {
	what, i, j int
}

type xFiles {
	rpath string
	f [2]file
	h hash
	prefix []int
	suffix []int
	lcs    [][]int
	repc   chan rep
}

type xCmd {
	*opt.Flags
	*app.Ctx

	lflag, qflag bool
	xFiles
}

func (f file) String() string {
	return fmt.Sprintf("%s[%d]", f.name, len(f.lines))
}

func prefix(ln1, ln2 []int) ([]int, []int, []int) {
	ni := len(ln1)
	nj := len(ln2)
	i := 0
	for i<ni && i<nj {
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
	for i<ni && i<nj {
		if ln1[ni-i-1] != ln2[nj-i-1] {
			break
		}
		i++
	}
	return ln1[ni-i:], ln1[:ni-i], ln2[:nj-i]
}

func (x *xCmd) addLine(ln []byte, fno int) {
	// app.Dprintf("add %d lines\n", fno)
	switch fno {
	case 0:
		return
	case 1,2:
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

func (x *xCmd) report(i, j int) {
	ln1 := x.f[0].lines
	ln2 := x.f[1].lines
	if i>0 && j>0 && ln1[i-1]==ln2[j-1] {
		x.report(i-1, j-1)
		x.repc <- rep{oEq, i - 1, j - 1}
	} else if j>0 && (i==0 || x.lcs[i][j-1]>=x.lcs[i-1][j]) {
		x.report(i, j-1)
		x.repc <- rep{oAdd, i, j - 1}
	} else if i>0 && (j==0 || x.lcs[i][j-1]<x.lcs[i-1][j]) {
		x.report(i-1, j)
		x.repc <- rep{oDel, i - 1, j}
	}
}

func (x *xCmd) showreport() error {
	if x.lflag {
		for _, ln := range x.prefix {
			app.Printf("  %s", x.h.hlines[ln])
		}
		app.Dprintf("---\n")
	}
	changing := false
	f1 := x.f[0]
	f2 := x.f[1]
	var err error
	for r := range x.repc {
		switch r.what {
		case oEq:
			changing = false
			if x.lflag {
				app.Printf("  %s", x.h.hlines[f1.lines[r.i]])
			}
		case oAdd:
			if !changing {
				changing = true
				app.Printf("#diff\t%s:%d\tand\t%s:%d\n",
					f1.name, r.i+len(x.prefix),
					f2.name, r.j+1+len(x.prefix))
			}
			app.Printf("+ %s", x.h.hlines[f2.lines[r.j]])
			if err == nil {
				err = errors.New("diffs")
			}
		case oDel:
			if !changing {
				changing = true
				app.Printf("#diff\t%s:%d\tand\t%s:%d\n",
					f1.name, r.i+1+len(x.prefix),
					f2.name, r.j+len(x.prefix))
			}
			app.Printf("- %s", x.h.hlines[f1.lines[r.i]])
			if err == nil {
				err = errors.New("diffs")
			}
		}
	}
	if x.lflag {
		app.Dprintf("---\n")
		for _, ln := range x.suffix {
			app.Printf("  %s", x.h.hlines[ln])
		}
	}
	return err
}

func (x *xCmd) diff() error {
	f1, f2 := x.f[0], x.f[1]
	x.prefix, f1.lines, f2.lines = prefix(f1.lines, f2.lines)
	x.suffix, f1.lines, f2.lines = suffix(f1.lines, f2.lines)
	x.f[0], x.f[1] = f1, f2
	if len(f1.lines) == 0 && len(f2.lines) == 0 {
		return nil
	}
	app.Printf("#diffs\t%s\tand\t%s\n", f1.name, f2.name)
	if x.qflag {
		return errors.New("diffs")
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
	go func() {
		x.report(ni-1, nj-1)
		close(x.repc)
	}()
	return x.showreport()
}

func (x *xCmd) getFile(in chan interface{}, fno int) zx.Dir {
	app.Dprintf("getfile %d\n", fno)
	doselect {
	case s := <-x.Sig:
		app.Dprintf("got sig %s\n", s)
		app.Fatal(dbg.ErrIntr)
	case m, ok := <- in:
		if !ok {
			return nil
		}
		if d, ok := m.(zx.Dir); ok {
			return d
		}
		if b, ok := m.([]byte); ok {
			x.addLine(b, fno)
			continue
		}
	}
}

func (x *xCmd) getFiles(i1, i2 chan interface{}) (zx.Dir, zx.Dir) {
	app.Dprintf("getfiles\n")
	doselect {
	case s := <-x.Sig:
		app.Dprintf("got sig %s\n", s)
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-i1:
		if !ok {
			return nil, x.getFile(i2, 2)
		}
		if d, ok := m.(zx.Dir); ok {
			return d, x.getFile(i2, 2)
		}
		if b, ok := m.([]byte); ok {
			x.addLine(b, 1)
			continue
		}
	case m, ok := <-i2:
		if !ok {
			return x.getFile(i1, 1), nil
		}
		if d, ok := m.(zx.Dir); ok {
			return x.getFile(i1, 1), d
		}
		if b, ok := m.([]byte); ok {
			x.addLine(b, 2)
			continue
		}
	}
}

func (x *xCmd) getDir(in chan interface{}) zx.Dir {
	doselect {
	case s := <-x.Sig:
		app.Dprintf("got sig %s\n", s)
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-in:
		if !ok {
			return nil
		}
		d, ok := m.(zx.Dir)
		if ok {
			return d
		}
	}
}

func (x *xCmd) getDirs(i1, i2 chan interface{}) (zx.Dir, zx.Dir) {
	app.Dprintf("getdirs\n")
	var d1, d2 zx.Dir
	doselect {
	case s := <-x.Sig:
		app.Dprintf("got sig %s\n", s)
		app.Fatal(dbg.ErrIntr)
	case m1, ok := <- i1:
		if !ok {
			app.Dprintf("getdir 2\n")
			return nil, x.getDir(i2)
		}
		d1, ok = m1.(zx.Dir)
		if !ok {
			continue
		}
		app.Dprintf("getdir 2\n")
		return d1, x.getDir(i2)
	case m2, ok := <- i2:
		if !ok {
			app.Dprintf("getdir 1\n")
			return x.getDir(i1), nil
		}
		d2, ok = m2.(zx.Dir)
		if !ok {
			continue
		}
		app.Dprintf("getdir 1\n")
		return x.getDir(i1), d2
	}
}

// Run cnt in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("#pipe")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("l", "long output", &x.lflag)
	x.NewFlag("q", "quiet output (report which files differ)", &x.qflag)
	args, err := x.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}
	if len(args) != 1 || len(args[0]) == 0 || args[0][0] != '#' {
		x.Usage()
		app.Exits("usage")
	}
	var d1, d2 zx.Dir
	i1 := app.Lines(app.In())
	i2, err := app.IOarg(args[0])
	if err != nil {
		app.Fatal(err)
	}
	i2 = app.Lines(i2)
	var sts error
	d1, d2 = x.getDirs(i1, i2)
	for d1 != nil || d2 != nil {
		app.Dprintf("d1 %s d2 %s\n", d1["rpath"], d2["rpath"])

		switch cmp := zx.PathCmp(d1["rpath"], d2["rpath"]); {
		case d2 == nil || cmp < 0 :
			app.Printf("#only %s type %s\n", d1["upath"], d1["type"])
			d1 = x.getDir(i1)
			sts = errors.New("diffs")
			continue
		case d1 == nil || cmp > 0:
			app.Printf("#only %s type %s\n", d2["upath"], d2["type"])
			d2 = x.getDir(i2)
			sts = errors.New("diffs")
			continue
		default:
			if d1["type"] != d2["type"] {
				app.Printf("#only %s type %s\n", d1["upath"], d1["type"])
				app.Printf("#only %s type %s\n", d2["upath"], d2["type"])
				sts = errors.New("diffs")
			}
		}
		o1, o2 := d1, d2
		x.rpath = d1["rpath"]
		x.f[0] = file{name: d1["upath"]}
		x.f[1] = file{name: d2["upath"]}
		app.Dprintf("getfiles %s %s\n", x.f[0], x.f[1])
		d1, d2 = x.getFiles(i1, i2)
		if o1["type"] == "-" && o2["type"] == "-" {
			app.Dprintf("run diff %s %s\n", x.f[0], x.f[1])
			if err := x.diff(); err != nil {
				sts = err
			}
			x.xFiles = xFiles{}
		}
	}
	app.Exits(sts)
}
