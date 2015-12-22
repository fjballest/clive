/*
	This command mounts a zx tree through the native OS FUSE
	driver.
*/
package main

import (
	"clive/app/opt"
	"clive/dbg"
	"clive/zx"
	"clive/zx/cfs"
	"clive/zx/lfs"
	"clive/zx/mdfs"
	"clive/zx/mfs"
	"clive/zx/rfs"
	"clive/zx/zxfs"
	"fmt"
	"os"
	"time"

	fs "clive/fuse"
	"clive/x/bazil.org/fuse"
)

var (
	addr         string
	mntdir       = "/n/zx"
	rflag, sflag bool
	nopings      bool

	zxdebug, lfsdebug, rfsdebug, verb bool

	nocache             bool
	lfscache, mlfscache string
	xaddr               string
	opts                = opt.New("addr|dir [mntdir] &")
	dprintf             = dbg.FlagPrintf(os.Stderr, &zxfs.Debug)
)

func mklfs(path string) (zx.RWTree, *zx.Flags, *zx.IOstats, error) {
	ronly := rflag && nocache
	fs, err := lfs.New(path, path, ronly)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("lfs: %s", err)
	}
	if sflag {
		fs.IOstats = &zx.IOstats{}
	}
	fs.Dbg = lfsdebug
	fs.SaveAttrs(true)
	return fs, fs.Flags, fs.IOstats, nil
}

func mkrfs(addr string) (zx.RWTree, *zx.Flags, *zx.IOstats, error) {
	fs, err := rfs.Import(addr)
	if err != nil {
		return nil, nil, nil, err
	}
	if r, ok := fs.(*rfs.Rfs); ok && !nopings {
		r.Pings(30 * time.Second)
	}
	var st *zx.IOstats
	if sflag {
		switch x := fs.(type) {
		case *lfs.Lfs:
			x.IOstats = &zx.IOstats{}
			st = x.IOstats
		case *rfs.Rfs:
			x.IOstats = &zx.IOstats{}
			st = x.IOstats
		}
	}
	var flg *zx.Flags
	if r, ok := fs.(*rfs.Rfs); ok {
		flg = r.Flags
	} else if l, ok := fs.(*lfs.Lfs); ok {
		flg = l.Flags
	} else {
		flg = &zx.Flags{}
	}
	return fs, flg, st, nil
}

// on-memory cache
func mcache() (zx.RWTree, *zx.Flags, error) {
	m, err := mfs.New("cmfs")
	if err != nil {
		return nil, nil, fmt.Errorf("mfs: %s", err)
	}
	m.IOstats = &zx.IOstats{}
	m.Dbg = lfsdebug
	m.WstatAll = true // cfs must be able to write it all
	return m, m.Flags, nil
}

// on-disk cache
func dcache() (zx.RWTree, *zx.Flags, error) {
	m, err := lfs.New("clfs", lfscache, lfs.RW)
	if err != nil {
		return nil, nil, fmt.Errorf("lfs", err)
	}
	m.SaveAttrs(true)
	m.IOstats = &zx.IOstats{}
	m.Dbg = lfsdebug
	m.WstatAll = true // cfs must be able to write it all
	return m, m.Flags, nil
}

// on-memory medatadata, on-disk data cache
func mdcache() (zx.RWTree, *zx.Flags, error) {
	d, err := lfs.New("mlfs", mlfscache, lfs.RW)
	if err != nil {
		return nil, nil, fmt.Errorf("lfs", err)
	}
	d.SaveAttrs(true)
	d.WstatAll = true // cfs must be able to write it all
	m, err := mdfs.New("mdfs", d)
	if err != nil {
		return nil, nil, fmt.Errorf("mdfs: %s", err)
	}
	m.IOstats = &zx.IOstats{}
	d.Dbg = lfsdebug && false
	m.Dbg = lfsdebug
	m.WstatAll = true // cfs must be able to write it all
	return m, m.Flags, nil
}

