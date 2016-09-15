/*
	sync a zx replica
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx/repl"
	"io/ioutil"
	"os"
	"strings"
)

func sync1(name string, rc chan face{}) *repl.Tree {
	c := cmd.AppCtx()
	if !strings.ContainsRune(name, '/') {
		name = "/u/lib/repl/" + name
	}
	tr, err := repl.Load(name)
	if err != nil {
		close(rc, err)
		return nil
	}
	if c.Debug {
		tr.Ldb.DumpTo(os.Stderr)
		tr.Rdb.DumpTo(os.Stderr)
	}
	go func() {
		if nflag {
			cc, err := tr.Changes()
			if err != nil {
				close(rc, err)
				return
			}
			for c := range cc {
				if ok := rc <- c; !ok {
					close(cc, cerror(rc))
				}
			}
			close(rc)
			return
		}
		var cc chan repl.Chg
		dc := make(chan bool)
		if c.Verb {
			cc = make(chan repl.Chg)
			go func() {
				for c := range cc {
					if ok := rc <- c; !ok {
						close(cc, cerror(rc))
					}
				}
				close(dc)
			}()
		} else {
			close(dc)
		}
		err = tr.Sync(cc)
		if err != nil {
			rc <- err
		}
		<-dc
		if err2 := tr.Save(name); err2 != nil {
			rc <- err
		}
		close(rc)
	}()
	return tr
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
	opts         = opt.New("[file]")
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
	rcs := []chan face{}{}
	nms := []string{}
	trs := []*repl.Tree{}
	switch len(args) {
	case 0:
		nms = names()
	case 1:
		nms = []string{args[0]}
	default:
		opts.Usage()
	}
	for _, nm := range nms {
		rc := make(chan face{}, 32)
		rcs = append(rcs, rc)
		trs = append(trs, sync1(nm, rc))
	}
	for i, nm := range nms {
		cmd.Printf("sync %s\n", nm)
		for x := range rcs[i] {
			switch x := x.(type) {
			case repl.Chg:
				cmd.Printf("chg %s %s\n", x.At, x)
			case error:
				cmd.Warn("%s: %s\n", nm, x)
				if err == nil {
					err = x
				}
			}
		}
		if err := cerror(rcs[i]); err != nil {
			cmd.Warn("%s: %s", nm, err)
		}
	}
	for _, tr := range trs {
		if tr != nil {
			tr.Close()
		}
	}
	cmd.Exit(err)
}
