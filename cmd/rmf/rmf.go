/*
	remove files command
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx"
)

var (
	opts                    = opt.New("{file}")
	aflag, fflag, dry, verb bool
	dirs                    []zx.Dir

	astr = map[bool]string{true: "all"}
)

func rmf(d zx.Dir) error {
	p := d.SPath()
	up := d["Upath"]
	if up == "" {
		up = d["path"]
	}
	if p == "" || p == "/" {
		cmd.Fatal("won't remove / in server for '%s'", up)
	}
	cmd.VWarn("rmf%s %s", astr[aflag], up)
	if dry {
		return nil
	}
	if aflag {
		return cmd.RemoveAll(d["path"])
	}
	return cmd.Remove(d["path"])
}

// Run rem in the current app context.
func main() {
	c := cmd.AppCtx()
	cmd.UnixIO("err")
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("v", "verbose; print the calls made in the order they are made.", &c.Verb)
	opts.NewFlag("a", "remove all", &aflag)
	opts.NewFlag("f", "quiet, called 'force' in unix", &fflag)
	opts.NewFlag("n", "dry run; report removes but do not do them", &dry)
	args := opts.Parse()
	if len(args) != 0 {
		cmd.SetIn("in", cmd.Dirs(args...))
	}
	c.Verb = c.Verb || dry
	in := cmd.In("in")
	var err error
	for m := range in {
		switch d := m.(type) {
		case zx.Dir:
			cmd.Dprintf("got %T %s\n", d, d["upath"])
			dirs = append(dirs, d)
		case error:
			cmd.Warn("%s", d)
		default:
			// ignored
			cmd.Dprintf("ignored %T\n", m)
		}
	}
	if aflag {
		for i := 0; i < len(dirs); i++ {
			if dirs[i] == nil {
				continue
			}
			pi := dirs[i]["path"]
			for j := 1; j < len(dirs); j++ {
				if i == j || dirs[j] == nil {
					continue
				}
				pj := dirs[j]["path"]
				if zx.HasPrefix(pj, pi) {
					dirs[j] = nil
				}
			}
		}
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if dirs[i] == nil {
			continue
		}
		if cerr := rmf(dirs[i]); cerr != nil {
			if !fflag {
				cmd.Warn("%s", cerr)
				err = cerr
			}
		}
	}
	if err == nil {
		err = cerror(in)
		if err != nil {
			cmd.Fatal(err)
		}
	}
	cmd.Exit(err)
}
