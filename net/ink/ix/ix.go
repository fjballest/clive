/*
	Ink exec.
	A shell for clive using ink.
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/net/ink"
	"strings"
	"clive/ch"
	"clive/cmd/run"
	"strconv"
	"time"
)

struct Cmds {
	win *ink.Txt
}

func newCmds() *Cmds {
	win := ink.NewTxt("Online and ready");
	cin := win.Events()
	c := &Cmds{win: win}
	go func() {
		for ev := range cin {
			ev := ev
			if ev == nil || len(ev.Args) == 0 {
				continue
			}
			cmd.Dprintf("cmds ev: %v\n", ev.Args)
			switch ev.Args[0] {
			case "click2":
				go c.exec(ev)
			}
		}
	}()
	return c
}

func (c *Cmds) exec(ev *ink.Ev) {
	if len(ev.Args) < 4 {
		cmd.Warn("bad args in click2")
		return
	}
	pos, err := strconv.Atoi(ev.Args[3])
	if err != nil {
		cmd.Warn("bad p1 in click2")
		return
	}
	line := ev.Args[1]
	hasnl := len(line) > 0 && line[len(line)-1] == '\n'
	ln := strings.TrimSpace(line)
	if len(ln) == 0 {
		return
	}
	c.win.SetMark("cmd", pos)
	cmd.Dprintf("exec %s\n", ln)
	x, err := run.UnixCmd(strings.Fields(ln)...)
	if err != nil {
		cmd.Warn("run: %s", err)
		c.win.MarkIns("cmd", []rune("error: " + err.Error() + "\n"));
		return
	}
	go func() {
		for m := range ch.GroupBytes(ch.Merge(x.Out, x.Err), time.Second, 4096) {
			switch m := m.(type) {
			case []byte:
				cmd.Dprintf("-> [%d] bytes\n", len(m))
				s := string(m)
				if !hasnl {
					hasnl = true
					s = "\n" + s
				}
				c.win.MarkIns("cmd", []rune(s))
			default:
				cmd.Dprintf("got type %T\n", m)
			}
		}
		if err := cerror(x.Err); err != nil {
			c.win.MarkIns("cmd", []rune("error: " + err.Error() + "\n"));
		}
		c.win.DelMark("cmd")
	}()
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
