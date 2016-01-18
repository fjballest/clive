/*
	print words in input
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"strings"
)

var (
	opts = opt.New("{file}")
	one    bool
	seps   string
)

func words(in <-chan interface{}, out chan<- interface{}) {
	for m := range in {
		dat, ok := m.([]byte)
		if !ok {
			cmd.Dprintf("got %T\n", m)
			if ok := out <- m; !ok {
				close(in, cerror(out))
			}
			continue
		}
		s := string(dat)
		if len(s) > 0 && s[len(s)-1] == '\n' {
			s = s[:len(s)-1]
		}
		var words []string
		if one {
			if seps == "" {
				seps = "\t"
			}
			words = strings.Split(s, seps)
		} else {
			if seps == "" {
				words = strings.Fields(s)
			} else {
				words = strings.FieldsFunc(s, func(r rune) bool {
					return strings.ContainsRune(seps, r)
				})
			}
		}
		for _, w := range words {
			if _, err := cmd.Printf("%s\n", w); err != nil {
				close(in, err)
			}
		}
	}
}

// Run print fields in the current app context.
func main() {
	c := cmd.AppCtx()
	cmd.UnixIO("err")
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("F", "sep: blank character(s) (or string under -1)", &seps)
	opts.NewFlag("1", "words separated by 1 run of the blank string given to -F", &one)
	ux := false
	opts.NewFlag("u", "use unix out", &ux)
	args := opts.Parse()
	if ux {
		cmd.UnixIO("out")
	}
	if len(args) != 0 {
		cmd.SetIn("in", cmd.Files(args...))
	}
	in := cmd.Lines(cmd.In("in")) // to make sure we don't break a word in recvs.
	words(in, cmd.Out("out"))
	if err := cerror(in); err != nil {
		cmd.Fatal(err)
	}
}
