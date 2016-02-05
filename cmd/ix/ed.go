package main

import (
	"fmt"
	"clive/ch"
	"clive/cmd"
	"clive/cmd/run"
	"clive/net/ink"
	"time"
	"strings"
	"strconv"
	"net/url"
	"clive/zx"
	fpath "path"
)

// command run within an edit
struct Cmd {
	ed *Ed
	name string
	mark string
	hasnl bool
	p *run.Proc
}

// edit
struct Ed {
	tag string
	ix *IX
	win *ink.Txt
	winid string
	markgen int
	temp bool	// don't save, don't ever flag as dirty
}

func (ix *IX) delEd(ed *Ed) {
	ix.Lock()
	defer ix.Unlock()
	if ix.dot == ed {
		ix.dot = nil
	}
	for i, e := range ix.eds {
		if e == ed {
			copy(ix.eds[i:], ix.eds[i+1:])
			ix.eds = ix.eds[:len(ix.eds)-1]
			ix.pg.Del(ed.winid)
			return
		}
	}
}

func (ix *IX) addCmd(c *Cmd) {
	ix.Lock()
	defer ix.Unlock()
	ix.cmds = append(ix.cmds, c)
}

func (ix *IX) delCmd(c *Cmd) {
	ix.Lock()
	defer ix.Unlock()
	for i, e := range ix.cmds {
		if e == c {
			copy(ix.cmds[i:], ix.cmds[i+1:])
			ix.cmds = ix.cmds[:len(ix.cmds)-1]
			return
		}
	}
}

func (ix *IX) newEd(tag string) *Ed {
	win := ink.NewTxt();
	win.SetTag(tag)
	ed := &Ed{win: win, ix: ix, tag: tag}
	return ed
}

func (ix *IX) newCmds() *Ed {
	tag := fmt.Sprintf("/ql/%d", ix.newId())
	ed := ix.newEd(tag)
	ed.temp = true
	ix.Lock()
	defer ix.Unlock()
	ix.eds = append(ix.eds, ed)
	cmd.New(ed.cmdLoop)
	return ed
}

func (ix *IX) newEdit(path string) *Ed {
	ed := ix.newEd(path)
	ix.Lock()
	defer ix.Unlock()
	ix.eds = append(ix.eds, ed)
	cmd.New(func(){
		cmd.ForkDot()
		cmd.Cd(fpath.Dir(ed.tag))
		cmd.Dprintf("edit %s dot %s\n", ed.tag, cmd.Dot())
		ed.editLoop()
	})
	return ed
}

func (ed *Ed) String() string {
	return ed.win.Tag()
}

func (ed *Ed) newMark(pos int) string {
	ed.markgen++
	m := fmt.Sprintf("cmd%d", ed.markgen)
	ed.win.SetMark(m, pos)
	return m
}

func (c *Cmd) printf(f string, args ...interface{}) {
	// XXX: TODO: if the win has no views, we must add
	// a new view to show the output from the command.
	// Or else, we might stop the command.
	s := fmt.Sprintf(f, args...)
	if (!c.hasnl) {
		s = "\n" + s
		c.hasnl = true
	}
	c.ed.win.MarkIns(c.mark, []rune(s))
}

func (c *Cmd) io(hasnl bool) {
	cmd.Dprintf("io started\n")
	defer cmd.Dprintf("io terminated\n")
	p := c.p
	ed := c.ed
	haderrors := false
	for m := range ch.GroupBytes(ch.Merge(p.Out, p.Err), time.Second, 4096) {
		switch m := m.(type) {
		case error:
			haderrors = true
		case []byte:
			cmd.Dprintf("ix cmd io: [%d] bytes\n", len(m))
			s := string(m)
			c.printf("%s", s)
		case zx.Dir:
			c.printf("%s\n", m.Fmt())
		default:
			cmd.Dprintf("ix cmd io: got type %T\n", m)
		}
	}
	if err := p.Wait(); err != nil {
		if !haderrors {
			cmd.Dprintf("ix cmd exit sts: %s\n", err)
			c.printf("cmd error: %s\n", err)
		}
	}
	ed.win.DelMark(c.mark)
	ed.ix.delCmd(c)
}

