// +install gx gg gv

/*
	grep in input
*/
package main

import (
	"bytes"
	"clive/cmd"
	"clive/cmd/opt"
	"clive/sre"
	"clive/zx"
	"unicode/utf8"
	"sort"
	"fmt"
)

type rgRep struct {
	name   string
	p0, p1 int
	b      bytes.Buffer
}

var (
	opts = opt.New("rexp [rexp]")
	found   bool
	re, ere *sre.ReProg
	out     chan<- interface{}

	sflag, aflag, mflag, vflag, fflag, lflag, xflag, eflag bool
)

// update ql/builtin.go bltin table if new aliases are added or some are removed.
var alias = map[string]string{
	"gg": "-xef",
	"gv": "-xvef",
	"gx": "-xf",
}

func aliases() {
	c := cmd.AppCtx()
	if len(c.Args) == 0 {
		return
	}
	if v, ok := alias[c.Args[0]]; ok {
		// old argv0 + "-aliased flags"  + "all other args"
		nargs := []string{c.Args[0]}
		c.Args[0] = v
		c.Args = append(nargs, c.Args...)
	}
}

func aliasUsage() string {
	var names []string
	for k := range alias {
		names = append(names, k)
	}
	sort.Sort(sort.StringSlice(names))
	out := ""
	for _, n := range names {
		out += fmt.Sprintf("\t%s is %s %s\n", n, "gr", alias[n])
	}
	return out
}

func rgreport(rg *rgRep) {
	if rg == nil {
		return
	}
	var err error
	switch {
	case sflag:
	case lflag:
		_, err = cmd.Printf("%s\n", rg.name)
	case aflag:
		_, err = cmd.Printf("%s:%d,%d\n", rg.name, rg.p0, rg.p1)
	case mflag:
		m := rg.b.String()
		eln := ""
		if len(m) == 0 || m[len(m)-1] != '\n' {
			eln = "\n"
		}
		_, err = cmd.Printf("%s%s", m, eln)
	case xflag:
		out <- zx.Addr{Name: rg.name, Ln0: rg.p0, Ln1: rg.p1}
		_, err = cmd.Printf("%s", rg.b.String())
	default:
		m := rg.b.String()
		eln := ""
		if len(m) == 0 || m[len(m)-1] != '\n' {
			eln = "\n"
		}
		_, err = cmd.Printf("%s:%d,%d:\n%s%s", rg.name, rg.p0, rg.p1, m, eln)
	}
	if err != nil {
		cmd.Exit(err)
	}
}

