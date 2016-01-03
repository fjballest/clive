/*
	print files command
*/
package main

import (
	"clive/ch"
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx"
)

var (
	opts = opt.New("")
	printf = cmd.Printf

	notux, lflag, pflag, nflag, iflag, dflag, aflag, fflag bool
)

func main() {
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("d", "no not print file data", &dflag)
	opts.NewFlag("f", "no not print dir data", &fflag)
	opts.NewFlag("n", "print just names", &nflag)
	opts.NewFlag("p", "print just pahts", &pflag)
	opts.NewFlag("l", "long list for dirs", &lflag)
	opts.NewFlag("i", "print also ignored data", &iflag)
	opts.NewFlag("u", "don't use unix out", &notux)
	opts.NewFlag("a", "print addresses", &aflag)
	args, err := opts.Parse()
	cmd.UnixIO("err")
	if !notux {
		cmd.UnixIO("out")
	}
	if err != nil {
		cmd.Warn("%s", err)
		opts.Usage()
	}
	if len(args) != 0 {
		opts.Usage()
	}
	dc := cmd.IO("in")
	out := cmd.IO("out")
	for m := range dc {
		cmd.Dprintf("got %T\n", m)
		switch m := m.(type) {
		case error:
			if notux {
				if ok := out <- m; !ok {
					close(dc, cerror(out))
				}
				continue
			}
			cmd.Warn("%s", m)
		case zx.Dir:
			if fflag {
				continue
			}
			switch {
			case notux:
				if ok := out <- m; !ok {
					close(dc, cerror(out))
				}
			case nflag:
				printf("%s\n", m["Upath"])
			case pflag:
				printf("%s\n", m["path"])
			case lflag:
				printf("%s\n", m.LongFmt())
			default:
				printf("%s\n", m.Fmt())
			}
		case []byte:
			if dflag {
				continue
			}
			if ok := out <- m; !ok {
				close(dc, cerror(out))
			}
		case ch.Ign:
			if dflag {
				continue
			}
			b := m.Dat
			if ok := out <- b; !ok {
				close(dc, cerror(out))
			}
		case zx.Addr:
			if !aflag {
				continue
			}
			printf("%s\n", m)
		}
	}
	if err := cerror(dc); err != nil {
		cmd.Fatal(err)
	}
}
