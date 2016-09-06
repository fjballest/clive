/*
	Mount a zx tree through the native OS FUSE driver.
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	fs "clive/fuse"
	"clive/net/auth"
	"clive/x/bazil.org/fuse"
	"clive/zx"
	"clive/zx/rzx"
	"clive/zx/zux"
	"clive/zx/zxc"
	"clive/zx/zxfs"
	"io"
	"strings"
	"time"
)

var (
	addr   string
	mntdir = "/n/zx"
	rflag  bool

	verb, xdebug bool

	nocache bool
	xaddr   string
	opts    = opt.New("addr|dir [mntdir] &")
)

func main() {
	cmd.UnixIO()
	opts.NewFlag("X", "debug exported requests", &xdebug)
	opts.NewFlag("D", "debug requests", &zxfs.Debug)
	opts.NewFlag("F", "verbose debug requests", &zxfs.Verb)
	opts.NewFlag("V", "verbose fuse debug", &fs.Debug)
	opts.NewFlag("A", "auth debug", &auth.Debug)
	opts.NewFlag("v", "verbose cache", &verb)
	opts.NewFlag("r", "read only", &rflag)
	opts.NewFlag("n", "no caching", &nocache)
	opts.NewFlag("x", "addr: re-export locally the mounted tree to this address", &xaddr)
	args := opts.Parse()
	fuse.Debug = func(m face{}) {
		if fs.Debug {
			cmd.Eprintf("fuse: %v\n", m)
		}
	}
	switch len(args) {
	case 2:
		addr = args[0]
		mntdir = args[1]
	case 1:
		addr = args[0]
	default:
		cmd.Warn("wrong number of arguments")
		opts.Usage()
	}
	var rfs zx.Getter
	var err error
	method := "lfs"
	if strings.ContainsRune(addr, '!') {
		if strings.HasPrefix(addr, "zx!") {
			addr = addr[3:]
		}
		rfs, err = rzx.Dial(addr, auth.TLSclient)
		method = "rfs"
	} else if rflag {
		rfs, err = zux.New(addr)
	} else {
		rfs, err = zux.NewZX(addr)
	}
	if err != nil {
		cmd.Fatal("dial %s: %s", addr, err)
	}
	xfs := rfs
	if !nocache {
		xfs, err = zxc.New(rfs)
		if err != nil {
			cmd.Fatal("cache fs: %s", err)
		}
		if verb {
			xfs.(*zxc.Fs).Flags.Set("verb", true)
		}
	}
	if rflag {
		xfs = zx.MakeRO(xfs)
	}
	if xaddr != "" {
		srv, err := rzx.NewServer(xaddr, auth.TLSserver)
		if err != nil {
			cmd.Fatal("serve: %s", err)
		}
		srv.Debug = xdebug
		if err := srv.Serve("main", xfs); err != nil {
			cmd.Warn("serve: %s: %s", xaddr, err)
		}
	}
	rs := map[bool]string{false: "rw", true: "ro"}
	cs := map[bool]string{false: "uncached", true: "cached"}
	cmd.Warn("mount %s: %s %s %s %s", mntdir, addr, method, rs[rflag], cs[!nocache])
	err = zxfs.MountServer(xfs, mntdir)
	if err != nil {
		cmd.Warn("%s %s: mount error: %s", mntdir, addr, err)
	} else {
		cmd.Warn("%s %s: unmounted: exiting", mntdir, addr)
	}
	if cfs := xfs.(io.Closer); cfs != nil {
		wc := make(chan bool)
		go func() {
			cmd.Warn("%s %s: closing", mntdir, addr)
			cfs.Close()
			wc <- true
		}()
		select {
		case <-wc:
			cmd.Warn("%s %s: closed", mntdir, addr)
		case <-time.After(5 * time.Second):
			cmd.Warn("%s %s: close timed out", mntdir, addr)
		}
	}
	cmd.Exit(err)
}
