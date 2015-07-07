/*
	History tool for the zx dump.

*/
package hist

import (
	"clive/app"
	"clive/app/opt"
	"clive/app/nsutil"
	"clive/dbg"
	"clive/zx"
	"errors"
	"fmt"
	"strings"
	"time"
)

type xCmd {
	*opt.Flags
	*app.Ctx

	force, all	bool
	lflag, cflag, dflag	bool
	cmd, dump	string

	lastyear, lastday string
}

var (
	errNoDump = errors.New("no dump")
	prefs = map[string]string{
		// frrom <pref>/path into: <dump>/<pref>/yyyy/mmdd/path
		"/zx": "/zx",
	}
)

func (x *xCmd) ignored(year, day string) bool {
	if x.lastyear!="" && strings.Split(year, ".")[0]>x.lastyear {
		app.Dprintf("ignored %s\n", year)
		return true
	}
	if day != "" && x.lastday != "" && strings.Split(day, ".")[0] >x.lastday {
		app.Dprintf("ignored %s %s\n", year, day)
		return true
	}
	return false
}

func (x *xCmd) find(dpref, rel string, dc chan<- zx.Dir, ufile zx.Dir) {
	droot := zx.Path(x.dump, dpref)
	years, err := nsutil.GetDir(droot)
	if err != nil {
		app.Warn("%s", err)
		return
	}
	for i := len(years) - 1; i >= 0; i-- {
		year := years[i]["name"]
		if x.ignored(year, "") {
			continue
		}
		ypath := years[i]["path"]
		days, err := nsutil.GetDir(ypath)
		if err != nil {
			app.Warn("%s: %s", ypath, err)
			continue
		}
		lastsz, lastmt, lastm := "", "", ""
		for j := len(days) - 1; j >= 0; j-- {
			day := days[j]["name"]
			if x.ignored(year, day) {
				continue
			}
			fpath := zx.Path(days[j]["path"], rel)
			d, err := nsutil.Stat(fpath)
			if err != nil {
				if !x.force {
					app.Dprintf("find: %s", err)
					return
				}
				continue
			}
			newm, newsz, newmt := d["mode"], d["size"], d["mtime"]
			if newsz==lastsz && newmt==lastmt && newm==lastm {
				continue
			}
			lastm, lastsz, lastmt = newm, newsz, newmt
			d["upath"] = ufile["path"]
			d["uupath"] = ufile["upath"]
			if ok := dc <- d; !ok {
				return
			}
			if !x.all {
				return
			}
		}
	}
}

func (x *xCmd) report(dc chan zx.Dir, donec chan bool) {
	last := ""
	for d := range dc {
		p := d["path"]
		if last == "" {
			last = d["upath"]
		}
		app.Dprintf("found '%s'\n", p)
		var err error
		switch {
		case x.cmd != "":
			err = app.Printf("%s %s %s\n", x.cmd, p, last)
		case x.dflag:
			dcmd := fmt.Sprintf(`gf %s | diffs  <|{gf %s}`, p, last)
			//o, err := exec.Command("ql", "-c", dcmd).Output()
			err = app.Printf("%s\n", dcmd)
			if err != nil {
				app.Warn("diff: %s", err)
				continue
			}
		case x.lflag:
			err = app.Printf("%s\n", d.Long())
		case x.cflag:
			err = app.Printf("cp %s %s\n", d["path"], d["uupath"])
		default:
			err = app.Printf("%s\n", d["path"])
		}
		if err != nil {
			close(dc, err)
		}
		last = p
	}
	close(donec, cerror(dc))
}

func (x *xCmd) hist(in chan interface{}) error {
	dc := make(chan zx.Dir)
	ec := make(chan bool)
	go x.report(dc, ec)
	var sts error
	doselect {
	case <-x.Sig:
		close(dc, dbg.ErrIntr)
		<-ec
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-in:
		if !ok {
			close(dc, cerror(in))
			break
		}
		switch m := m.(type) {
		case zx.Dir:
			app.Dprintf("got %T %s\n", m, m["path"])
			file := m["path"]
			dpref := ""
			rel := ""
			for p, v := range prefs {
				if zx.HasPrefix(file, p) {
					dpref = v
					rel = zx.Suffix(file, p)
					break
				}
			}
			if dpref == "" {
				app.Warn("%s: %s", m["upath"], errNoDump)
				sts = errNoDump
			}
			x.find(dpref, rel, dc, m.Dup())
		default:
			app.Dprintf("got %T\n", m)
			
		}
	}
	<-ec
	if sts == nil {
		sts = cerror(ec)
	}
	if sts == nil {
		sts = cerror(in)
	}
	return sts
}

// Run cnt in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("{file}")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("f", "force search past file removals", &x.force)
	x.NewFlag("l", "produce a long listing (or print just the name)", &x.lflag)
	x.NewFlag("c", "copy the file from the dump", &x.cflag)
	x.NewFlag("d", "print file differences", &x.dflag)
	x.NewFlag("x", "cmd: print lines to execute this command between versions", &x.cmd)
	x.NewFlag("a", "list all copies that differ, not just the last one.", &x.all)
	x.dump = "/dump"
	x.NewFlag("p", "dumpdir: path to dump (default is /dump)", &x.dump)
	t := time.Now()
	when := t
	x.NewFlag("w", "date: backward search start time (default is now)", &when)
	args, err := x.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}
	if (x.all && x.cflag) || (x.force && !x.all) {
		app.Warn("incompatible flags")
		x.Usage()
		app.Exits("usage")
	}
	x.lastyear = ""
	x.lastday = ""
	if !t.Equal(when) {
		y := when.Year()
		m := when.Month()
		d := when.Day()
		if y == 0 {
			y = t.Year()
		}
		x.lastyear = fmt.Sprintf("%04d", y)
		x.lastday = fmt.Sprintf("%02d%02d", m, d)
	}
	if len(args) != 0 {
		in := app.Dirs(args...)
		app.SetIO(in, 0)
	}
	in := app.In()
	app.Exits(x.hist(in))
}