func report(name string, nln int, s string) {
	var err error
	switch {
	case sflag:
	case lflag:
		_, err = cmd.Printf("%s\n", name)
	case aflag:
		_, err = cmd.Printf("%s:%d\n", name, nln)
	case mflag:
		_, err = cmd.Printf("%s", s)
	case xflag:
		out <- zx.Addr{Name: name, Ln0: nln, Ln1: nln}
		_, err = cmd.Printf("%s", s)
	default:
		_, err = cmd.Printf("%s:%d: %s", name, nln, s)
	}
	if err != nil {
		cmd.Exit(err)
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

func gr(in <-chan interface{}) {
	nln := 0
	ffound := false
	name := "stdin"
	matching := false
	var rg *rgRep
	for m := range in {
		ok := true
		switch d := m.(type) {
		case zx.Dir:
			rgreport(rg)
			rg = nil
			nln = 0
			name = d["Upath"]
			if name == "" {
				name = d["path"]
			}
			ffound = false
			ok = out <- m
		case string:
			nln += nlines(d)
			ok = out <- m
		case []byte:
			s := string(d)
			cmd.Dprintf("matching for <%s>\n", s)
			nln += nlines(s)
			matches := matching
			if ere != nil {
				if matching {
					if ere.ExecStr(s, 0, -1) != nil {
						matching = false
					}
				} else {
					if re.ExecStr(s, 0, -1) != nil {
						matching = true
						matches = true
					}
				}
			} else {
				matches = re.ExecStr(s, 0, -1) != nil
			}
			if matches && vflag || !matches && !vflag {
				rgreport(rg)
				rg = nil
				if xflag {
					ok = out <- s // fwd as a string
					if !ok {
						break;
					}
				}
				continue
			}
			found = true
			if ffound && (sflag || lflag) {
				continue
			}
			ffound = true
			if ere != nil {
				if rg == nil {
					rg = &rgRep{name: name, p0: nln, p1: nln}
				}
				rg.b.WriteString(s)
				rg.p1 = nln
				continue
			}
			report(name, nln, s)
		case zx.Addr:
			if !xflag {
				ok = out <- m
			}
		default:
			ok = out <- m
		}
		if !ok {
			close(in, cerror(out))
		}
	}
	rgreport(rg)
	
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

func freport(name string, rs []rune, rg sre.Range, off int) {
	var err error
	switch {
	case sflag:
	case lflag:
		_, err = cmd.Printf("%s\n", name)
	case aflag:
		_, err = cmd.Printf("%s:#%d,#%d\n", name, rg.P0+off, rg.P1+off)
	case mflag:
		m := match(rs, rg.P0, rg.P1)
		eln := ""
		if len(m) == 0 || m[len(m)-1] != '\n' {
			eln = "\n"
		}
		_, err = cmd.Printf("%s%s", m, eln)
	case xflag:
		out <- zx.Addr{Name: name, P0: rg.P0 + off, P1: rg.P1 + off}
		_, err = cmd.Printf("%s", match(rs, rg.P0, rg.P1))
	default:
		m := match(rs, rg.P0, rg.P1)
		eln := ""
		if len(m) == 0 || m[len(m)-1] != '\n' {
			eln = "\n"
		}
		_, err = cmd.Printf("%s:#%d,#%d:\n%s%s", name, rg.P0+off, rg.P1+off, m, eln)
	}
	if err != nil {
		cmd.Exit(err)
	}
}

func xreport(rs []rune, rg sre.Range) {
	m := match(rs, rg.P0, rg.P1)
	if ok := out <- m; !ok { // fwd as string, not as []byte
		cmd.Exit(cerror(out))
	}
}

type gRange struct {
	sre.Range
	matches bool
}

func matches(rs []rune) []gRange {
	var rgs []gRange
	for off := 0; ; {
		rg := re.ExecRunes(rs, off, -1)
		if rg != nil && ere != nil {
			erg := ere.ExecRunes(rs, rg[0].P1, -1)
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
		if sflag || lflag {
			break
		}
	}
	return rgs
}

func fullgr(in <-chan interface{}) {
	ffound := false
	name := "stdin"
	off := 0
	for m := range in {
		ok := true
		switch d := m.(type) {
		case zx.Dir:
			name = d["Upath"]
			ffound = false
			ok = out <- m
			off = 0
		case string:
			off += utf8.RuneCountInString(d) // Ada people came to Golang?
			ok = out <- d
		case []byte:
			s := string(d)
			cmd.Dprintf("matching for <%s>\n", s)
			rs := []rune(s)
			matches := matches(rs)
			for _, rg := range matches {
			cmd.Dprintf("\tmatch %v\n", rg)
				if vflag && rg.matches || !vflag && !rg.matches {
					if xflag {
						xreport(rs, rg.Range)
					}
					continue
				}
				found = true
				if ffound && (sflag || lflag) {
					break
				}
				ffound = true
				freport(name, rs, rg.Range, off)
			}
			// if there are further isolated dots, the next one must
			// take into account this one in addresses.
			off += len(rs)
		case zx.Addr:
			if !xflag {
				ok = out <- m
			}
		default:
			ok = out <- m
		}
		if !ok {
			close(in, cerror(out))
		}
	}
}

func chkFlags() {
	flgs := []bool{sflag, aflag, mflag, lflag, xflag}
	n := 0
	for _, f := range flgs {
		if f {
			n++
		}
	}
	if n > 1 {
		cmd.Warn("incompatible flags supplied")
		opts.Usage()
	}
}

// Run gr in the current app context.
func main() {
	c := cmd.AppCtx()
	cmd.UnixIO("err")
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("s", "just status", &sflag)
	opts.NewFlag("l", "print just the names of matching files", &lflag)
	opts.NewFlag("a", "print just addresses", &aflag)
	opts.NewFlag("m", "print just matching text", &mflag)
	opts.NewFlag("v", "invert match", &vflag)
	opts.NewFlag("f", "print addresses for matches in full files (like sam)", &fflag)
	opts.NewFlag("x", "print selections for further editing commands", &xflag)
	opts.NewFlag("e", "extend regexps to match all the text", &eflag)
	ux := false
	opts.NewFlag("u", "use unix out", &ux)
	aliases()
	opts.AddUsage(aliasUsage())
	args := opts.Parse()
	if ux {
		cmd.UnixIO("out")
	}
	chkFlags()
	if len(args) == 0 || len(args) > 2 {
		cmd.Warn("wrong number or arguments")
		opts.Usage()
	}
	if eflag {
		for i, a := range args {
			args[i] = `(.|\n)*(` + a + `)(.|\n)*`
		}
	}
	var err error
	re, err = sre.CompileStr(args[0], sre.Fwd)
	if err != nil {
		cmd.Fatal(err)
	}
	if len(args) == 2 {
		ere, err = sre.CompileStr(args[1], sre.Fwd)
		if err != nil {
			cmd.Fatal(err)
		}
	}
	var in <-chan interface{}
	out = cmd.Out("out")
	if !fflag {
		in = cmd.Lines(cmd.In("in"))
		gr(in)
	} else {
		in = cmd.FullFiles(cmd.In("in"))
		fullgr(in)
	}

	if err := cerror(in); err != nil {
		cmd.Fatal(err)
	}
	if !found && !xflag {
		if !sflag {
			cmd.Fatal("no match")
		}
		cmd.Exit("no match")
	}
}
