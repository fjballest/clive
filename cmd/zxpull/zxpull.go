/*
	pull a zx replica
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx/repl"
	"os"
	"strings"
)

var (
	opts = opt.New("file")
	notux, aflag, bflag, nflag bool
)

func main() {
	cmd.UnixIO("err")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("v", "verbose", &c.Verb)
	opts.NewFlag("a", "all", &aflag)
	opts.NewFlag("b", "blind", &bflag)
	opts.NewFlag("n", "dry run", &nflag)
	opts.NewFlag("u", "don't use unix out", &notux)
	args := opts.Parse()
	if !notux {
		cmd.UnixIO("out")
	}
	if len(args) != 1 || (aflag && bflag) {
		opts.Usage()
	}
	if !strings.ContainsRune(args[0], '/') {
		args[0] = "/u/lib/repl/" + args[0]
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
		var cc <-chan repl.Chg
		switch {
		case aflag:
			cc, err = tr.AllPullChanges()
		default:
			cc, err = tr.PullChanges()
		}
		if err != nil {
			cmd.Fatal(err)
		}
		for c := range cc {
			cmd.Printf("%s\n", c)
		}
		cmd.Exit(nil)
	}
	var cc chan repl.Chg
	dc := make(chan bool)
	if c.Verb {
		cc = make(chan repl.Chg)
		go func() {
			for c := range cc {
				cmd.Printf("%s\n", c)
			}
			close(dc)
		}()
	} else {
		close(dc)
	}
	switch {
	case aflag:
		err = tr.PullAll(cc)
	case bflag:
		err = tr.BlindPull(cc)
	default:
		err = tr.Pull(cc)
	}
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
