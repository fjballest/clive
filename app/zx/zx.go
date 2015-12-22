/*
	ZX file server.

*/
package main

import (
	"clive/app/opt"
	"clive/dbg"
	"clive/nchan"
	"clive/net/auth"
	"clive/net/ds"
	"clive/net/wax"
	"clive/net/wax/ctl"
	"clive/zx"
	"clive/zx/cfs"
	"clive/zx/lfs"
	"clive/zx/mfs"
	"clive/zx/rfs"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	Verbose, Debug, Zdebug bool

	Dprintf = dbg.FlagPrintf(os.Stdout, &Debug)
	Vprintf = dbg.FlagPrintf(os.Stdout, &Verbose)

	opts  = opt.New("{spec}")
	port  string
	wport string
	svc   string
	addr  string

	zxw = map[string]string{}
)

func zxwax() {
	ctrl := ctl.NewControl()
	ctrl.BoolFlag("Debug", &Debug)
	ctrl.CmdFlag("Exit", func() {
		dbg.Fatal("exiting at user request")
	})
	wax.ServeLogin("/", "/index")
	pg := `$tb$<p>$zx$<p>`
	part, err := wax.New(pg)
	if err != nil {
		dbg.Warn("wax new: %s", err)
		return
	}
	part.SetEnv(map[string]interface{}{
		"tb": ctrl.Bar("tb"),
		"zx": zxw,
	})
	part.Serve("/index")
	if err = wax.Serve(":" + wport); err != nil {
		dbg.Warn("wax serve: %s", err)
	}
}

func usage(err error) {
	dbg.Warn("%s", err)
	opts.Usage()
	fmt.Fprintf(os.Stderr, "\tspec is name | name!file | name!file!flags \n")
	fmt.Fprintf(os.Stderr, "\tspec flags are ro | rw | ncro | ncrw \n")
	dbg.Exits(err)
}

func main() {
	defer dbg.Exits("")
	os.Args[0] = "zx"
	port = "8002"
	opts.NewFlag("p", "port: tcp server port (8002 by default)", &port)
	wport = "9002"
	opts.NewFlag("w", "port: wax http server port", &wport)
	svc = "zx"
	opts.NewFlag("s", "srv: service name (zx by default)", &svc)
	addr = "*!*!zx"
	opts.NewFlag("a", "addr: service address (*!*!zx by default)", &addr)
	opts.NewFlag("v", "report users logged in/out (verbose)", &Verbose)
	opts.NewFlag("D", "debug", &Debug)
	opts.NewFlag("Z", "verbose debug", &Zdebug)
	opts.NewFlag("M", "debug mutexes", &cfs.DebugLocks)
	nopings := false
	opts.NewFlag("k", "do not use zx keep alives", &nopings)
	args, err := opts.Parse(os.Args)
	if err != nil {
		usage(err)
	}
	if len(args) == 0 {
		usage(errors.New("missing arguments"))
	}
	Debug = Debug || Zdebug
	auth.Debug = Debug
	cfs.Debug = Debug
	rfs.Verb = Verbose
	var trs []zx.Tree
	var ros = map[bool]string{false: "rw", true: "ro"}

	for i := 0; i < len(args); i++ {
		al := strings.Split(args[i], "!")
		if len(al) == 1 {
			al = append(al, al[0])
			al[0] = path.Base(al[0])
		}
		ronly := false
		caching := true
		if len(al) == 3 && strings.Contains(al[2], "ro") {
			ronly = true
		}
		if len(al) == 3 && strings.Contains(al[2], "nc") {
			caching = false
		}
		t, err := lfs.New(al[0], al[1], ronly && !caching)
		if err != nil {
			dbg.Warn("%s: %s", al[0], err)
			continue
		}
		t.ReadAttrs(true)
		t.SaveAttrs(caching)
		t.IOstats = &zx.IOstats{}
		fp, _ := filepath.Abs(al[1])
		if caching {
			dbg.Warn("%s mfs + lfs %s caching", al[0], ros[ronly])
			cache, err := mfs.New("mfs:" + al[0])
			if err != nil {
				dbg.Warn("%s: mfs: %s", al[0], err)
				continue
			}
			cache.IOstats = &zx.IOstats{}
			cache.Dbg = Zdebug
			cache.WstatAll = true // cfs must be able to write it all
			x, err := cfs.New(al[0], cache, t, ronly)
			if err != nil {
				dbg.Warn("%s: cfs: %s", al[0], err)
				continue
			}
			x.IOstats = &zx.IOstats{}
			zxw[al[0]] = fp
			trs = append(trs, x)

		} else {
			dbg.Warn("%s lfs %s uncached", al[0], ros[ronly])
			zxw[al[0]] = fp
			t.Dbg = Debug
			trs = append(trs, t)
		}
	}
	if len(trs) == 0 {
		dbg.Fatal("no trees to serve")
	}

	ds.DefSvc(svc, port)
	Vprintf("%s: serve %s...\n", os.Args[0], addr)
	cc, _, err := ds.Serve(os.Args[0], addr)
	if err != nil {
		dbg.Fatal("%s: serve: %s", os.Args[0], err)
	}

	go zxwax()
	for c := range cc {
		go func(c nchan.Conn) {
			ai, err := auth.AtServer(c, "", "zx", "finder")
			if err != nil && err != auth.ErrDisabled {
				Vprintf("%s: auth %s: %s\n", os.Args[0], c.Tag, err)
				close(c.In, err)
				close(c.Out, err)
				return
			}
			srv := rfs.Serve("rfs:"+c.Tag, c, ai, rfs.RW, trs...)
			if false {
				srv.Debug = Debug
			}
			srv.Pings = !nopings

		}(*c)
	}
	if err := cerror(cc); err != nil {
		dbg.Fatal("%s: serve: %s", os.Args[0], err)
	}
}
