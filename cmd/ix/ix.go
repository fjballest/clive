/*
	Ink exec.
	An ink shell and window system for clive.
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/net/ink"
	"clive/cmd/look"
	"clive/zx"
	fpath "path"
	"strings"
	"sync"
	"fmt"
)

struct IX {
	pg   *ink.Pg
	eds  []*Ed
	dot  *Ed
	cmds []*Cmd
	addrs []zx.Addr
	sync.Mutex
	msgs *Ed	// commands window used to notify the user
	idgen int
	lookstr string
}

var (
	ix     *IX
	rules	look.Rules
	dryrun bool

	defaultRules = `
		^([a-zA-Z.]+)\(([0-9]+)\)$
			doc \2 \1|rf
	`
)

func newIX() *IX {
	ix := &IX{}
	cmds := ix.newCmds(cmd.Dot(), "")
	if cmds == nil {
		cmd.Fatal("can't create command window")
	}
	col1 := []face{}{cmds.win}
	col2 := []face{}{}
	ix.pg = ink.NewColsPg("/", col1, col2)
	ix.pg.Tag = "IX"
	ix.msgs = cmds
	cols := ix.pg.Cols()
	cmds.winid = cols[0][0]
	ix.pg.Cmds = []string{"win", "quit"}
	return ix
}

func (ix *IX) String() string {
	return ix.pg.Tag
}

func (ix *IX) newId() int {
	ix.Lock()
	defer ix.Unlock()
	ix.idgen++
	return ix.idgen
}

var wlock sync.Mutex
var warning bool

func (ix *IX) Warn(fmts string, arg ...interface{}) {
	// ix might be locked and we still warns to find their way...
	go func() {
		wlock.Lock()
		defer wlock.Unlock()
		cmd.Warn(fmts, arg...)
		ix.Lock()
		c := ix.msgs
		ix.Unlock()
		if c == nil {
			c = ix.newCmds(cmd.Dot(), "")
			if c == nil {
				cmd.Warn("can't create commands window")
				return
			}
			c.winid, _ = ix.pg.Add(c.win)
		}
		msg := fmt.Sprintf(fmts+"\n", arg...)
		c.win.Ins([]rune(msg), 0)
	}()
}

func (x *IX) quit() {
}

func (ix *IX) loop() {
	cmd.Dprintf("%s started\n", ix)
	defer cmd.Dprintf("%s terminated\n", ix)
	for ev := range ix.pg.Events() {
		ev := ev
		cmd.Dprintf("%s ev: %v %v\n", ix, ev.Src, ev.Args)
		switch ev.Args[0] {
		case "click2":
			switch ev.Args[1] {
			case "win":
				go func() {
					icmds := ix.newCmds(cmd.Dot(), "")
					if icmds == nil {
						cmd.Warn("can't create commands window")
					} else {
						icmds.winid, _ = ix.pg.Add(icmds.win)
					}
				}()
			case "quit":
				// XXX: MUST save everything here.
				cmd.Fatal("user quit")
			}
		}
	}
}

func (ix *IX) editFor(what string) *Ed {
	ix.Lock()
	defer ix.Unlock()
	what = fpath.Clean(what)
	for _, e := range ix.eds {
		if fpath.Clean(e.tag) == what {
			return e
		}
	}
	return nil
}

func (ix *IX) cmdsAt(dir string) *Ed {
	ix.Lock()
	defer ix.Unlock()
	dir = fpath.Clean(dir)
	for _, e := range ix.eds {
		if e.iscmd && e.dir == dir {
			return e
		}
	}
	return nil
}

func (ix *IX) cleanAddrs() {
	ix.Lock()
	defer ix.Unlock()
	ix.addrs = ix.addrs[:0]
}

func (ix *IX) addAddr(a zx.Addr) {
	ix.Lock()
	defer ix.Unlock()
	ix.addrs = append(ix.addrs, a)
}

func (ix *IX) lookNext(a zx.Addr) {
	for i, na := range ix.addrs {
		cmd.Dprintf("look next %s %s\n", a, na)
		if na.Name == a.Name &&
			((na.Ln0 == a.Ln0 && na.Ln1 == a.Ln1 && a.Ln0 > 0) ||
		   	 (na.P0 == a.P0 && na.P1 == a.P1)) && i < len(ix.addrs)-1 {
			na = ix.addrs[i+1]
			if ed := ix.editFor(na.Name); ed != nil {
				ed.win.Show()
				ed.SetAddr(na)
			}
			break
		}
	}
			
}

func (ix *IX) lookCmds(dir string, at int) *Ed {
	var ed *Ed
	if ed = ix.cmdsAt(dir); ed != nil {
		ed.win.Show()
		return ed
	}
	ed = ix.newCmds(dir, "")
	if ed == nil {
		cmd.Warn("can't create commands window at %s", dir)
		return nil
	}
	ed.winid, _ = ix.pg.AddAt(ed.win, at)
	return nil
}

func (ix *IX) lookFile(file, addr string, at int) *Ed {
	file = cmd.AbsPath(strings.TrimSpace(file))
	var ed *Ed
	if ed = ix.editFor(file); ed != nil {
		ed.win.Show()
	} else {
		ed = ix.editFile(file, at)
	}
	if ed != nil && addr != "" {
		a := zx.ParseAddr(addr)
		a.Name = ed.tag
		ed.SetAddr(a)
	}
	return ed
}

func (ix *IX) lookURL(what string) {
	ix.pg.Add(ink.Url(what))
}

func (ix *IX) editFile(what string, at int) *Ed {
	d, err := cmd.Stat(what)
	if err != nil {
		ix.Warn("%s: look: %s", what, err)
		return nil
	}
	dot := d["path"]
	if d["type"] == "d" {
		what += "/"
	} else {
		dot = fpath.Dir(dot)
	}
	ed := ix.newEdit(what)
	ed.dir = dot
	ed.load(d)	// sets temp
	ed.winid, _ = ix.pg.AddAt(ed.win, at)
	return ed
}

func (ix *IX) winEd(id string) *Ed {
	ix.Lock()
	defer ix.Unlock()
	for _, ed := range ix.eds {
		if ed.winid == id {
			return ed
		}
	}
	return nil
}

func (ix *IX) layout() [][]*Ed {
	pgcols := ix.pg.Cols()
	var cols [][]*Ed
	for _, c := range pgcols {
		var col []*Ed
		for _, el := range c {
			if ed := ix.winEd(el); ed != nil {
				col = append(col, ed)
			}
		}
		cols = append(cols, col)
	}
	return cols
}

func makeRules() error {
	r := cmd.DotFile("look")
	if r == "" {
		r = defaultRules
	}
	rs, err := look.ParseRules(r)
	rules = rs
	return err
}

func main() {
	opts := opt.New("{file}")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("I", "debug ink", &ink.Debug)
	opts.NewFlag("n", "dry run (don't ever save)", &dryrun)
	var dmpf string
	opts.NewFlag("l", "file: load the session from the given file", &dmpf)
	cmd.UnixIO()
	args := opts.Parse()
	look.Debug = c.Debug
	ix = newIX()
	ink.ServeZX()
	done := make(chan bool)
	go func() {
		if err := ink.Serve(); err != nil {
			cmd.Fatal("can't listen")
		}
	}()
	go func() {
		ix.loop()
		close(done)
	}()
	if len(args) > 0 {
		ds := cmd.Dirs(args...)
		for m := range ds {
			if err, ok := m.(error); ok {
				ix.Warn("%s", err)
				continue
			}
			if d, ok := m.(zx.Dir); ok {
				ix.lookFile(d["path"], "", -1)
			}
		}
	}
	err := makeRules()
	if err != nil {
		ix.Warn("rules: %s", err)
	}
	if dmpf != "" {
		if err := ix.load(dmpf); err != nil {
			ix.Warn("load: %s: %s", dmpf, err)
		}
	}
	<-done
}
