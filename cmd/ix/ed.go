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
	all bool
}

struct Dot {
	P0, P1 int
}

// edit
struct Ed {
	tag string
	d zx.Dir
	dot Dot
	ix *IX
	win *ink.Txt
	winid string
	markgen int
	temp bool	// don't save, don't ever flag as dirty
}

func (d Dot) String() string {
	return fmt.Sprintf(":#%d,#%d", d.P0, d.P1)
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

func (ed *Ed) replDot(s string) {
	some := false
	t := ed.win.GetText()
	defer ed.win.PutText()
	rs := []rune(s)
	if ed.dot.P1 > ed.dot.P0 {
		t.Del(ed.dot.P0, ed.dot.P1-ed.dot.P0)
	}
	if len(rs) > 0 {
		t.ContdEdit()
		t.Ins(rs, ed.dot.P0)
	}
	ed.win.SetSel(ed.dot.P0, ed.dot.P0+len(rs))
	return
	// This is how we should do it, but it's quite slow
	// Safari takes a very long time to post the ins events
	// perhaps because we take some time in js to process
	// them, although safari delay is like 30s (!!) and
	// we take just a bit of time.
	// It seems that a plain reload is a lot faster, because it
	// just adds the data as it comes to the lines array in js
	// and then updates everything.
	if ed.dot.P1 > ed.dot.P0 {
		some = true
		ed.win.Del(ed.dot.P0, ed.dot.P1-ed.dot.P0)
	}
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

func (ed *Ed) newMark(pos int) string {
	ed.markgen++
	m := fmt.Sprintf("cmd%d", ed.markgen)
	ed.win.SetMark(m, pos)
	return m
}

func (ed *Ed) Addr() zx.Addr {
	ln0, ln1 := ed.win.LinesAt(ed.dot.P0, ed.dot.P1)
	return zx.Addr{
		Name: ed.tag,
		Ln0: ln0,
		Ln1: ln1,
		P0: ed.dot.P0,
		P1: ed.dot.P1,
	}
}

func (ed *Ed) SetAddr(a zx.Addr) {
	p0, p1 := a.P0, a.P1
	if a.Ln0 != 0 && a.Ln1 != 0 && p0 == 0 && p1 == 0 {
		p0, p1 = ed.win.LinesOff(a.Ln0, a.Ln1)
	}
	ed.dot.P0 = p0
	ed.dot.P1 = p1
	cmd.Dprintf("%s: dot set to %s (%s) for %s\n", ed, ed.dot, ed.Addr(), a)
	ed.win.SetSel(p0, p1)
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
	if err := c.ed.win.MarkIns(c.mark, []rune(s)); err != nil {
		cmd.Warn("mark ins: %s", err)
	}
}

func (c *Cmd) io(hasnl bool) {
	cmd.Dprintf("io started\n")
	defer cmd.Dprintf("io terminated\n")
	p := c.p
	ed := c.ed
	haderrors := false
	_ = time.Second
	cmd.Warn("merge... %v", time.Now())
	first := true
	for m := range ch.Merge(p.Out, p.Err) {
		switch m := m.(type) {
		case error:
			haderrors = true
		case []byte:
			cmd.Dprintf("ix cmd io: [%d] bytes\n", len(m))
			s := string(m)
			c.printf("%s", s)
		case zx.Dir:
			c.printf("%s\n", m.Fmt())
			first = true
		case zx.Addr:
			c.printf("%s\n", m)
			if first {
				if ed = ix.editFor(m.Name); ed != nil {
					ed.win.Show()
					ed.SetAddr(m)
				}
			}
			first = false
		default:
			cmd.Dprintf("ix cmd io: got type %T\n", m)
		}
	}
	cmd.Warn("wait...%v", time.Now())
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
	if b := builtin(args[0]); b != nil {
		cmd.Warn("run: %s", args)
		b(c, args...)
		// We don't del the output mark for builtins,
		// Some will keep bg processes and their io()
		// procs will del their marks,
		// Those who don't, del the marks before they return
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
		ed.ix.lookFile(d["path"], "")
	}
}

func (ed *Ed) look(what string) {
	s := strings.TrimSpace(what)
	names := strings.SplitN(s, ":", 2)
	d, err := cmd.Stat(names[0])
	if err == nil {
		names[0] = d["path"]
		// It's a file
		if len(names) == 1 {
			names = append(names, "")
		} else {
			names[1] = ":" + names[1]
		}
		cmd.Dprintf("look file %q %q\n", names[0], names[1])
		ed.ix.lookFile(names[0], names[1])
		return
	}
	toks := strings.Split(s, "|")
	uri, err := url.Parse(toks[0])
	if err == nil && uri.IsAbs() {
		cmd.Dprintf("look url %q\n", s)
		ed.ix.lookURL(s)
		return
	}
	cmd.Dprintf("look files %q\n", s)
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
		case "focus":
			ed.ix.dot = ed
		case "tick":
			if p0 := ed.win.Mark("p0"); p0 != nil {
				ed.dot.P0 = p0.Off
			}
			if p1 := ed.win.Mark("p1"); p1 != nil {
				ed.dot.P1 = p1.Off
			}
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
		case "clear":
			ed.clear()
		}
	}
	cmd.Dprintf("%s terminated\n", ed)
	ed.ix.delEd(ed)
}

func (ed *Ed) clear() {
	ed.win.SetSel(0, 0)
	t := ed.win.GetText()
	defer ed.win.PutText()
	t.DelAll()
	t.Ins([]rune("\n"), 0)
	t.DropEdits()
}

func (ed *Ed) save() bool {
	s := ed.win.IsDirty()
	if ed.win.IsDirty() {
		// XXX: save it
	}
	ed.win.Clean()
	return s
}

func (ed *Ed) load() {
	what := ed.tag
	t := ed.win.GetText()
	defer ed.win.PutText()
	if t.Len() > 0 {
		t.DelAll()
	}
	t.DropEdits()
	var dc <-chan []byte
	if ed.d["type"] == "d" {
		ed.temp = true
		c := make(chan []byte)
		dc = c
		go func() {
			ds, err := cmd.GetDir(what)
			for _, d := range ds {
				c <- []byte(d.Fmt()+"\n")
			}
			close(c, err)
		}()
	} else {
		dc = cmd.Get(what, 0, -1)
	}
	for m := range dc {
		runes := []rune(string(m))
		t.ContdEdit()
		if err := t.Ins(runes, t.Len()); err != nil {
			close(dc, err)
			cmd.Warn("%s: insert: %s", what, err)
		}
	}
	if err := cerror(dc); err != nil {
		cmd.Warn("%s: get: %s", what, err)
	}
	ed.win.Clean()
}

func (ed *Ed) editLoop() {
	cmd.Dprintf("%s started\n", ed)
	c := ed.win.Events()
	for ev := range c {
		ev := ev
		cmd.Dprintf("ix ev %v\n", ev)
		switch ev.Args[0] {
		case "focus":
			ed.ix.dot = ed
		case "tick":
			if p0 := ed.win.Mark("p0"); p0 != nil {
				ed.dot.P0 = p0.Off
			}
			if p1 := ed.win.Mark("p1"); p1 != nil {
				ed.dot.P1 = p1.Off
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
			ed.save()
		case "quit":
			ed.ix.delEd(ed)
			cmd.Dprintf("%s terminated\n", ed)
			close(c, "quit")
			return
		case "clear":
			ed.clear()
		}
	}
	cmd.Dprintf("%s terminated\n", ed)
	ed.ix.delEd(ed)
}
