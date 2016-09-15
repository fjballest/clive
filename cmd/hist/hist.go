/*
	History tool for the zx dump.

*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx"
	"errors"
	"fmt"
	fpath "path"
	"strings"
	"time"
)

var (
	opts                = opt.New("{file}")
	force, all          bool
	lflag, cflag, dflag bool
	xcmd, dump          string

	lastyear, lastday string

	errNoDump = errors.New("no dump")
)

func ignored(year, day string) bool {
	if lastyear != "" && strings.Split(year, ".")[0] > lastyear {
		cmd.Dprintf("ignored %s\n", year)
		return true
	}
	if day != "" && lastday != "" && strings.Split(day, ".")[0] > lastday {
		cmd.Dprintf("ignored %s %s\n", year, day)
		return true
	}
	return false
}

func find(dump, dpref, rel string, dc chan<- zx.Dir, ufile zx.Dir) {
	droot := fpath.Join(dump, dpref)
	years, err := cmd.GetDir(droot)
	if err != nil {
		cmd.Warn("%s", err)
		return
	}
	for i := len(years) - 1; i >= 0; i-- {
		year := years[i]["name"]
		if ignored(year, "") {
			continue
		}
		ypath := years[i]["path"]
		days, err := cmd.GetDir(ypath)
		if err != nil {
			cmd.Warn("%s: %s", ypath, err)
			continue
		}
		lastsz, lastmt, lastm := "", "", ""
		for j := len(days) - 1; j >= 0; j-- {
			day := days[j]["name"]
			if ignored(year, day) {
				continue
			}
			fpath := fpath.Join(days[j]["path"], rel)
			d, err := cmd.Stat(fpath)
			if err != nil {
				if !force {
					cmd.Dprintf("find: %s", err)
					return
				}
				continue
			}
			newm, newsz, newmt := d["mode"], d["size"], d["mtime"]
			if newsz == lastsz && newmt == lastmt && newm == lastm {
				continue
			}
			lastm, lastsz, lastmt = newm, newsz, newmt
			d["upath"] = ufile["path"]
			d["uupath"] = ufile["upath"]
			if ok := dc <- d; !ok {
				return
			}
			if !all {
				return
			}
		}
	}
}

func report(dc chan zx.Dir, donec chan bool) {
	last := ""
	for d := range dc {
		if last == "" {
			last = d["Upath"]
			if last == "" {
				last = d["path"]
			}
		}
		p := d["path"]
		cmd.Dprintf("found '%s'\n", p)
		var err error
		switch {
		case xcmd != "":
			_, err = cmd.Printf("%s %s %s\n", xcmd, p, last)
		case dflag:
			dcmd := fmt.Sprintf(`9 diff -n %s %s`, p, last)
			_, err = cmd.Printf("%s\n", dcmd)
			if err != nil {
				cmd.Warn("diff: %s", err)
				continue
			}
		case lflag:
			_, err = cmd.Printf("%s\n", d.Fmt())
		case cflag:
			_, err = cmd.Printf("cp %s %s\n", p, d["Upath"])
		default:
			_, err = cmd.Printf("%s\n", d["path"])
		}
		if err != nil {
			close(dc, err)
		}
		last = p
	}
	close(donec, cerror(dc))
}

func hist(in <-chan face{}) error {
	dc := make(chan zx.Dir)
	ec := make(chan bool)
	go report(dc, ec)
	var sts error
	for m := range in {
		switch m := m.(type) {
		case zx.Dir:
			cmd.Dprintf("got %T %s\n", m, m["path"])
			file := m["path"]
			if m["upath"] == "" {
				m["upath"] = m["path"]
			}
			ddir := dump
			dpref := ""
			rel := ""
			switch {
			case zx.HasPrefix(file, "/zx"):
				if ddir == "" {
					ddir = "/dump"
				}
				dpref = "/zx"
				rel = zx.Suffix(file, "/zx")
			case zx.HasPrefix(file, "/u/gosrc/src/clive"):
				if ddir == "" {
					ddir = "/u/dump"
				}
				dpref = "clive"
				rel = zx.Suffix(file, "/u/gosrc/src/clive")
			case zx.HasPrefix(file, "/u"):
				if ddir == "" {
					ddir = "/u/dump"
				}
				els := zx.Elems(file)
				if len(els) < 3 {
					cmd.Warn("%s: too few path elements", m["upath"])
					sts = errNoDump
					continue
				}
				dpref = els[1]
				rel = zx.Path(els[2:]...)
			default:
				cmd.Warn("%s: %s", m["upath"], errNoDump)
				sts = errNoDump
				continue
			}
			find(ddir, dpref, rel, dc, m.Dup())
		default:
			cmd.Dprintf("got %T\n", m)

		}
	}
	close(dc, cerror(in))
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
func main() {
	c := cmd.AppCtx()
	cmd.UnixIO("err")
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("f", "force search past file removals", &force)
	opts.NewFlag("l", "produce a long listing (or print just the name)", &lflag)
	opts.NewFlag("c", "copy the file from the dump", &cflag)
	opts.NewFlag("d", "print file differences", &dflag)
	opts.NewFlag("x", "cmd: print lines to execute this command between versions", &xcmd)
	opts.NewFlag("a", "list all copies that differ, not just the last one.", &all)
	opts.NewFlag("p", "dumpdir: path to dump (default is /dump or /u/dump)", &dump)
	t := time.Now()
	when := t
	opts.NewFlag("w", "date: backward search start time (default is now)", &when)
	ux := false
	opts.NewFlag("u", "unix IO", &ux)
	args := opts.Parse()
	if (all && cflag) || (force && !all) {
		cmd.Warn("incompatible flags")
		opts.Usage()
	}
	if ux {
		cmd.UnixIO("out")
	}
	lastyear = ""
	lastday = ""
	if !t.Equal(when) {
		y := when.Year()
		m := when.Month()
		d := when.Day()
		if y == 0 {
			y = t.Year()
		}
		lastyear = fmt.Sprintf("%04d", y)
		lastday = fmt.Sprintf("%02d%02d", m, d)
	}
	if len(args) != 0 {
		cmd.SetIn("in", cmd.Dirs(args...))
	}
	in := cmd.In("in")
	if err := hist(in); err != nil {
		cmd.Fatal(err)
	}
}
