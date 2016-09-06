/*
	make a replica for zx trees
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx/repl"
	fpath "path"
	"os"
	"io/ioutil"
	"strings"
)

func list(name string) error {
	if !strings.ContainsRune(name, '/') {
		name = "/u/lib/repl/" + name
	}
	tr, err := repl.Load(name)
	if err != nil {
		return err
	}
	defer tr.Close()
	c := cmd.AppCtx()
	if c.Verb {
		tr.Ldb.DumpTo(os.Stderr)
		tr.Rdb.DumpTo(os.Stderr)
	} else {
		xs := strings.Join(tr.Ldb.Excl, " ")
		cmd.Printf("%s %s %s %s\n", tr.Ldb, tr.Ldb.Addr, tr.Rdb.Addr, xs)
	}
	return nil
}

func mk(file, ldir, rdir string) error {
	if !strings.ContainsRune(file, '/') {
		file = "/u/lib/repl/" + file
	}
	tr, err:= repl.New(fpath.Base(file), ldir, rdir, excl...)
	if err != nil {
		return err
	}
	defer tr.Close()
	c := cmd.AppCtx()
	tr.Ldb.Debug = c.Debug
	tr.Rdb.Debug = c.Debug
	if err := tr.Save(file); err != nil {
		return err
	}
	if c.Verb {
		tr.Ldb.DumpTo(os.Stderr)
		tr.Rdb.DumpTo(os.Stderr)
		cmd.VWarn("saved %s", file+".ldb")
		cmd.VWarn("saved %s", file+".rdb")
	}
	return nil
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
	opts = opt.New("[file [ldir rdir]]")
	excl []string
	notux bool
)

func main() {
	cmd.UnixIO("err")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("v", "verbose", &c.Verb)
	opts.NewFlag("x", "exclude", &excl)
	opts.NewFlag("u", "don't use unix out", &notux)
	args := opts.Parse()
	if !notux {
		cmd.UnixIO("out")
	}
	var err error
	switch len(args) {
	case 0:
		for _, nm := range names() {
			if err2 := list(nm); err2 != nil {
				cmd.Warn("%s: %s", nm, err2)
				if err == nil {
					err = err2
				}
			}
		}
	case 1:
		err = list(args[0])
	case 3:
		err = mk(args[0], args[1], args[2])
	default:
		opts.Usage()
	}
	if err != nil {
		cmd.Fatal(err)
	}
	cmd.Exit(nil)
}
