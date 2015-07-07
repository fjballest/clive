/*
	Ql builtin and external comp command.
	compare files
*/
package comp

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/dbg"
	"clive/nchan"
	"clive/zx"
	"errors"
	"strings"
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

type xCmd  {
	*cmd.Ctx
	*opt.Flags
	debug, lflag bool
	dprintf      dbg.PrintFunc
	files        []file
	h            hash

	ndiffs int
	f1, f2 file
	prefix []int
	suffix []int
	lcs    [][]int
	repc   chan rep
}

func (x *xCmd) RunFile(d zx.Dir, dc <-chan []byte) error {
	name := d["path"]
	if dc == nil {
		return nil
	}
	rc := nchan.Lines(dc, '\n')
	// change the file lines with a sequence of ids for hashed line values
	f := file{name: name}
	var err error
	doselect {
	case <-x.Intrc:
		close(rc, "interrupted")
		return errors.New("interrupted")
	case s, ok := <-rc:
		if !ok {
			err = cerror(rc)
			break
		}
		if id, ok := x.h.lines[s]; ok {
			f.lines = append(f.lines, id)
		} else {
			x.h.lines[s] = len(x.h.hlines)
			f.lines = append(f.lines, len(x.h.hlines))
			x.h.hlines = append(x.h.hlines, s)
		}
	}
	x.files = append(x.files, f)
	return err
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

func (x *xCmd) report(i, j int) {
	ln1 := x.f1.lines
	ln2 := x.f2.lines
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

func (x *xCmd) diff() {
	if x.debug {
		for _, ln := range x.f1.lines {
			x.dprintf("1: %s", x.h.hlines[ln])
		}
		x.dprintf("---\n")
		for _, ln := range x.f2.lines {
			x.dprintf("2: %s", x.h.hlines[ln])
		}
		x.dprintf("---\n")
	}
	ni := len(x.f1.lines) + 1
	nj := len(x.f2.lines) + 1
	x.lcs = make([][]int, ni)
	for i := 0; i < ni; i++ {
		x.lcs[i] = make([]int, nj)
	}
	for i := 1; i < ni; i++ {
		for j := 1; j < nj; j++ {
			if x.f1.lines[i-1] == x.f2.lines[j-1] {
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
	x.showreport()
}

func (x *xCmd) showreport() {
	if x.lflag {
		for _, ln := range x.prefix {
			x.Printf("  %s", x.h.hlines[ln])
		}
		x.dprintf("---\n")
	}
	changing := false
	for r := range x.repc {
		switch r.what {
		case oEq:
			changing = false
			if x.lflag {
				x.Printf("  %s", x.h.hlines[x.f1.lines[r.i]])
			}
		case oAdd:
			if !changing {
				changing = true
				x.Printf("#diff:\t%s:%d:\tand\t%s:%d:\n",
					x.f1.name, r.i+len(x.prefix),
					x.f2.name, r.j+1+len(x.prefix))
			}
			x.Printf("+ %s", x.h.hlines[x.f2.lines[r.j]])
		case oDel:
			if !changing {
				changing = true
				x.Printf("#diff\t%s:%d:\tand\t%s:%d:\n",
					x.f1.name, r.i+1+len(x.prefix),
					x.f2.name, r.j+len(x.prefix))
			}
			x.Printf("- %s", x.h.hlines[x.f1.lines[r.i]])
		}
	}
	if x.lflag {
		x.dprintf("---\n")
		for _, ln := range x.suffix {
			x.Printf("  %s", x.h.hlines[ln])
		}
	}
}

func (x *xCmd) comp() error {
	if len(x.files) != 2 {
		return errors.New("diff must have 2 files")
	}

	x.f1, x.f2 = x.files[0], x.files[1]
	x.prefix, x.f1.lines, x.f2.lines = prefix(x.f1.lines, x.f2.lines)
	x.suffix, x.f1.lines, x.f2.lines = suffix(x.f1.lines, x.f2.lines)
	if len(x.f1.lines)!=0 || len(x.f2.lines)!=0 {
		x.ndiffs++
		x.diff()
	}
	return nil
}

func (x *xCmd) compTreeFile(d1, d2 zx.Dir) error {
	x.h.lines = make(map[string]int)
	x.h.hlines = nil
	x.files = nil
	x.f1 = file{}
	x.f2 = file{}
	x.prefix = nil
	x.suffix = nil
	x.lcs = nil
	x.repc = nil
	if d1["type"] != d2["type"] {
		x.Printf("#removed: %s\n", d1["path"])
		x.Printf("#created: %s\n", d2["path"])
		x.ndiffs++
		return nil
	}
	if d1["type"] == "d" {
		return nil
	}
	fs1, err := zx.DirTree(d1)
	if err != nil {
		return err
	}
	fs2, err := zx.DirTree(d2)
	if err != nil {
		return err
	}
	d1c := fs1.Get(d1["spath"], 0, zx.All, "")
	if err := x.RunFile(d1, d1c); err != nil {
		return err
	}
	d2c := fs2.Get(d2["spath"], 0, zx.All, "")
	if err := x.RunFile(d2, d2c); err != nil {
		return err
	}
	if x.lflag {
		x.Printf("#comp %s\t%s\n", d1["path"], d2["path"])
	}
	return x.comp()
}

// like zx.Suffix, but working also for relative paths and returning
// suffixes that can be compared right away.
// (i.e., report suffixes without the leading "/" and  "." as ".")
func relSuffix(name, pref string) string {
	if pref=="" || pref=="." {
		if name == "." {
			return ""
		}
		return name
	}
	suf := zx.Suffix(name, pref)
	if len(suf) > 0 {
		return suf[1:]
	}
	return suf
}

func (x *xCmd) compTree(args ...string) error {
	if len(args) > 2 {
		return errors.New("can't compare more than two files")
	}
	toks := strings.SplitN(args[0], ",", 2)
	name1 := toks[0]
	toks = strings.SplitN(args[1], ",", 2)
	name2 := toks[0]
	x.dprintf("comptree '%s' '%s'\n", name1, name2)
	d1s := []zx.Dir{}
	d1c := cmd.Files(args[0])
	for d1 := range d1c {
		x.dprintf("1: %s\n", d1["path"])
		d1s = append(d1s, d1)
	}
	if cerror(d1c) != nil {
		return cerror(d1c)
	}
	d2s := []zx.Dir{}
	d2c := cmd.Files(args[1])
	for d2 := range d2c {
		x.dprintf("2: %s\n", d2["path"])
		d2s = append(d2s, d2)
	}
	if cerror(d2c) != nil {
		return cerror(d2c)
	}
	i, j := 0, 0
	var sts error
	for i<len(d1s) || j<len(d2s) {
		if i == len(d1s) {
			x.Printf("#created: %s\n", d2s[j]["path"])
			j++
			continue
		}
		if j == len(d2s) {
			x.Printf("#removed: %s\n", d1s[i]["path"])
			i++
			continue
		}
		d1, d2 := d1s[i], d2s[j]
		rel1 := relSuffix(d1["path"], name1)
		rel2 := relSuffix(d2["path"], name2)
		x.dprintf("comp '%s'  '%s'\n", rel1, rel2)
		switch c := zx.Cmp(rel1, rel2); {
		case c < 0:
			x.Printf("#removed: %s\n", d1["path"])
			x.ndiffs++
			i++
		case c > 0:
			x.Printf("#created: %s\n", d2["path"])
			x.ndiffs++
			j++
		default:
			if err := x.compTreeFile(d1, d2); err != nil {
				sts = err
			}
			i++
			j++
		}
	}
	return sts
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("file [file]")
	x.Argv0 = argv[0]
	x.NewFlag("D", "debug", &x.debug)
	x.NewFlag("l", "long listing; print all the file with changes marked", &x.lflag)
	x.dprintf = dbg.FlagPrintf(x.Stderr, &x.debug)
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
	x.h.lines = make(map[string]int)
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
		if err := cmd.RunFiles(x, args...); err != nil {
			return err
		}
		if err := x.comp(); err != nil {
			return err
		}
	} else {
		if err := x.compTree(args...); err != nil {
			return err
		}
	}
	if x.ndiffs > 0 {
		return errors.New("some diffs")
	}
	return nil
}
