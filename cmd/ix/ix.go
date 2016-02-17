/*
	Ink exec.
	A shell for clive using ink.
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/net/ink"
	"clive/zx"
	fpath "path"
	"strings"
	"sync"
)

struct IX {
	pg   *ink.Pg
	eds  []*Ed
	dot  *Ed
	cmds []*Cmd
	addrs []zx.Addr
	sync.Mutex
	idgen int
}

var (
	ix     *IX
	dryrun bool
)

func newIX() *IX {
	ix := &IX{}
	cmds := ix.newCmds()
	cmds.d = zx.Dir{
		"type": "-",
		"path": cmds.tag,
		"name": fpath.Base(cmds.tag),
	}
	col1 := []face{}{cmds.win}
	col2 := []face{}{}
	ix.pg = ink.NewColsPg("/", col1, col2)
	ix.pg.Tag = "IX"
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
					icmds := ix.newCmds()
					icmds.d = zx.Dir{
						"type": "-",
						"path": icmds.tag,
						"name": fpath.Base(icmds.tag),
					}
					icmds.winid, _ = ix.pg.Add(icmds.win)
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
	for _, e := range ix.eds {
		if fpath.Clean(e.tag) == fpath.Clean(what) {
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

func (ix *IX) lookFile(file, addr string) *Ed {
	file = cmd.AbsPath(strings.TrimSpace(file))
	var ed *Ed
	if ed = ix.editFor(file); ed != nil {
		ed.win.Show()
	} else {
		ed = ix.editFile(file)
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

func (ix *IX) editFile(what string) *Ed {
	d, err := cmd.Stat(what)
	if err != nil {
		cmd.Warn("%s: look: %s", what, err)
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
	ed.d = d
	ed.load()	// sets temp
	ed.winid, _ = ix.pg.Add(ed.win)
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

func main() {
	opts := opt.New("{file}")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("I", "debug ink", &ink.Debug)
	opts.NewFlag("n", "dry run (don't ever save)", &dryrun)
	cmd.UnixIO()
	args := opts.Parse()
	ix = newIX()
	done := make(chan bool)
	go ink.Serve()
	go func() {
		ix.loop()
		close(done)
	}()
	if len(args) > 0 {
		ds := cmd.Dirs(args...)
		for m := range ds {
			if err, ok := m.(error); ok {
				cmd.Warn("%s", err)
				continue
			}
			if d, ok := m.(zx.Dir); ok {
				ix.lookFile(d["path"], "")
			}
		}
	}
	<-done
}
