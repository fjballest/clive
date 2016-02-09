package main

import (
	"bytes"
	"fmt"
	"clive/cmd"
	"clive/cmd/run"
	"clive/sre"
	"clive/txt"
	"strings"
	"io"
	"clive/ch"
	"time"
)

var (
	btab = map[string] func(*Cmd, ...string) {}
)

func init() {
	btab["cmds"] = bcmds
	btab["X"] = bX
	btab["cd"] = bcd
	btab["="] = beq
	btab["w"] = bw
	btab["r"] = br
	btab["d"] = bd
}

func builtin(arg0 string) func(*Cmd, ...string) {
	if arg0 == "" {
		return nil
	}
	if fn, ok := btab[arg0]; ok {
		return fn
	}
	switch arg0[0] {
	case '>':
		return bpipeTo
	case '<':
		return bpipeFrom
	case '|':
		return bpipe
	}
	return nil
}

func bpipeTo(c *Cmd, args ...string) {
	if(args[0][0] == '>') {
		args[0] = args[0][1:]
	}
	if dot := c.ed.ix.dot; dot  != nil {
		go c.pipeTo([]*Ed{dot}, false, args...)
		return
	}
	c.ed.win.DelMark(c.mark)
}

func bpipeFrom(c *Cmd, args ...string) {
	if(args[0][0] == '<') {
		args[0] = args[0][1:]
	}
	if dot := c.ed.ix.dot; dot  != nil {
		go c.pipeFrom([]*Ed{dot}, false, args...)
		return
	}
	c.ed.win.DelMark(c.mark)
}

func bpipe(c *Cmd, args ...string) {
	if(args[0][0] == '|') {
		args[0] = args[0][1:]
	}
	if dot := c.ed.ix.dot; dot  != nil {
		go c.pipe(dot, true, false, args...)
		return
	}
	c.ed.win.DelMark(c.mark)
}

func beq(c *Cmd, args ...string) {
	if dot := c.ed.ix.dot; dot  != nil {
		c.printf("%s\n", dot.Addr());
	}
	c.ed.win.DelMark(c.mark)
}

func bw(c *Cmd, args ...string) {
	if dot := c.ed.ix.dot; dot  != nil {
		if dot.save() {
			c.printf("saved %s\n", dot)
		}
	}
	c.ed.win.DelMark(c.mark)
}

func br(c *Cmd, args ...string) {
	if dot := c.ed.ix.dot; dot  != nil {
		d, err := cmd.Stat(dot.tag)
		if err != nil {
			cmd.Warn("%s: look: %s", dot, err)
		} else {
			dot.d = d
		}
		go dot.load()
	}
	c.ed.win.DelMark(c.mark)
}

