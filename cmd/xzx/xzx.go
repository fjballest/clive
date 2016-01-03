/*
	ZX file server.

	Export zx trees
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/dbg"
	"clive/net/auth"
	"clive/zx"
	"clive/zx/zxc"
	"clive/zx/rzx"
	"clive/zx/zux"
	fpath "path"
	"path/filepath"
	"strings"
)

var (
	noauth bool
	Zdebug bool
	dprintf = cmd.Dprintf
	vprintf = cmd.VWarn

	opts  = opt.New("{spec}")
	port, addr  string
)

func main() {
	cmd.UnixIO()
	opts.AddUsage("\tspec is name | name!file | name!file!flags \n")
	opts.AddUsage("\tspec flags are ro | rw | ncro | ncrw \n")
	port = "8002"
	addr = "*!*!zx"
	opts.NewFlag("p", "port: tcp server port (8002 by default)", &port)
	opts.NewFlag("a", "addr: service address (*!*!zx by default)", &addr)
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("v", "report users logged in/out (verbose)", &c.Verb)
	opts.NewFlag("Z", "verbose debug", &Zdebug)
	opts.NewFlag("n", "no auth", &noauth)
	args, err := opts.Parse()
	if err != nil {
		cmd.Warn("%s", err)
		opts.Usage()
	}
	if len(args) == 0 {
		cmd.Warn("missing arguments")
		opts.Usage()
	}
	c.Debug = c.Debug || Zdebug
	auth.Debug = c.Debug

	trs := map[string]zx.Fs{}
	ros := map[bool]string{false: "rw", true: "ro"}
	cs := map[bool]string{false: "uncached", true: "cached"}

	for i := 0; i < len(args); i++ {
		al := strings.Split(args[i], "!")
		if len(al) == 1 {
			al = append(al, al[0])
			al[0] = fpath.Base(al[0])
		}
		if _, ok := trs[al[0]]; ok {
			cmd.Warn("dup tree name %s", al[0])
			continue
		}
		ronly := false
		caching := true
		if len(al) == 3 && strings.Contains(al[2], "ro") {
			ronly = true
		}
		if len(al) == 3 && strings.Contains(al[2], "nc") {
			caching = false
		}
		fp, _ := filepath.Abs(al[1])
		t, err := zux.NewZX(fp)
		if err != nil {
			cmd.Warn("%s: %s", al[0], err)
			continue
		}
		t.Tag = al[0]
		ro := ronly && !caching
		t.Flags.Set("rdonly", ro)
		cmd.Warn("%s %s %s", al[0], ros[ronly], cs[caching])
		var x zx.Fs = t
		if caching {
			if ronly {
				x, err = zxc.NewRO(t)
			} else {
				x, err = zxc.New(t)
			}
			if err != nil {
				dbg.Warn("%s: zxc: %s", al[0], err)
				continue
			}
		}
		trs[t.Tag] = x
	}
	if len(trs) == 0 {
		cmd.Fatal("no trees to serve")
	}

	vprintf("serve %s...", addr)
	srv, err := rzx.NewServer(addr, auth.TLSserver)
	if err != nil {
		cmd.Fatal("serve: %s", err)
	}
	if noauth {
		srv.NoAuth()
	}
	for nm, fs := range trs {
		if err := srv.Serve(nm, fs); err != nil {
			cmd.Fatal("serve: %s: %s", nm, err)
		}
	}
	if err := srv.Wait(); err != nil {
		cmd.Fatal("srv: %s", err)
	}
}
