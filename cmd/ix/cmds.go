package main

import (
	"bytes"
	"fmt"
	"clive/cmd"
	"clive/cmd/run"
	"clive/sre"
	"clive/zx"
	fpath "path"
	"clive/txt"
)

var (
	bltin = map[string] func(*Cmd, ...string) {}
)

func init() {
	bltin["cmds"] = bcmds
	bltin["X"] = bX
	bltin["cd"] = bcd
}

// NB: All builtins must do a c.ed.win.DelMark(c.mark) once no
// further I/O is expected from them.
// In those that print something and die, they do it before returning.
// In those that fire up commands and accept output from them, their
// io() processes should del the mark when done.

func bcmds(c *Cmd, args ...string) {
	ed := c.ed
	ix := ed.ix
	var out bytes.Buffer
	ix.Lock()
	if len(ix.cmds) == 0 {
		fmt.Fprintf(&out, "no commands\n")
	}
	for i, c := range ix.cmds {
		fmt.Fprintf(&out, "%d\t%s\n", i, c.name)
	}
	ix.Unlock()
	s := out.String()
	c.printf("%s", s)
	c.ed.win.DelMark(c.mark)
}

func (ed *Ed) menuLine() string {
	switch {
	case ed.temp:
		return "/ " + ed.tag
	case ed.win.IsDirty():
		return "! " + ed.tag
	default:
		return "- " + ed.tag
	}
}

func (ix *IX) edits(args ...string) []*Ed {
	var eds []*Ed
	ix.Lock()
	defer ix.Unlock()
	if len(args) > 0 && args[0] == "." {
		if ix.dot != nil {
			eds = append(eds, ix.dot)
		}
		return eds
	}
	match := func(s string) bool { return true; }
	if len(args) > 0 {
		x, err := sre.CompileStr(args[0], sre.Fwd)
		if err != nil {
			return nil
		}
		match = func(s string) bool {
			rg := x.ExecStr(s, 0, len(s))
			return len(rg) > 0
		}
	}
	if ix.dot != nil {
		if s := ix.dot.menuLine(); match(s) {
			eds = append(eds, ix.dot)
		}
	}
	for _, e := range ix.eds {
		if e != ix.dot {
			if s := e.menuLine(); match(s) {
				eds = append(eds, e)
			}
		}
	}
	return eds
}

func (c *Cmd) pipeEdTo(ed *Ed, all bool) bool {
	p  := c.p
	d := zx.Dir{
		"type": "-",
		"path": ed.tag,
		"name": fpath.Base(ed.tag),
	}
	if ok := p.In <- d; !ok {
		c.printf("output: %s\n", cerror(p.In))
		return false
	}
	buf := &bytes.Buffer{}
	t := ed.win.GetText()
	defer ed.win.UngetText()
	gc := t.Get(0, txt.All)
	for rs := range gc {
		for _, r := range rs {
			if r == '\n' {
				if ok := p.In <- buf.Bytes(); !ok {
					c.printf("output: %s\n", cerror(p.In))
					close(gc, cerror(p.In))
					return false
				}
				buf = &bytes.Buffer{}
			} else {
				buf.WriteRune(r)
			}
		}
		if buf.Len() > 0 {
			if ok := p.In <- buf.Bytes(); !ok {
				c.printf("output: %s\n", cerror(p.In))
				close(gc, cerror(p.In))
				return false
			}
		}
	}
	return true
}

func (c *Cmd) pipeTo(eds []*Ed, all bool, args ...string) {
	inkc := make(chan  face{})
	setio := func(c *cmd.Ctx) {
		c.ForkEnv()
		c.ForkNS()
		c.ForkDot()
		c.SetOut("ink", inkc)
	}
	cmd.Dprintf("pipe to %s\n", args)
	args = append([]string{"ql", "-uc"}, args...)
	p, err := run.PipeToCtx(setio, args...)
	if err != nil {
		cmd.Warn("run: %s", err)
		c.printf("error: %s\n", err)
		c.ed.win.DelMark(c.mark)
		return
	}
	c.p = p
	c.ed.ix.addCmd(c)
	// c.io dels the cmd mark
	go c.io(false)
	go c.inkio(inkc)
	go func() {
		for _, ed := range eds {
			cmd.Dprintf("pipe %s to %s\n", ed, args)
			if !c.pipeEdTo(ed, all) {
				break
			}
		}
		close(p.In)
	}()
}

func (c *Cmd) edcmd(eds []*Ed, args ...string) {
	switch args[0] {
	case "D":
		for _, ed := range eds {
			if ed.win != nil {
				ed.win.Close()
			} else {
				ed.ix.delEd(ed)
			}
		}
		c.ed.win.DelMark(c.mark)
	case ">":
		c.pipeTo(eds, true, args[1:]...)
	default:
		cmd.Warn("edit: %q not implemented", args[0])
	}
}

func bX(c *Cmd, args ...string) {
	var out bytes.Buffer
	eds := ix.edits(args[1:]...)
	if len(args) < 3 {
		for _, e := range eds {
			fmt.Fprintf(&out, "%s\n", e)
		}
		if out.Len() == 0 {
			fmt.Fprintf(&out, "none\n")
		}
		c.printf("%s", out.String())
		c.ed.win.DelMark(c.mark)
		return
	}
	switch args[2] {
	case "D", "X", "|", ">", "<":
	default:
		c.printf("unknown edit command %q\n", args[2])
		c.ed.win.DelMark(c.mark)
		return
	}
	cmd.Dprintf("edcnd %s", args[2:])
	c.edcmd(eds, args[2:]...)
}

func bcd(c *Cmd, args ...string) {
	defer c.ed.win.DelMark(c.mark)
	if len(args) == 1 {
		c.printf("missing destination dir\n")
		return
	}
	if err := cmd.Cd(args[1]); err != nil {
		c.printf("cd: %s\n", err)
	} else {
		c.printf("dot: %s\n", cmd.Dot())
	}
}