func bd(c *Cmd, args ...string) {
	if dot := c.ed.ix.dot; dot  != nil && dot != c.ed {
		if dot.win != nil {
			dot.win.Close()
		} else {
			dot.ix.delEd(dot)
		}
	}
	c.ed.win.DelMark(c.mark)
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
	d := ed.d.Dup()
	// For the commant, the input is text
	d["type"] = "-"
	if ok := p.In <- d; !ok {
		c.printf("output: %s\n", cerror(p.In))
		return false
	}
	buf := &bytes.Buffer{}
	t := ed.win.GetText()
	defer ed.win.UngetText()
	var gc <-chan []rune
	if all {
		gc = t.Get(0, txt.All)
	} else if ed.dot.P1 == ed.dot.P0 {
		return true
	} else {
		gc = t.Get(ed.dot.P0, ed.dot.P1-ed.dot.P0)
	}
	for rs := range gc {
		for _, r := range rs {
			buf.WriteRune(r)
			if r == '\n' {
				if ok := p.In <- buf.Bytes(); !ok {
					c.printf("output: %s\n", cerror(p.In))
					close(gc, cerror(p.In))
					return false
				}
				buf = &bytes.Buffer{}
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

func (c *Cmd) getOut(w io.Writer, donec chan bool) {
	cmd.Dprintf("getOut started\n")
	defer cmd.Dprintf("getOut terminated\n")
	p := c.p
	for m := range ch.GroupBytes(p.Out, time.Second, 4096) {
		switch m := m.(type) {
		case error:
			c.printf("%s\n", m)
		case []byte:
			cmd.Dprintf("ix cmd out: [%d] bytes\n", len(m))
			w.Write(m)
		default:
			cmd.Dprintf("ix cmd out: got type %T\n", m)
		}
	}
	donec <- true
}

func (c *Cmd) getErrs(donec chan bool) {
	cmd.Dprintf("getErrs started\n")
	defer cmd.Dprintf("getErrs terminated\n")
	p := c.p
	for m := range ch.GroupBytes(p.Err, time.Second, 4096) {
		switch m := m.(type) {
		case error:
			c.printf("%s\n", m)
		case []byte:
			cmd.Dprintf("ix cmd err: [%d] bytes\n", len(m))
			s := string(m)
			c.printf("%s\n", s)
		default:
			cmd.Dprintf("ix cmd out: got type %T\n", m)
		}
	}
	donec <- true
}

func (ed *Ed) replDot(s string) {
	some := false
	if ed.dot.P1 > ed.dot.P0 {
		some = true
		ed.win.Del(ed.dot.P0, ed.dot.P1-ed.dot.P0)
	}
	rs := []rune(s)
	if len(rs) > 0 {
		some = true
		ed.win.ContdEdit()
		ed.win.Ins(rs, ed.dot.P0)
	}
	if some {
		ed.dot.P1 = ed.dot.P0 + len(rs)
		// sets p0 and p1 marks
		ed.win.SetSel(ed.dot.P0, ed.dot.P1)
	}
}

func (c *Cmd) pipeFrom(eds []*Ed, all bool, args ...string) {
	for _, ed := range eds {
		c.pipe(ed, false, all, args...)
	}
}

func (c *Cmd) pipe(ed *Ed, sendin, all bool, args ...string) {
	// we ignore all for pipeFrom, so it always replaces the dot.
	// it's not ignored for pipeTo, so the input may be dot or all the file
	inkc := make(chan  face{})
	setio := func(c *cmd.Ctx) {
		c.ForkEnv()
		c.ForkNS()
		c.ForkDot()
		c.SetOut("ink", inkc)
	}
	cmd.Dprintf("pipe from %s\n", args)
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
	var buf bytes.Buffer
	donec := make(chan bool, 2)
	go c.getOut(&buf, donec)
	go c.getErrs(donec)
	go c.inkio(inkc)
	go func() {
		if sendin {
			c.pipeEdTo(ed, all)
		}
		close(p.In)
	}()
	go func() {
		<-donec
		<-donec
		if err := p.Wait(); err != nil {
			cmd.Dprintf("ix cmd exit sts: %s\n", err)
			c.printf("cmd error: %s\n", err)
		}
		s := buf.String()
		cmd.Dprintf("pipe output %q\n", s)
		ed.replDot(s)
		c.ed.win.DelMark(c.mark)
		c.ed.ix.delCmd(c)
	}()
}


func (c *Cmd) edcmd(eds []*Ed, args ...string) {
	switch args[0] {
	case "d":
		for _, ed := range eds {
			if ed.win != nil {
				ed.win.Close()
			} else {
				ed.ix.delEd(ed)
			}
		}
		c.ed.win.DelMark(c.mark)
	case "=":
		var buf bytes.Buffer
		for _, ed := range eds {
			fmt.Fprintf(&buf, "%s\n", ed.Addr());
		}
		if buf.Len() > 0 {
			c.printf("%s\n", buf.String())
		}
		c.ed.win.DelMark(c.mark)

	case ">":
		go c.pipeTo(eds, true, args[1:]...)
	case "<":
		go c.pipeFrom(eds, true, args[1:]...)
	case "|":
		go func() {
			for _, ed := range eds {
				c.pipe(ed, true, true, args[1:]...)
			}
		}()
	case "w":
		for _, ed := range eds {
			if ed.save() {
				c.printf("%s saved\n", ed)
			}
		}
	case "r":
		go func() {
			for _, ed := range eds {
				ed.load()
			}
		}()
	default:
		cmd.Warn("edit: %q not implemented", args[0])
	}
}

// TODO: We could add a W (win) command reporting all elements in the page,
// but then we must record tags for them in ix and make edits() accept a flag
// to report also those.
// The implementation should be bX, but taking care than only the D subcommand
// and perhaps X (xerox) make sense for non-edit windows.

func bX(c *Cmd, args ...string) {
	var out bytes.Buffer
	eds := ix.edits(args[1:]...)
	if len(args) < 3 || len(args[2]) == 0 {
		for _, e := range eds {
			fmt.Fprintf(&out, "%s%s\n", e.menuLine(), e.dot)
		}
		if out.Len() == 0 {
			fmt.Fprintf(&out, "none\n")
		}
		c.printf("%s", out.String())
		c.ed.win.DelMark(c.mark)
		return
	}
	isio := strings.ContainsRune("|><", rune(args[2][0]))
	if args[2] != "r" && args[2] != "w" && args[2] != "=" && args[2] != "d" && args[2] != "X" && !isio {
		c.printf("unknown edit command %q\n", args[2])
		c.ed.win.DelMark(c.mark)
		return
	}
	if isio {
		a2 := args[2][:1]
		args[2] = args[2][1:]
		args = append(args, "")
		copy(args[3:], args[2:])
		args[2] = a2
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
