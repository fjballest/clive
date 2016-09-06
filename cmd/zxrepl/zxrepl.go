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
	"strings"
)

var (
	opts = opt.New("file [ldir rdir]")
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
	if len(args) == 1 {
		tr, err := repl.Load(args[0])
		if err != nil {
			cmd.Fatal(err)
		}
		tr.Ldb.DumpTo(os.Stderr)
		tr.Rdb.DumpTo(os.Stderr)
		tr.Close()
		cmd.Exit(nil)
	}
	if len(args) != 3 {
		opts.Usage()
	}
	file, ldir, rdir := args[0], args[1], args[2]
	if !notux {
		cmd.UnixIO("out")
	}
	if !strings.ContainsRune(file, '/') {
		file = "/u/lib/repl/" + file
	}
	tr, err:= repl.New(fpath.Base(file), ldir, rdir, excl...)
	if err != nil {
		cmd.Fatal(err)
	}
	tr.Ldb.Debug = c.Debug
	tr.Rdb.Debug = c.Debug
	if err := tr.Save(file); err != nil {
		cmd.Fatal(err)
	}
	if c.Verb {
		tr.Ldb.DumpTo(os.Stderr)
		tr.Rdb.DumpTo(os.Stderr)
		cmd.VWarn("saved %s", file+".ldb")
		cmd.VWarn("saved %s", file+".rdb")
	}
	tr.Close()
	cmd.Exit(nil)
}
