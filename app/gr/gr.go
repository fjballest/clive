/*
	grep in input
*/
package gr

import (
	"clive/dbg"
	"clive/app"
	"clive/sre"
	"clive/zx"
	"clive/app/opt"
	"bytes"
	"fmt"
	"unicode/utf8"
)

type xCmd {
	*opt.Flags
	*app.Ctx
	found	bool
	re, ere	*sre.ReProg
	out chan interface{}

	sflag, aflag, mflag, vflag, fflag, lflag, xflag, eflag bool
}

type rgRep {
	name string
	p0, p1 int
	b bytes.Buffer
}

// addresses written by gr under the -x flag
type Addr {
	Name string
	Ln0, Ln1 int	// line range for next output
	P0, P1 int		// point (rune) range for next output
}

func (a Addr) String() string {
	if a.Name == "" {
		a.Name = "stdin"
	}
	if a.Ln0 != 0 || a.Ln1 != 0 {
		return fmt.Sprintf("%s:%d,%d", a.Name, a.Ln0, a.Ln1)
	}
	return fmt.Sprintf("%s:#%d,#%d", a.Name, a.P0, a.P1)
}

func (x *xCmd) rgreport(rg *rgRep) {
	if rg == nil {
		return
	}
	var err error
	switch {
	case x.sflag:
	case x.lflag:
		err = app.Printf("%s\n", rg.name)
	case x.aflag:
		err = app.Printf("%s:%d,%d\n", rg.name, rg.p0, rg.p1)
	case x.mflag:
		m := rg.b.String()
		eln := ""
		if len(m) == 0 || m[len(m)-1] != '\n' {
			eln="\n"
		}
		err = app.Printf("%s%s", m, eln)
	case x.xflag:
		x.out <- Addr{Name: rg.name, Ln0: rg.p0, Ln1: rg.p1}
		err = app.Printf("%s", rg.b.String())
	default:
		m := rg.b.String()
		eln := ""
		if len(m) == 0 || m[len(m)-1] != '\n' {
			eln="\n"
		}
		err = app.Printf("%s:%d,%d:\n%s%s", rg.name, rg.p0, rg.p1, m, eln)
	}
	if err != nil {
		app.Exits(err)
	}
}

func (x *xCmd) report(name string, nln int, s string) {
	var err error
	switch {
	case x.sflag:
	case x.lflag:
		err = app.Printf("%s\n", name)
	case x.aflag:
		err = app.Printf("%s:%d\n", name, nln)
	case x.mflag:
		err = app.Printf("%s", s)
	case x.xflag:
		x.out <- Addr{Name: name, Ln0: nln, Ln1: nln}
		err = app.Printf("%s", s)
	default:
		err = app.Printf("%s:%d: %s", name, nln, s)
	}
	if err != nil {
		app.Exits(err)
	}
}

func nlines(s string) int {
	n := 0
	for _, r := range s {
		if r == '\n' {
			n++
		}
	}
	return n
}

func (x *xCmd) gr(in chan interface{}) {
	nln := 0
	ffound := false
	name := "stdin"
	matching := false
	var rg *rgRep
	doselect {
	case <-x.Sig:
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-in:
		if !ok {
			break
		}
		switch d := m.(type) {
		case zx.Dir:
			x.rgreport(rg)
			rg = nil
			nln = 0
			name = d["upath"]
			ffound = false
			x.out <- m
		case string:
			nln += nlines(d)
			x.out <- m
		case []byte:
			s := string(d)
			nln +=nlines(s)
			matches := matching
			if x.ere != nil {
				if matching {
					if x.ere.ExecStr(s, 0, -1) != nil {
						matching = false
					}
				} else {
					if x.re.ExecStr(s, 0, -1) != nil {
						matching = true
						matches = true
					}
				}
			} else {
				matches = x.re.ExecStr(s, 0, -1) != nil
			}
			if matches && x.vflag || !matches && !x.vflag {
				x.rgreport(rg)
				rg = nil
				if x.xflag {
					x.out <- s	// fwd as a string
				}
				continue
			}
			x.found = true
			if ffound && (x.sflag || x.lflag) {
				continue
			}
			ffound = true
			if x.ere != nil {
				if rg == nil {
					rg = &rgRep{name: name, p0: nln, p1: nln}
				}
				rg.b.WriteString(s)
				rg.p1 = nln
				continue
			}
			x.report(name, nln, s)
		case Addr:
			if !x.xflag {
				if ok := x.out <- m; !ok {
					app.Exits(cerror(x.out))
				}
			}
		default:
			if ok := x.out <- m; !ok {
				app.Exits(cerror(x.out))
			}
		}
	}
	x.rgreport(rg)
}

func okp(p, n int) int {
	if p < 0 {
		return 0
	}
	if p >= n {
		return n
	}
	return p
}

func match(rs []rune, p0, p1 int) string {
	p0 = okp(p0, len(rs))
	p1 = okp(p1, len(rs))
	return string(rs[p0:p1])
}

func (x *xCmd) freport(name string, rs []rune, rg sre.Range, off int) {
	var err error
	switch {
	case x.sflag:
	case x.lflag:
		err = app.Printf("%s\n", name)
	case x.aflag:
		err = app.Printf("%s:#%d,#%d\n", name, rg.P0+off, rg.P1+off)
	case x.mflag:
		m := match(rs, rg.P0, rg.P1)
		eln := ""
		if len(m) == 0 || m[len(m)-1] != '\n' {
			eln="\n"
		}
		err = app.Printf("%s%s", m, eln)
	case x.xflag:
		x.out <- Addr{Name: name, P0: rg.P0+off, P1: rg.P1+off}
		err = app.Printf("%s", match(rs, rg.P0, rg.P1))
	default:
		m := match(rs, rg.P0, rg.P1)
		eln := ""
		if len(m) == 0 || m[len(m)-1] != '\n' {
			eln="\n"
		}
		err = app.Printf("%s:#%d,#%d:\n%s%s", name, rg.P0+off, rg.P1+off, m, eln)
	}
	if err != nil {
		app.Exits(err)
	}
}

