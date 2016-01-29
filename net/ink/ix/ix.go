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
	"os/exec"
	"os"
	"strconv"
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
	ln := strings.TrimSpace(ev.Args[1])
	if len(ln) == 0 {
		return
	}
	cmd.Dprintf("exec %s\n", ln)
	argv := strings.Fields(ln)
	x := exec.Command(argv[0], argv[1:]...)
	r, w, err := os.Pipe()
	if err != nil {
		cmd.Warn("pipe: %s", err)
		return
	}
	x.Stdout = w
	x.Stderr = w
	if err := x.Start(); err != nil {
		w.Close()
		cmd.Warn("cmd: %s", err)
		return
	}
	c.win.SetMark("cmd", pos)
	cmd.Warn("cmd: started")
	go func() {
		var buf [4096]byte
		for {
			nr, err := r.Read(buf[0:])
			if nr > 0 {
				s := string(buf[:nr])
				cmd.Warn("out: %s", s)
				c.win.MarkIns("cmd", []rune(s))
			}
			if err != nil {
				break
			}
		}
		err := x.Wait()
		if err != nil {
			cmd.Warn("exit: %s", err)
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
