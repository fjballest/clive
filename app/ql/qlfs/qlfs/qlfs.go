/*
	a FUSE ql fs
*/
package main

import (
	"clive/app"
	"clive/app/opt"
	"clive/app/ql/qlfs"
	"clive/dbg"
	"clive/zx"
	"clive/zx/rfs"
	"clive/zx/zxfs"
	"os"
)

var (
	sflag       bool
	opts        = opt.New("[mntdir] &")
	addr, xaddr string
	mntdir      = "/n/ql"
	dprintf     = dbg.FlagPrintf(os.Stderr, &zxfs.Debug)
)

func main() {
	defer app.Exiting()
	x := app.New()
	stacks := false
	opts.NewFlag("F", "debug fuse requests", &zxfs.Debug)
	opts.NewFlag("D", "debug", &x.Debug)
	opts.NewFlag("s", "statistics", &sflag)
	opts.NewFlag("x", "addr: re-export locally the ql tree to this address", &xaddr)
	opts.NewFlag("S", "dump stacks on unmount for debugging", &stacks)
	args, err := opts.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		opts.Usage()
		app.Exits(err)
	}
	switch len(args) {
	case 0:
	case 1:
		mntdir = args[1]
	default:
		app.Warn("wrong number of arguments")
		opts.Usage()
		app.Exits("usage")
	}
	dprintf("debug on\n")
	qfs, err := qlfs.New("qlfs")
	if err != nil {
		app.Fatal(err)
	}
	qfs.Dbg = x.Debug
	qfs.Flags.Add("fdebug", &zxfs.Debug)
	st := &zx.IOstats{}
	qfs.IOstats = st
	if xaddr != "" {
		rfs.Server(qfs, xaddr)
	}
	err = zxfs.MountServer(qfs, mntdir)
	if sflag {
		st.Averages()
		app.Warn("%s iostats:\n%s\n", qfs.Name(), st)
	}
	if stacks {
		app.Warn("*** PANICING ON USER REQUEST (-S) ***")
		panic("stack dump")
	}
	if err != nil {
		app.Fatal("%s", err)
	}
	app.Warn("unmounted: exiting")
}
