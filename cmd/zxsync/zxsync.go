/*
	push a zx replica
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx/repl"
	"os"
	"io/ioutil"
	"strings"
)

func sync1(name string) error {
	c := cmd.AppCtx()
	if !strings.ContainsRune(name, '/') {
		name = "/u/lib/repl/" + name
	}
	tr, err := repl.Load(name)
	if err != nil {
		cmd.Warn("load %s: %s", name, err)
		return err
	}
	defer tr.Close()
	if c.Debug {
		tr.Ldb.DumpTo(os.Stderr)
		tr.Rdb.DumpTo(os.Stderr)
	}
	if nflag {
		cc, err := tr.Changes()
		if err != nil {
			cmd.Warn("changes %s: %s", name, err)
			return err
		}
		for c := range cc {
			cmd.Printf("chg %s %s\n", c.At, c)
		}
		return nil
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
	if err != nil {
		cmd.Warn("sync %s: %s", name, err)
	}
	<-dc
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
	opts = opt.New("[file]")
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
	var err error
	switch len(args) {
	case 0:
		for _, nm := range names() {
			cmd.Printf("sync %s...\n", nm)
			if err2 := sync1(nm); err == nil {
				err = err2
			}
		}
	case 1:
		err = sync1(args[0])
	default:
		opts.Usage()
	}
	cmd.Exit(err)
}
