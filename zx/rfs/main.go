package rfs

import (
	"clive/dbg"
	"clive/nchan"
	"clive/net/auth"
	"clive/net/ds"
	"clive/zx"
	"os"
)

func serveFor(t zx.Tree, c nchan.Conn) {
	ai, err := auth.AtServer(c, "", "zx", "finder")
	if err != nil && err != auth.ErrDisabled {
		dbg.Warn("auth %s: %s\n", c.Tag, err)
		close(c.In, err)
		close(c.Out, err)
		return
	}
	srv := Serve("rfs:"+c.Tag, c, ai, RW, t)
	srv.Debug = false
}

// Facade that serves t at the given address by using the ds
// to serve and calling Serve for each client.
func Server(t zx.Tree, addr string) {
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
