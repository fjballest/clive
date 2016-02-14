/*
	list files command
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx"
	"fmt"
)

var (
	opts      = opt.New("{file}")
	ux, gflag bool
	printf    = cmd.Printf
)

func main() {
	cmd.UnixIO("err")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("u", "unix IO", &ux)
	opts.NewFlag("g", "get contents", &gflag)
	args := opts.Parse()
	if ux {
		cmd.UnixIO()
	}
	if cmd.Args()[0] == "gf" {
		gflag = true
	}
	if len(args) == 0 {
		args = append(args, ".,1")
	}

	var dc <-chan face{}
	if !gflag {
		dc = cmd.Dirs(args...)
	} else {
		dc = cmd.Files(args...)
	}

	out := cmd.Out("out")
	var err error
	for m := range dc {
		cmd.Dprintf("got %T\n", m)
		switch m := m.(type) {
		case error:
			err = m
			cmd.Warn("%s", m)
			if !ux {
				m := fmt.Errorf("%s: %s", cmd.Args()[0], m)
				if ok := out <- m; !ok {
					close(dc, cerror(out))
				}
			}
		case zx.Dir:
			if !ux {
				if ok := out <- m; !ok {
					close(dc, cerror(out))
				}
			} else {
				printf("%s\n", m.Fmt())
			}
		case []byte:
			if ok := out <- m; !ok {
				close(dc, cerror(out))
			}
		}
	}
	if err := cerror(dc); err != nil {
		if !ux {
			out <- fmt.Errorf("%s: %s", cmd.Args()[0], err)
		}
		cmd.Fatal(err)
	}
	cmd.Exit(err)
}
