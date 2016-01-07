/*
	translate expressions in input
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/sre"
	"strings"
)

var (
	opts = opt.New("to | {from to}")
	res        []*sre.ReProg
	froms, tos []string
	all        bool

	sflag, fflag, gflag, tflag, lflag, uflag, rflag, xflag bool
)

func replre(s string, re *sre.ReProg, to string, glob bool) string {
	rfrom := []rune(s)
	rto := []rune(to)
	nrefs := 0
	for i := 0; i < len(rto)-1; i++ {
		if nb := rto[i+1]; rto[i] == '\\' {
			if nb >= '0' && nb <= '9' {
				nb -= '0'
				rto[i] = nb
				nrefs++
				n := copy(rto[i+1:], rto[i+2:])
				rto = rto[:i+1+n]
			} else if nb == '\\' {
				n := copy(rto[i+1:], rto[i+2:])
				rto = rto[:i+1+n]
			}
		}
	}
	st := 0
	for {
		cmd.Dprintf("re match [%d:%d]\n", st, len(rfrom))
		rg := re.ExecRunes(rfrom, st, len(rfrom))
		if len(rg) == 0 {
			break
		}
		r0 := rg[0]
		var ns []rune
		ns = append(ns, rfrom[:r0.P0]...)
		if nrefs == 0 {
			ns = append(ns, rto...)
		} else {
			for _, r := range rto {
				if r > 10 {
					ns = append(ns, r)
					continue
				}
				if r < rune(len(rg)) {
					t := rfrom[rg[r].P0:rg[r].P1]
					ns = append(ns, t...)
				}
			}
		}
		st = len(ns)
		ns = append(ns, rfrom[r0.P1:]...)
		rfrom = ns
		if !glob {
			break
		}
	}
	return string(rfrom)
}

func replset(s string, from, to string) string {
	rfrom := []rune(from)
	rto := []rune(to)
	if len(rfrom) != len(rto) && len(rto) > 0 {
		cmd.Fatal("incompatible replacement '%s' to '%s'", from, to)
	}
	rs := []rune(s)
Loop:
	for i := 0; i < len(rs); {
		for n := 0; n < len(rfrom); n++ {
			if n < len(rfrom)-2 && rfrom[n+1] == '-' {
				if rs[i] >= rfrom[n] && rs[i] <= rfrom[n+2] {
					if len(rto) > 0 {
						rs[i] = rto[n] + rs[i] - rfrom[n]
					} else {
						n := copy(rs[i:], rs[i+1:])
						rs = rs[:i+n]
						continue Loop
					}
				}
				n += 2
			} else if rs[i] == rfrom[n] {
				if len(rto) > 0 {
					rs[i] = rto[n]
				} else {
					n := copy(rs[i:], rs[i+1:])
					rs = rs[:i+n]
					continue Loop
				}
			}
		}
		i++
	}
	return string(rs)
}

func trex(in <-chan interface{}) error {
	nrepl := 1
	if gflag {
		nrepl = -1
	}
	out := cmd.IO("out")
	doall := false
	for m := range in {
		ok := true
		switch m := m.(type) {
		case []byte:
			s := string(m)
			cmd.Dprintf("got '%s'\n", s)
			if all {
				doall = true
				continue
			}
			if uflag {
				s = strings.ToUpper(s)
			} else if lflag {
				s = strings.ToLower(s)
			} else if tflag {
				s = strings.ToTitle(s)
			}
			for i := 0; i < len(froms); i++ {
				if res != nil {
					s = replre(s, res[i], tos[i], gflag)
				} else if rflag {
					s = replset(s, froms[i], tos[i])
				} else {
					s = strings.Replace(s, froms[i], tos[i], nrepl)
				}
			}
			cmd.Printf("%s", s)
		default:
			if all && doall {
				doall = false
				out <- []byte(tos[0])
			}
			// ignored
			cmd.Dprintf("got %T\n", m)
			ok = out <- m
		}
		if !ok {
			close(in, cerror(out))
		}
	}
	if all && doall {
		m := []byte(tos[0])
		if ok := out <- m; !ok {
			return cerror(out)
		}
	}
	return cerror(in)
}

// Run trex in the current app context.
func main() {
	c := cmd.AppCtx()
	cmd.UnixIO("err")
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("s", "handle args as strings and not rexps.", &sflag)
	opts.NewFlag("g", "change globally (as many times as can be done per line)", &gflag)
	opts.NewFlag("f", "match expressions in full files", &fflag)
	opts.NewFlag("c", "translate to capitals (upper case)", &uflag)
	opts.NewFlag("l", "translate to lower case", &lflag)
	opts.NewFlag("t", "translate to title case", &tflag)
	opts.NewFlag("r", "interpret replacements as rune sets", &rflag)
	opts.NewFlag("x", "match against each extracted text (eg., out from gr -x)", &xflag)
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
	if len(args) == 1 {
		args = []string{".*", args[0]}
		all = true
	}
	if len(args)%2 != 0 {
		cmd.Warn("wrong number of arguments")
		opts.Usage()
	}
	if rflag && gflag || rflag && sflag || gflag && sflag {
		cmd.Warn("incompatible flags given")
		opts.Usage()
	}
	for i := 0; i < len(args); i += 2 {
		froms = append(froms, args[0])
		tos = append(tos, args[1])
		if !sflag && !rflag {
			re, err := sre.CompileStr(args[0], sre.Fwd)
			if err != nil {
				cmd.Fatal("rexp: %s", err)
			}
			res = append(res, re)
		}
	}
	var sts error
	switch {
	case xflag:
		sts = trex(cmd.IO("in"))
	case fflag:
		sts = trex(cmd.FullFiles(cmd.IO("in")))
	default:
		sts = trex(cmd.Lines(cmd.IO("in")))
	}
	if sts != nil {
		cmd.Fatal(sts)
	}
}