func (c *Cmd) inkio(inkc <-chan face{}) {
	cmd.Dprintf("inkio started\n")
	defer cmd.Dprintf("inkio terminated\n")
	nb := 0
	for m := range inkc {
		m, ok := m.([]byte)
		if !ok {
			continue
		}
		s := string(m)
		cmd.Dprintf("got ink %s\n", s)
		if strings.HasPrefix(s, "http") {
			nb++
			u := fmt.Sprintf("%s|/ink/%s/%d", s, c.name, nb)
			go c.ed.look(u)
			continue
		}
		c.ed.ix.pg.Add(ink.Html(string(m)))
	}
}

func (ed *Ed) runCmd(at int, line string) {
	cmd.Dprintf("run cmd %s at %d\n", line, at)
	hasnl := len(line) > 0 && line[len(line)-1] == '\n'
	ln := strings.TrimSpace(line)
	if len(ln) == 0 {
		return
	}
	args := strings.Fields(ln)
	c := &Cmd{
		name: args[0],
		ed: ed,
		mark: ed.newMark(at),
		hasnl: hasnl,
	}
	if b := bltin[args[0]]; b != nil {
		cmd.Warn("run: %s", args)
		b(c, args...)
		ed.win.DelMark(c.mark)
		return
	}
	args = append([]string{"ql", "-uc"}, args...)
	inkc := make(chan  face{})
	setio := func(c *cmd.Ctx) {
		c.ForkEnv()
		c.ForkNS()
		c.ForkDot()
		c.SetOut("ink", inkc)
	}
	p, err := run.CtxCmd(setio, args...)
	if err != nil {
		cmd.Warn("run: %s", err)
		c.printf("error: %s\n", err)
		ed.win.DelMark(c.mark)
		return
	}
	c.p = p
	ed.ix.addCmd(c)
	go c.io(hasnl)
	go c.inkio(inkc)
}

func (ed *Ed) lookFiles(name string) {
	dc := cmd.Dirs(name)
	for d := range dc {
		d, ok := d.(zx.Dir)
		if !ok || d["type"] != "-" {
			continue
		}
		ed.ix.lookFile(d["path"])
	}
}

func (ed *Ed) look(what string) {
	cmd.Dprintf("look for %q\n", what)
	s := strings.TrimSpace(what)
	_, err := cmd.Stat(s)
	if err == nil {
		// It's a file
		ed.ix.lookFile(s)
	}
	toks := strings.Split(s, "|")
	uri, err := url.Parse(toks[0])
	if err == nil && uri.IsAbs() {
		ed.ix.lookURL(s)
	}
	ed.lookFiles(s)
}

func (ed *Ed) click24(ev *ink.Ev) {
	if len(ev.Args) < 4 {
		cmd.Warn("edit: short click2 event")
		return
	}
	pos, err := strconv.Atoi(ev.Args[3])
	if err != nil {
		cmd.Warn("bad p1 in click2 event")
		return
	}
	if ev.Args[0] == "click2" {
		go ed.runCmd(pos, ev.Args[1])
	} else {
		go ed.look(ev.Args[1])
	}
}

func (ed *Ed) cmdLoop() {
	cmd.ForkDot()
	cmd.ForkNS()
	cmd.ForkEnv()
	cmd.Dprintf("%s started\n", ed)
	c := ed.win.Events()
	for ev := range c {
		ev := ev
		switch ev.Args[0] {
		case "click2", "click4":
			ed.click24(ev)
		case "end":
			if len(ed.win.Views()) == 0 {
				cmd.Dprintf("%s w/o views\n", ed)
			}
		case "quit":
			ed.ix.delEd(ed)
			cmd.Dprintf("%s terminated\n", ed)
			close(c, "quit")
			return
		}
	}
	cmd.Dprintf("%s terminated\n", ed)
	ed.ix.delEd(ed)
}

func (ed *Ed) editLoop() {
	cmd.Dprintf("%s started\n", ed)
	c := ed.win.Events()
	for ev := range c {
		ev := ev
		cmd.Dprintf("ix ev %v\n", ev)
		switch ev.Args[0] {
		case "click1":
			if !ed.temp {
				ed.ix.dot = ed
			}
		case "eins", "edel":
			ed.win.Dirty()
		case "click2", "click4":
			ed.click24(ev)
		case "end":
			if len(ed.win.Views()) == 0 {
				cmd.Dprintf("%s w/o views\n", ed)
			}
		case "save":
			// XXX: save it
			ed.win.Clean()
		case "quit":
			ed.ix.delEd(ed)
			cmd.Dprintf("%s terminated\n", ed)
			close(c, "quit")
			return
		}
	}
	cmd.Dprintf("%s terminated\n", ed)
	ed.ix.delEd(ed)
}
