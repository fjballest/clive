/*
	push a zx replica
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx/repl"
	"os"
)

var (
	opts = opt.New("file")
	notux, nflag bool
)

func main() {
	cmd.UnixIO("err")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("v", "verbose", &c.Verb)
	opts.NewFlag("u", "don't use unix out", &notux)
	opts.NewFlag("n", "dry run", &nflag)
	args := opts.Parse()
	if !notux {
		cmd.UnixIO("out")
	}
	if len(args) != 1 {
		opts.Usage()
	}
	tr, err := repl.Load(args[0])
	if err != nil {
		cmd.Fatal(err)
	}
	if c.Debug {
		tr.Ldb.DumpTo(os.Stderr)
		tr.Rdb.DumpTo(os.Stderr)
	}
	if nflag {
		cc, err := tr.Changes()
		if err != nil {
			cmd.Fatal(err)
		}
		for c := range cc {
			cmd.Printf("chg %s %s\n", c.At, c)
		}
		cmd.Exit(nil)
	}
	var cc chan repl.Chg
	dc := make(chan bool)
	if c.Verb {
		cc = make(chan repl.Chg)
		go func() {
			for c := range cc {
				cmd.Printf("chg %s %s\n", c.At, c)
			}
			close(dc)
		}()
	} else {
		close(dc)
	}
	err = tr.Sync(cc)
	<-dc
	if err := tr.Save(args[0]); err != nil {
		tr.Close()
		cmd.Fatal("save: %s", err)
	}
	tr.Close()
	if err != nil {
		cmd.Fatal(err)
	}
	cmd.Exit(nil)
}
