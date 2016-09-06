/*
	push a zx replica
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx/repl"
	"strings"
	"io/ioutil"
	"os"
)

func push1(name string) error {
	if !strings.ContainsRune(name, '/') {
		name = "/u/lib/repl/" + name
	}
	tr, err := repl.Load(name)
	if err != nil {
		cmd.Warn("load %s: %s", name, err)
		return err
	}
	defer tr.Close()
	c := cmd.AppCtx()
	if c.Debug {
		tr.Ldb.DumpTo(os.Stderr)
		tr.Rdb.DumpTo(os.Stderr)
	}
	if nflag {
		var cc <-chan repl.Chg
		switch {
		case aflag:
			cc, err = tr.AllPushChanges()
		default:
			cc, err = tr.PushChanges()
		}
		if err != nil {
			cmd.Warn("push changes %s: %s", name, err)
			return err
		}
		for c := range cc {
			cmd.Printf("%s\n", c)
		}
		return nil
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
		err = tr.PushAll(cc)
	case bflag:
		err = tr.BlindPush(cc)
	default:
		err = tr.Push(cc)
	}
	<-dc
	if err != nil {
		cmd.Warn("push %s: %s", name, err)
	}
	if err2 := tr.Save(name); err2 != nil {
		cmd.Warn("save %s: %s", name, err2)
		if err == nil {
			err = err2
		}
	}
	return err
}

func names() []string {
	ds, err := ioutil.ReadDir("/u/lib/repl")
	if err != nil {
		cmd.Warn("/u/lib/repl: %s", err)
		return nil
	}
	nms := []string{}
	for _, d := range ds {
		if nm := d.Name(); strings.HasSuffix(nm, ".ldb") {
			nm = nm[:len(nm)-4]
			nms = append(nms, nm)
		}
	}
	return nms
}

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
	if aflag && bflag {
		opts.Usage()
	}
	var err error
	switch len(args) {
	case 0:
		for _, nm := range names() {
			cmd.Printf("push %s...\n", nm)
			if err2 := push1(nm); err == nil {
				err = err2
			}
		}
	case 1:
		err = push1(args[0])
	default:
		opts.Usage()
	}
	cmd.Exit(err)
}

