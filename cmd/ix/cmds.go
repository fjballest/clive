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
	btab["cd"] = bcd
	btab["cmds"] = bcmds
	btab["="] = beq
	btab["w"] = bw
	btab["r"] = br
	btab["d"] = bd
	btab["."] = bdot
	btab[","] = bdot
	btab["x"] = bX
	btab["X"] = bX
}

// NB: All builtins must do a c.ed.win.DelMark(c.mark) once no
// further I/O is expected from them.
// In those that print something and die, they do it before returning.
// In those that fire up commands and accept output from them, their
// io() processes should del the mark when done.
//
// This is the command language:
//	cd dir
//	cmds	// print running commands
//	=	// print dot
//	w	// save
//	r	// undo edits and get from disk
//	d	// delete
//	x	// list edits
//	x expr	// list edits matching expr ("." means dot)
//	x [expr] c	// apply cmd c to dots of matching edits.
//		// where c is any of: = w r d X >... |... <...
//	X [expr] c	// like x expr c, but apply to all the edit text
//	. ...	// like x . ... (apply ... to dot)
//	, ...	// like X . ... (apply ... to all text in dot's edit)
//	>...	// like . > ...
//	< ...	// like . > ...
//	| ...	// like . | ...

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

func bcmds(c *Cmd, args ...string) {
	ed := c.ed
	ix := ed.ix
	var out bytes.Buffer
	ix.Lock()
	if len(ix.cmds) == 0 {
		fmt.Fprintf(&out, "no commands\n")
	}
	for _, c := range ix.cmds {
		p := c.p
		id := 0
		if p != nil {
			id = p.Id
		}
		fmt.Fprintf(&out, "%d\t%s\n", id, c.name)
	}
	ix.Unlock()
	s := out.String()
	c.printf("%s", s)
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
		if err := dot.save(); err == nil {
			c.printf("saved %s\n", dot)
		} else if err != notDirty {
			c.printf("%s: %s\n", dot, err)
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

func bpipeTo(c *Cmd, args ...string) {
	if(args[0][0] == '>') {
		args[0] = args[0][1:]
	}
	if dot := c.ed.ix.dot; dot  != nil {
		go c.pipeTo([]*Ed{dot}, args...)
		return
	}
	c.ed.win.DelMark(c.mark)
}

func bpipeFrom(c *Cmd, args ...string) {
	if(args[0][0] == '<') {
		args[0] = args[0][1:]
	}
	if dot := c.ed.ix.dot; dot  != nil {
		go c.pipeFrom([]*Ed{dot}, args...)
		return
	}
	c.ed.win.DelMark(c.mark)
}

func bpipe(c *Cmd, args ...string) {
	if(args[0][0] == '|') {
		args[0] = args[0][1:]
	}
	if dot := c.ed.ix.dot; dot  != nil {
		go c.pipe(dot, true, args...)
		return
	}
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
	if len(args) > 0 && args[0] != ".*" {
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

func (c *Cmd) pipeEdBytesTo(t *txt.Text, p0, p1 int, asbytes bool) bool {
	var ok bool
	gc := t.Get(p0, p1-p0)
	buf := &bytes.Buffer{}
	p  := c.p
	for rs := range gc {
		for _, r := range rs {
			buf.WriteRune(r)
			if r == '\n' {
				if asbytes {
					ok = p.In <- buf.Bytes()
				} else {
					ok = p.In <- buf.String()
				}
				if !ok {
					c.printf("output: %s\n", cerror(p.In))
					close(gc, cerror(p.In))
					return false
				}
				buf = &bytes.Buffer{}
			}
		}
		if buf.Len() > 0 {
			if asbytes {
				ok = p.In <- buf.Bytes()
			} else {
				ok = p.In <- buf.String()
			}
			if !ok {
				c.printf("output: %s\n", cerror(p.In))
				close(gc, cerror(p.In))
				return false
			}
		}
	}
	return true
}

func (c *Cmd) pipeEdTo(ed *Ed) bool {
	p  := c.p
	d := ed.d.Dup()
	// For the commant, the input is text
	d["type"] = "-"
	if ok := p.In <- d; !ok {
		c.printf("output: %s\n", cerror(p.In))
		return false
	}
	t := ed.win.GetText()
	defer ed.win.UngetText()
	if c.all {
		return c.pipeEdBytesTo(t, 0, t.Len(), true)
	}
	if ed.dot.P1 == ed.dot.P0 {
		return true
	}
	p0, p1 := ed.dot.P0, ed.dot.P1
	if p0 > 0 {
		if !c.pipeEdBytesTo(t, 0, ed.dot.P0, false) {
			return false
		}
	}
	if p1 > p0 {
		if !c.pipeEdBytesTo(t, ed.dot.P0, ed.dot.P1, true) {
			return false
		}
	}
	if p1 < ed.win.Len() {
		if !c.pipeEdBytesTo(t, ed.dot.P1, ed.win.Len(), false) {
			return false
		}
	}
	return true
}

func (c *Cmd) pipeTo(eds []*Ed, args ...string) {
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
			if !c.pipeEdTo(ed) {
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

func (c *Cmd) pipeFrom(eds []*Ed, args ...string) {
	for _, ed := range eds {
		c.pipe(ed, false, args...)
	}
}

func (c *Cmd) pipe(ed *Ed, sendin bool, args ...string) {
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
			c.pipeEdTo(ed)
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
		if c.all {
			ed.dot.P0 = 0
			ed.dot.P1 = ed.win.Len()
		}
		ed.replDot(s)
		c.ed.win.DelMark(c.mark)
		if n := ed.ix.delCmd(c); n == 0 && ed.gone {
			close(ed.waitc)
		}
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
		go c.pipeTo(eds, args[1:]...)
	case "<":
		go c.pipeFrom(eds, args[1:]...)
	case "|":
		go func() {
			for _, ed := range eds {
				c.pipe(ed, true, args[1:]...)
			}
		}()
	case "w":
		for _, ed := range eds {
			if err := ed.save(); err == nil {
				c.printf("%s saved\n", ed)
			} else if err != notDirty {
				c.printf("%s: %s\n", ed, err)
			}
		}
	case "r":
		go func() {
			for _, ed := range eds {
				if err := ed.load(); err == nil {
					c.printf("%s\n", ed)
				} else {
					c.printf("%s: %s\n", ed, err)
				}
			}
		}()
	default:
		cmd.Warn("edit: %q not implemented", args[0])
	}
}

func bX(c *Cmd, args ...string) {
	var out bytes.Buffer
	if len(args) > 1 {
		isio := strings.ContainsRune("|><", rune(args[1][0]))
		iscmd := args[1] == "r" ||
			args[1] == "w" || args[1] == "=" ||
			 args[1] == "d" || args[1] == "X"
		if isio || iscmd {
			args = append([]string{args[0], ".*"}, args[1:]...)
		}
	}
	eds := ix.edits(args[1:]...)

	c.all = args[0] == "X"
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

// "." -> "x ."
// "," -> "X ."
func bdot(c *Cmd, args ...string) {
	if args[0] == "." {
		args = append([]string{"x"}, args...)
	} else {
		args[0] = "."
		args = append([]string{"X"}, args...)
	}
	bX(c, args...)
}
