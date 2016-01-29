/*
	Ink exec.
	A shell for clive using ink.
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/net/ink"
)

struct Cmds {
	win *ink.Txt
}

func newCmds() *Cmds {
	win := ink.NewTxt("Type your commands here");
	cin := win.Events()
	c := &Cmds{win: win}
	go func() {
		for ev := range cin {
			if ev == nil || len(ev.Args) == 0 {
				continue
			}
			cmd.Dprintf("cmds ev: %v\n", ev.Args)
			switch ev.Args[0] {
			case "click2":
				c.exec(ev)
			}
		}
	}()
	return c
}

func (c *Cmds) exec(ev *ink.Ev) {
	cmd.Dprintf("exec %v\n", ev.Args[1])
}


func main() {
	opts := opt.New("")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	cmd.UnixIO()
	args := opts.Parse()
	if len(args) != 0 {
		opts.Usage()
	}
	cmds := newCmds();
	pg := ink.NewPg("/", cmds.win)
	pg.Tag = "Ink cmds"
	ink.ServeLoginFor("/")
	go ink.Serve(":8181")
	cmds.win.Wait()
}