func mkfs(addr string, quiet bool) (zx.Tree, func(), error) {
	if len(addr) == 0 {
		dbg.Warn("missing address")
		opts.Usage()
		dbg.Fatal("usage")
	}
	dbp := &fs.Debug
	var m, fs zx.RWTree
	var fsst *zx.IOstats
	var err error
	var lflags, flags *zx.Flags
	if _, err = os.Stat(addr); err == nil {
		fs, flags, fsst, err = mklfs(addr)
	} else {
		fs, flags, fsst, err = mkrfs(addr)
	}
	if err != nil {
		return nil, nil, err
	}

	switch {
	case nocache:
		dbg.Warn("%s: uncached", addr)
		if xaddr != "" {
			rfs.Server(fs, xaddr)
		}
		fn := func() {}
		if sflag {
			fn = func() {
				fsst.Averages()
				dbg.Warn("%s iostats:\n%s\n", fs.Name(), fsst)
			}
		}
		return fs, fn, nil
	case lfscache != "":
		dbg.Warn("%s: lfs cache %s", addr, lfscache)
		m, lflags, err = dcache()
	case mlfscache != "":
		dbg.Warn("%s: mdfs cache %s", addr, mlfscache)
		m, lflags, err = mdcache()
	default:
		dbg.Warn("%s: mfs cache", addr)
		m, lflags, err = mcache()
	}
	if err != nil {
		return nil, nil, err
	}

	cfs.Debug = zxdebug
	xfs, err := cfs.New("cfs", m, fs, rflag)
	if err != nil {
		return nil, nil, fmt.Errorf("cfs: %s", err)
	}
	xfs.Flags.Add("rdebug", &flags.Dbg)
	if lflags != nil {
		xfs.Flags.Add("ldebug", &lflags.Dbg)
	}
	xfs.Flags.Add("fdebug", &zxfs.Debug)
	xfs.Flags.Add("vdebug", dbp)
	xfs.Flags.Set("verbsync", !quiet)
	st := &zx.IOstats{}
	xfs.IOstats = st
	if xaddr != "" {
		rfs.Server(xfs, xaddr)
	}
	fn := func() {}
	if sflag {
		fn = func() {
			st.Averages()
			dbg.Warn("%s iostats:\n%s\n", xfs.Name(), st)
			fsst.Averages()
			dbg.Warn("%s iostats:\n%s\n", fs.Name(), fsst)
		}
	}
	return xfs, fn, nil
}

func ncmount(xfs zx.Tree) error {
	if rflag {
		xfs = zx.ROTreeFor(xfs)
	}
	return zxfs.MountServer(xfs, mntdir)
}

func main() {
	os.Args[0] = "zxfs"
	quiet := false
	stacks := false

	opts.NewFlag("q", "don't print errors to stderr", &quiet)
	opts.NewFlag("D", "debug zx calls", &zxdebug)
	opts.NewFlag("L", "debug lfs calls", &lfsdebug)
	opts.NewFlag("R", "debug rfs calls", &rfsdebug)
	opts.NewFlag("F", "debug fuse requests", &zxfs.Debug)
	opts.NewFlag("M", "debug mutexes", &cfs.DebugLocks)
	opts.NewFlag("V", "verbose fuse debug", &fs.Debug)
	opts.NewFlag("S", "dump stacks on unmount for debugging", &stacks)
	opts.NewFlag("k", "do not use zx keep alives", &nopings)
	opts.NewFlag("r", "read only", &rflag)
	opts.NewFlag("s", "statistics", &sflag)
	opts.NewFlag("n", "no caching", &nocache)
	opts.NewFlag("l", "dir: use lfs caching at dir", &lfscache)
	opts.NewFlag("m", "dir: use on-memory stat caching, on disk data caching at dir", &mlfscache)
	opts.NewFlag("x", "addr: re-export locally the cached tree to this address", &xaddr)
	args, err := opts.Parse(os.Args)
	if err != nil {
		dbg.Warn("%s", err)
		opts.Usage()
		dbg.Exits(err)
	}
	if nocache && (lfscache != "" || mlfscache != "") {
		dbg.Warn("can't use both caching and non-caching")
		opts.Usage()
		dbg.Exits(err)
	}
	if lfscache != "" && mlfscache != "" {
		dbg.Warn("can use only a single cache type")
		opts.Usage()
		dbg.Exits(err)
	}
	zxfs.Verb = !quiet
	fuse.Debug = func(m interface{}) {
		if fs.Debug {
			fmt.Fprintf(os.Stderr, "fuse: %v\n", m)
		}
	}
	switch len(args) {
	case 2:
		addr = args[0]
		mntdir = args[1]
	case 1:
		addr = args[0]
	default:
		dbg.Warn("wrong number of arguments")
		opts.Usage()
		dbg.Exits("usage")
	}
	dprintf("debug on\n")
	if cfs.DebugLocks {
		dbg.Warn("*** debug locks enabled")
	}
	xfs, fn, err := mkfs(addr, quiet)
	if err != nil {
		dbg.Fatal("%s: %s", addr, err)
	}
	defer fn()
	err = ncmount(xfs)
	if stacks {
		dbg.Warn("*** PANICING ON USER REQUEST (-S) ***")
		panic("stack dump")
	}
	if err != nil {
		dbg.Fatal("%s", err)
	}
	dbg.Warn("unmounted: exiting")
}
