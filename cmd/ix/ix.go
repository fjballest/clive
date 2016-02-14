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
	ix.pg.Cmds = []string{"win", "dump", "quit"}
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
			case "dump":
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

func (ix *IX) lookFile(file, addr string) *Ed {
	file = cmd.AbsPath(strings.TrimSpace(file))
	var ed *Ed
	if ed = ix.editFor(file); ed != nil {
		ed.win.Show()
	} else {
		ed = ix.editFile(file)
	}
	if ed != nil && addr != "" {
		ed.SetAddr(zx.ParseAddr(addr))
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
	if d["type"] == "d" {
		what += "/"
	}
	ed := ix.newEdit(what)
	ed.d = d
	ed.load()
	ed.winid, _ = ix.pg.Add(ed.win)
	return ed
}

func main() {
	opts := opt.New("")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
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
