/*
	DEPRECATED OLD VERSION FOR ZXFS
	Use zxfs instead. This is kept only for measuring in the old setup.

	If the first argument is a file existing in the local file tree (a directory),
	a local zx file system is started and served through fuse.

	Otherwise, it is understood as a network address to import a remote
	zx file tree from it, and serve it through fuse.

	By default, the command mounts the tree at /n/zx, unless an
	extra argument with the mount path is given.

	The preferred form of caching is starting a zx cfs wrapping the underlying
	zx tree being served. This is a write-through cache and is enabled unless
	-n (no caching) or -d (delayed writes) is used.

	A delayed write cache is also available under flag -d.

	None of the caches ever evict contents, assuming that memory is plenty
	and that swap is also configured. Seldom used (but cached) files are
	expected to be paged out in those cases when the tree does not fit.

	It is suggested not to use caching when mounting a dump file system.

	Flag -s may be used to ask the command to report usage statistics
	when unmounted.

	The tree is served read-write by default, unless -r is given.

	For example,

		zxfs  tcp!nautilus!zx

	makes available at /n/zx the tree found at tcp!nautilus!zx, using a
	write-through cache.

		zxfs  -nr tcp!nautilus!zx

	does the same but with no cache and in read-only mode.


*/
package main

import (
	"clive/cmd/opt"
	"clive/dbg"
	"clive/nchan"
	"clive/net/auth"
	"clive/net/ds"
	"clive/zx"
	ncfs "clive/zx/cfs"
	cfs "clive/zx/ocfs"
	"clive/zx/mfs"
	"clive/zx/lfs"
	"clive/zx/zxfs"
	"clive/zx/rfs"
	"fmt"
	"io"
	"os"

	fs "clive/fuse"
	"clive/x/bazil.org/fuse"
)

var (
	addr         string
	mntdir       = "/n/zx"
	rflag, sflag bool
	zdebug       bool
	delayed      bool
	nocaching    bool
	mfscfs	bool
	lfsdir	string
	xaddr        string
	opts         = opt.New("addr|dir [mntdir] & ***DEPRECATED***")
	dprintf      = dbg.FlagPrintf(os.Stderr, &zxfs.Debug)
)

func serveFor(t zx.Tree, c nchan.Conn) {
	ai, err := auth.AtServer(c, "", "zx", "finder")
	if err!=nil && err!=auth.ErrDisabled {
		dbg.Warn("auth %s: %s\n", c.Tag, err)
		close(c.In, err)
		close(c.Out, err)
		return
	}
	srv := rfs.Serve("rfs:"+c.Tag, c, ai, rflag, t)
	srv.Debug = false
}

func serve(t zx.Tree, addr string) {
	dbg.Warn("serve %s at %s...", t.Name(), addr)
	cc, ec, err := ds.Serve(os.Args[0], addr)
	if err != nil {
		dbg.Warn("serve: %s", err)
		return
	}
	go func() {
		for c := range cc {
			if c != nil {
				go serveFor(t, *c)
			}
		}
		if err := cerror(cc); err != nil {
			dbg.Warn("serve: %s", err)
		}
		close(ec, "done")
	}()
}

func cache(t zx.Tree, fn func()) (zx.Tree, func(), error) {
	cfs.Debug = zdebug
	cfs.Cdebug = zdebug
	xfs, err := cfs.New("", t, rflag)
	if err != nil {
		return nil, nil, fmt.Errorf("cfs: %s", err)
	}
	st := &zx.IOstats{}
	xfs.IOstats = st
	if xaddr != "" {
		serve(xfs, xaddr)
	}
	if sflag {
		xfn := func() {
			st.Averages()
			dbg.Warn("%s iostats:\n%s\n", xfs.Name(), st)
			if fn != nil {
				fn()
			}
		}
		return xfs, xfn, nil
	}
	return xfs, fn, nil
}

func mfscache(t zx.Tree, fn func()) (zx.Tree, func(), error) {
	m, err := mfs.New("cmfs")
	if err != nil {
		return nil, nil, fmt.Errorf("mfs: %s", err)
	}
	m.IOstats = &zx.IOstats{}
	m.Dbg = zdebug
	ncfs.Debug = zdebug
	xfs, err := ncfs.New("cfs", m, t, rflag)
	if err != nil {
		return nil, nil, fmt.Errorf("cfs: %s", err)
	}
	st := &zx.IOstats{}
	xfs.IOstats = st
	if xaddr != "" {
		serve(xfs, xaddr)
	}
	if sflag {
		xfn := func() {
			st.Averages()
			dbg.Warn("%s iostats:\n%s\n", xfs.Name(), st)
			if fn != nil {
				fn()
			}
		}
		return xfs, xfn, nil
	}
	return xfs, fn, nil
}