func (x *xCmd) xreport(rs []rune, rg sre.Range) {
	m := match(rs, rg.P0, rg.P1)
	x.out <- m	// fwd as string, not as []byte
}

type gRange {
	sre.Range
	matches bool
}

func (x *xCmd) matches(rs []rune) []gRange {
	var rgs []gRange
	for off := 0; ; {
		rg := x.re.ExecRunes(rs, off, -1)
		if rg != nil && x.ere != nil {
			erg := x.ere.ExecRunes(rs, rg[0].P1, -1)
			if erg == nil {
				rg[0].P1 = len(rs)
			} else {
				rg[0].P1 = erg[0].P1
			}
		}
		if rg == nil {
			if off < len(rs) {
				r := gRange{
					Range: sre.Range{P0: off, P1: len(rs)},
				}
				rgs = append(rgs, r)
			}
			break
		}
		if off < rg[0].P0 {
			r := gRange{Range: sre.Range{P0: off, P1: rg[0].P0}}
			rgs = append(rgs, r)
		}
		rgs = append(rgs, gRange{Range: rg[0], matches: true})
		off = rg[0].P1
		if x.sflag || x.lflag {
			break
		}
	}
	return rgs
}

func (x *xCmd) fullgr(in chan interface{}) {
	ffound := false
	name := "stdin"
	off := 0
	doselect {
	case <-x.Sig:
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-in:
		if !ok {
			break
		}
		switch d := m.(type) {
		case zx.Dir:
			name = d["upath"]
			ffound = false
			x.out <- m
			off = 0
		case string:
			off += utf8.RuneCountInString(d)	// Ada people came to Golang?
			x.out <- d
		case []byte:
			s := string(d)
			rs := []rune(s)
			matches := x.matches(rs)
			for _, rg := range matches {
				if x.vflag && rg.matches || !x.vflag && !rg.matches {
					if x.xflag {
						x.xreport(rs, rg.Range)
					}
					continue
				}
				x.found = true
				if ffound && (x.sflag || x.lflag) {
					break
				}
				ffound = true
				x.freport(name, rs, rg.Range, off)
			}
			// if there are further isolated dots, the next one must
			// take into account this one in addresses.
			off += len(rs)
		case Addr:
			if !x.xflag {
				if ok := x.out <- m; !ok {
					app.Exits(cerror(x.out))
				}
			}
		default:
			if ok := x.out <- m; !ok {
				app.Exits(cerror(x.out))
			}
		}
	}
}

func (x *xCmd) chkFlags() {
	flgs := []bool{x.sflag, x.aflag, x.mflag, x.lflag, x.xflag}
	n := 0
	for _, f := range flgs {
		if f {
			n++
		}
	}
	if n > 1 {
		app.Warn("incompatible flags supplied")
		x.Usage()
		app.Exits("usage")
	}
}

// update ql/builtin.go bltin table if new aliases are added or some are removed.
var aliases = map[string]string {
	"gg": "-xef",
	"gv": "-xvef",
	"gx": "-xf",
}

func (x *xCmd) aliases() {
	if len(x.Args) == 0 {
		return
	}
	if v, ok := aliases[x.Args[0]]; ok {
		// old argv0 + "-aliased flags"  + "all other args"
		nargs := []string{x.Args[0]}
		x.Args[0] = v
		x.Args = append(nargs, x.Args...)
	}
}

// Run gr in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("rexp [rexp]")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("s", "just status", &x.sflag)
	x.NewFlag("l", "print just the names of matching files", &x.lflag)
	x.NewFlag("a", "print just addresses", &x.aflag)
	x.NewFlag("m", "print just matching text", &x.mflag)
	x.NewFlag("v", "invert match", &x.vflag)
	x.NewFlag("f", "print addresses for matches in full files (like sam)", &x.fflag)
	x.NewFlag("x", "print selections for further editing commands", &x.xflag)
	x.NewFlag("e", "extend regexps to match all the text", &x.eflag)
	x.aliases()
	args, err := x.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}
	x.chkFlags()
	if len(args) > 2 {
		app.Warn("wrong number or arguments")
		x.Usage()
		app.Exits("usage")
	}
	if x.eflag {
		for i, a := range args {
			args[i] = `(.|\n)*(` + a + `)(.|\n)*`
		}
	}
	x.re, err = sre.CompileStr(args[0], sre.Fwd)
	if err != nil {
		app.Fatal(err)
	}
	if len(args) == 2 {
		x.ere, err = sre.CompileStr(args[1], sre.Fwd)
		if err != nil {
			app.Fatal(err)
		}
	}
	var in chan interface{}
	x.out = app.Out()
	if !x.fflag {
		in = app.Lines(app.In())
		x.gr(in)
	} else {
		in = app.FullFiles(app.In())
		x.fullgr(in)
	}

	if err := cerror(in); err != nil {
		app.Exits(err)
	}
	if !x.found {
		if !x.sflag {
			app.Fatal("no match")
		}
		app.Exits("no match")
	}
	app.Exits(nil)
}
