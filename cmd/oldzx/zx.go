/*
	DEPRECATED OLD VERSION FOR ZX
	Use zx instead. This is kept only for measuring in the old setup.


	Zx exports one or more local file trees to the network using the
	clive zx protocol. It registers the exported trees in the clive name
	space and serves a small wax interface.

	Each argument may be of the form

		/path/to/a/tree

	to export the tree, or

		name!/path

	to export the path but with the given name. By default, the
	name is the base name of the path exported.

	The form

		name!/path!ro

	may be used to export the tree read-only, using a memory cache
	for the exported tree. The form

		name!/path!rw

	exports it read-write using a memory cache for the tree. The forms

		name!/path!ncro
		name!/path!ncrw

	are similiar and do not use a memory cache.


	For example, this is suggested to export the main tree and the dump:

		zx   -v zx!/home/lsub!rw dump!/home/dump!ncro



*/
package main

// REFERENCE(x): clive/cmd/ns, the name space server.

// REFERENCE(x): clive/cmd/nsh, the name space shell.

import (
	"clive/cmd/opt"
	"clive/dbg"
	"clive/nchan"
	"clive/net/auth"
	"clive/net/ds"
	"clive/net/wax"
	"clive/net/wax/ctl"
	"clive/zx"
	ncfs "clive/zx/cfs"
	"clive/zx/lfs"
	"clive/zx/mfs"
	cfs "clive/zx/ocfs"
	"clive/zx/rfs"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	Verbose                     = true
	Persist, Debug, Zdebug, New bool

	Dprintf = dbg.FlagPrintf(os.Stdout, &Debug)
	Vprintf = dbg.FlagPrintf(os.Stdout, &Verbose)

	opts  = opt.New("spec... ***DEPRECATED***")
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
	opts.Usage(os.Stderr)
	fmt.Fprintf(os.Stderr, "\tspec = name | name!file | name!file!flags \n")
	fmt.Fprintf(os.Stderr, "\tspec flags = ro | rw | ncro | ncrw \n")
	if err == nil {
		dbg.Fatal("usage")
	}
	dbg.Fatal(err)
}

func main() {
	defer dbg.Exits("")
	os.Args[0] = "zx"
	port = "8002"
	opts.NewFlag("p", "port: tcp server port", &port)
	wport = "9002"
	opts.NewFlag("w", "port: wax http server port", &wport)
	svc = "zx"
	opts.NewFlag("s", "srv: service name", &svc)
	addr = "*!*!zx"
	opts.NewFlag("a", "addr: service address", &addr)
	opts.NewFlag("D", "debug", &Debug)
	opts.NewFlag("Z", "verbose debug", &Zdebug)
	opts.NewFlag("k", "make cfg entries persist in the ns", &Persist)
	opts.NewFlag("N", "use new cfs", &New)
	args, err := opts.Parse(os.Args)
	if err != nil {
		usage(err)
	}
	if len(args) == 0 {
		usage(nil)
	}
	Debug = Debug || Zdebug
	auth.Debug = Debug
	ncfs.Debug = Debug
	cfs.Debug = Debug
	cfs.Cdebug = Zdebug
	cfs.Zdebug = Zdebug
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
		if New && caching {
			dbg.Warn("%s mfs + lfs %s caching", al[0], ros[ronly])
			cache, err := mfs.New("mfs:" + al[0])
			if err != nil {
				dbg.Warn("%s: mfs: %s", al[0], err)
				continue
			}
			cache.IOstats = &zx.IOstats{}
			cache.Dbg = Zdebug
			x, err := ncfs.New("cfs", cache, t, ronly)
			if err != nil {
				dbg.Warn("%s: cfs: %s", al[0], err)
				continue
			}
			x.IOstats = &zx.IOstats{}
			zxw[al[0]] = fp
			trs = append(trs, x)

		} else if !New && caching {
			dbg.Warn("%s old cfs + lfs %s caching", al[0], ros[ronly])
			x, err := cfs.New("", t, ronly)
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

		}(*c)
	}
	if err := cerror(cc); err != nil {
		dbg.Fatal("%s: serve: %s", os.Args[0], err)
	}
}
