/*
	replicate files
*/
package main

import (
	"clive/app/opt"
	"clive/dbg"
	"clive/zx/sync"
	"clive/zx/sync/repl"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

var (
	opts                                 = opt.New("cfg [laddr raddr]")
	pullonly, pushonly, skip, dry, print bool
	quiet                                bool
	name                                 string
)

func waitReplTime(name string) {
	t := time.Now()
	dt := time.Date(t.Year(), t.Month(), t.Day(), 3, 0, 0, 0, time.Local)
	if dt.Before(time.Now()) {
		dt = dt.Add(24 * time.Hour)
	}
	dbg.Warn("next %s sync at %v", name, dt)
	delta := dt.Sub(t)
	time.Sleep(delta)
}

func cfgPath(nm string) string {
	if _, err := os.Stat(nm); err == nil {
		return nm
	}
	if strings.ContainsRune(nm, '/') {
		return nm
	}
	if nm == "." || nm == ".." {
		dbg.Fatal("Can't use . or .. as the cfg name")
	}
	return fmt.Sprintf("%s/lib/%s", dbg.Home, nm)
}

func main() {
	defer dbg.Exits("")
	os.Args[0] = "repl"
	opts.NewFlag("D", "debug", &sync.Debug)
	opts.NewFlag("V", "verbose debug", &sync.Verb)
	opts.NewFlag("q", "quiet, do not print files pulled/pushed", &quiet)
	opts.NewFlag("1", "pull only", &pullonly)
	opts.NewFlag("2", "push only", &pushonly)
	opts.NewFlag("s", "sleep and sync everyday at 3am", &skip)
	opts.NewFlag("n", "dry run", &dry)
	opts.NewFlag("p", "print the replica state and exit", &print)
	opts.NewFlag("m", "name: make the named repl and exit", &name)
	args, err := opts.Parse(os.Args)
	mk := name != ""
	if err != nil || skip && mk || pullonly && pushonly || dry && mk || print && mk || dry && print {
		if err == nil {
			err = errors.New("incompatible flags")
		}
		dbg.Warn("%s", err)
		opts.Usage()
		dbg.Exits(err)
	}
	if len(args) < 1 {
		dbg.Warn("missing arguments")
		opts.Usage()
		dbg.Exits("usage")
	}
	sync.Debug = sync.Debug || sync.Verb
	cfile := cfgPath(args[0])
	if mk {
		if len(args) != 3 {
			dbg.Warn("wrong number of arguments")
			opts.Usage()
			dbg.Exits("usage")
		}
		laddr, raddr := args[1], args[2]
		r, err := repl.New(name, repl.NoDots, laddr, raddr)
		if err != nil {
			dbg.Fatal(err)
		}
		if err := r.Save(cfile); err != nil {
			dbg.Fatal(err)
		}
		dbg.Warn("%s created", cfile)
		dbg.Exits(nil)
	}
	if len(args) != 1 {
		dbg.Warn("wrong number of arguments")
		opts.Usage()
		dbg.Exits("usage")
	}
	r, err := repl.Load(cfile)
	if err != nil {
		dbg.Fatal("%s: %s", cfile, err)
	}
	if print {
		r.DryRun = true // safety first.
		switch {
		case pullonly:
			r.Ldb.DumpTo(os.Stdout)
		case pushonly:
			r.Rdb.DumpTo(os.Stdout)
		default:
			r.DumpTo(os.Stdout)
		}
		dbg.Exits(nil)
	}
	r.DryRun = dry
	r.Verb = !quiet
	for {
		switch {
		case pullonly:
			err = r.Pull()
		case pushonly:
			err = r.Push()
		default:
			err = r.Sync()
		}
		if err != nil {
			dbg.Warn("%s: %s", r.Name, err)
		}
		if !dry {
			if err = r.Save(cfile); err != nil {
				dbg.Warn("%s: %s", r.Name, err)
			} else {
				dbg.Warn("%s: %s saved", r.Name, cfile)
			}
		}
		if skip {
			waitReplTime(r.Name)
			continue
		}
		dbg.Exits("")
	}
}
