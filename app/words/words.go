/*
	print words in input
*/
package words

import (
	"clive/dbg"
	"clive/app"
	"clive/app/opt"
	"strings"
)

type xCmd {
	*opt.Flags
	*app.Ctx
	ranges  []string
	one     bool
	seps    string
}

func (x *xCmd) words(in, out chan interface{}) {
	doselect {
	case <-x.Sig:
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-in:
		if !ok {
			break
		}
		dat, ok := m.([]byte)
		if !ok {
			app.Dprintf("got %T\n", m)
			out <- m
			continue
		}
		s := string(dat)
		if len(s)>0 && s[len(s)-1]=='\n' {
			s = s[:len(s)-1]
		}
		var words []string
		if x.one {
			if x.seps == "" {
				x.seps = "\t"
			}
			words = strings.Split(s, x.seps)
		} else {
			if x.seps == "" {
				words = strings.Fields(s)
			} else {
				words = strings.FieldsFunc(s, func(r rune) bool {
					return strings.ContainsRune(x.seps, r)
				})
			}
		}
		for _, w := range words {
			if err := app.Printf("%s\n", w); err != nil {
				app.Exits(err)
			}
		}
	}
}

// Run print fields in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("{file}")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("F", "sep: blank character(s) (or string under -1)", &x.seps)
	x.NewFlag("1", "words separated by 1 run of the blank string given to -F", &x.one)
	args, err := x.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}
	if len(args) != 0 {
		in := app.Files(args...)
		app.SetIO(in, 0)
	}
	in := app.Lines(app.In())	// to make sure we don't break a word in recvs.
	x.words(in, app.Out())
	app.Exits(cerror(in))
}