func lfscache(t zx.Tree, fn func()) (zx.Tree, func(), error) {
	m, err := lfs.New("clfs", lfsdir, lfs.RW)
	if err != nil {
		return nil, nil, fmt.Errorf("lfs", err)
	}
	m.SaveAttrs(true)
	m.IOstats = &zx.IOstats{}
	m.Dbg = zdebug
	ncfs.Debug = zdebug
	xfs, err := ncfs.New("cfs", m, t, rflag)
	if err != nil {
		return nil, nil, fmt.Errorf("cfs: %s", err)
	}
	st := &zx.IOstats{}
	xfs.IOstats = st
	if xaddr != "" {
		serve(xfs, xaddr)
	}
	if sflag {
		xfn := func() {
			st.Averages()
			dbg.Warn("%s iostats:\n%s\n", xfs.Name(), st)
			if fn != nil {
				fn()
			}
		}
		return xfs, xfn, nil
	}
	return xfs, fn, nil
}


func mklfs(path string) (zx.Tree, *zx.IOstats, error) {
	ronly := rflag && (nocaching || delayed)
	fs, err := lfs.New(path, path, ronly)
	if err != nil {
		return nil, nil, fmt.Errorf("lfs: %s", err)
	}
	if sflag {
		fs.IOstats = &zx.IOstats{}
	}
	fs.Dbg = zdebug
	fs.SaveAttrs(true)
	return fs, fs.IOstats, nil
}

func mkrfs(addr string) (zx.Tree, *zx.IOstats, error) {
	fs, err := rfs.Import(addr)
	if err != nil {
		return nil, nil, fmt.Errorf("rfs: %s: %s", addr, err)
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
	return fs, st, nil
}

func mkfs(addr string) (zx.Tree, func(), error) {
	if len(addr) == 0 {
		opts.Usage(os.Stderr)
		dbg.Fatal("usage")
	}
	var fs zx.Tree
	var st *zx.IOstats
	var err error
	if _, err := os.Stat(addr); err == nil {
		fs, st, err = mklfs(addr)
	} else {
		fs, st, err = mkrfs(addr)
	}
	if err != nil {
		return nil, nil, err
	}
	fn := func() {}
	if sflag {
		fn = func() {
			st.Averages()
			dbg.Warn("%s iostats:\n%s\n", fs.Name(), st)
		}
	}
	if mfscfs {
		dbg.Warn("%s: mfs cache", addr)
		return mfscache(fs, fn)
	}
	if lfsdir != "" {
		dbg.Warn("%s: lfs cache", addr)
		return lfscache(fs, fn)
	}
	if !nocaching && !delayed {
		dbg.Warn("%s: old cache", addr)
		return cache(fs, fn)
	}
	dbg.Warn("%s: uncached", addr)
	return fs, fn, nil
}

type dumper interface {
	DumpTo(io.Writer, int)
}

type syncer interface {
	Sync() chan error
}

// this must go. New interfaces are needed here.
var dmp dumper
var sync syncer

func ncmount(xfs zx.Tree) error {
	if rflag {
		xfs = zx.ROTreeFor(xfs)
	}
	zfs, err := zxfs.New(xfs)
	if err != nil {
		return fmt.Errorf("new zxfs: %s", err)
	}
	dmp = zfs
	c, err := fuse.Mount(mntdir)
	if err != nil {
		return fmt.Errorf("mount zxfs: %s", err)
	}
	defer c.Close()
	err = fs.Serve(c, zfs)
	if err != nil {
		return fmt.Errorf("serve zxfs: %s", err)
	}

	<-c.Ready
	if err := c.MountError; err != nil {
		return fmt.Errorf("mount error: %s", err)
	}
	return nil
}

func main() {
	defer dbg.Exits("")
	os.Args[0] = "zxfs"
	quiet := false

	opts.NewFlag("q", "don't print errors to stderr", &quiet)
	opts.NewFlag("D", "debug and zxfs calls", &zxfs.Debug)
	opts.NewFlag("r", "read only", &rflag)
	opts.NewFlag("s", "statistics", &sflag)
	opts.NewFlag("n", "don't use caching (otherwise write-through cache)", &nocaching)
	opts.NewFlag("d", "use delayed writes cache", &delayed)
	opts.NewFlag("Z", "debug zx requests", &zdebug)
	opts.NewFlag("V", "verbose debug and fuse requests", &fs.Debug)
	opts.NewFlag("x", "addr: re-export locally the cached tree to this address, if any", &xaddr)
	opts.NewFlag("m", "use mfs caching", &mfscfs)
	opts.NewFlag("l", "dir: use lfs caching at dir", &lfsdir)
	args, err := opts.Parse(os.Args)
	if err != nil {
		opts.Usage(os.Stderr)
		dbg.Fatal(err)
	}
	zxfs.Debug = zxfs.Debug || fs.Debug
	zxfs.Verb = !quiet || zxfs.Debug
	if fs.Debug {
		fuse.Debug = func(m interface{}) {
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
		opts.Usage(os.Stderr)
		dbg.Fatal("usage")
	}
	dprintf("debug on\n")
	xfs, fn, err := mkfs(addr)
	if err != nil {
		dbg.Fatal("%s", err)
	}
	defer fn()
	if nocaching || !delayed {
		err = ncmount(xfs)
	} else {
		dbg.Fatal("delayed write mount is gone")
	}
	if err != nil {
		dbg.Fatal("%s", err)
	}
	dbg.Warn("unmounted: exiting")
}
