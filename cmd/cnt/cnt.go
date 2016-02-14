/*
	cnt command
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx"
	"unicode"
	"unicode/utf8"
)

struct count {
	name string

	msgs, lines, words, runes, bytes int64
}

var (
	lflag, wflag, rflag, bflag, nflag, mflag, aflag, ux bool

	tots []*count
	opts = opt.New("{file}")
)

func isword(r rune) bool {
	iscon := unicode.Is(unicode.Pc, r) || unicode.Is(unicode.Pd, r)
	return !unicode.IsControl(r) && !unicode.IsSpace(r) &&
		(!unicode.IsPunct(r) || iscon)
}

func report(c *count) {
	s := "  " + c.name
	if nflag {
		s = ""
	}
	if mflag {
		cmd.Printf("%8d%s\n", c.msgs, s)
		return
	}
	if lflag {
		cmd.Printf("%8d%s\n", c.lines, s)
		return
	}
	if wflag {
		cmd.Printf("%8d%s\n", c.words, s)
		return
	}
	if rflag {
		cmd.Printf("%8d%s\n", c.runes, s)
		return
	}
	if bflag {
		cmd.Printf("%8d%s\n", c.bytes, s)
		return
	}
	cmd.Printf("%8d %8d %8d %8d %8d%s\n",
		c.msgs, c.lines, c.words, c.runes, c.bytes, s)
}

func add(c *count) {
	tots = append(tots, c)
	if !nflag {
		report(c)
	}
}

func cnt(in <-chan face{}) {
	var c *count
	var saved []byte
	inword := false
	for m := range in {
		switch m := m.(type) {
		case zx.Dir:
			cmd.Dprintf("got %T %s\n", m, m["path"])
			inword = false
			saved = nil
			if c != nil {
				add(c)
			}
			c = &count{name: m["Upath"]}
			if c.name == "" {
				c.name = m["path"]
			}
			if m["type"] == "d" {
				c = nil
			}
			if aflag {
				c.msgs++
			}
		case []byte:
			c.msgs++
			cmd.Dprintf("got %T\n", m)
			if c == nil {
				c = &count{name: "in"}
			}
			c.bytes += int64(len(m))
			if len(saved) > 0 {
				m = append(saved, m...)
				saved = nil
			}
			for len(m) > 0 {
				if !utf8.FullRune(m) {
					saved = m
					break
				}
				r, n := utf8.DecodeRune(m)
				m = m[n:]
				c.runes++
				if unicode.IsSpace(r) {
					inword = false
				} else if !inword {
					inword = true
					c.words++
				}
				if r == '\n' {
					c.lines++
				}
			}
		default:
			if aflag {
				c.msgs++
			}
			cmd.Dprintf("got %T\n", m)
		}
	}
	if c != nil {
		add(c)
	}
}

// Run cnt in the current app context.
func main() {
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("m", "count just msgs", &mflag)
	opts.NewFlag("l", "count just lines", &lflag)
	opts.NewFlag("w", "count just words", &wflag)
	opts.NewFlag("r", "count just runes", &rflag)
	opts.NewFlag("b", "count just bytes", &bflag)
	opts.NewFlag("c", "count just characters", &bflag)
	opts.NewFlag("n", "print just totals", &nflag)
	opts.NewFlag("u", "use unix output", &ux)
	opts.NewFlag("a", "count all messages and not just data msgs", &aflag)
	cmd.UnixIO("err")
	args := opts.Parse()
	if len(args) != 0 {
		cmd.SetIn("in", cmd.Files(args...))
	}
	if ux {
		cmd.UnixIO("out")
	}
	in := cmd.In("in")
	cnt(in)

	tot := &count{name: "total"}
	for i := 0; i < len(tots); i++ {
		tot.msgs += tots[i].msgs
		tot.lines += tots[i].lines
		tot.words += tots[i].words
		tot.runes += tots[i].runes
		tot.bytes += tots[i].bytes
	}
	if len(tots) > 1 || nflag {
		report(tot)
	}
	if err := cerror(in); err != nil {
		cmd.Fatal(err)
	}
}
