/*
	Create a ZX dump for UNIX files.
*/
package main

import (
	"clive/app/opt"
	"clive/dbg"
	"clive/zx"
	"clive/zx/lfs"
	"os"
	"path"
	"strings"
)

var (
	Debug, Verbose bool
	Dump           string
	Xcludes        []string
	Once           bool
	Skip           bool

	opts    = opt.New("{file|name!file}")
	vprintf = dbg.FlagPrintf(os.Stderr, &Verbose)
	dprintf = dbg.FlagPrintf(os.Stderr, &Debug)
)

func main() {
	defer dbg.Exits("")
	os.Args[0] = "zxdump"
	dfltdump := zx.Path(dbg.Home, "dump")
	opts.NewFlag("s", "don't dump right now, wait until next at 5am", &Skip)
	opts.NewFlag("1", "dump once and exit", &Once)
	opts.NewFlag("v", "verbose", &Verbose)
	opts.NewFlag("D", "debug", &Debug)
	opts.NewFlag("x", "expr: files excluded (.*, tmp.* if none given); tmp always excluded.", &Xcludes)
	Dump = dfltdump
	opts.NewFlag("d", "dir: where to keep the dump, or empty if none", &Dump)
	args, err := opts.Parse(os.Args)
	if err != nil {
		dbg.Warn("%s", err)
		opts.Usage()
		dbg.Exits(err)
	}
	if len(Xcludes) == 0 {
		Xcludes = []string{".*", "tmp.*", "*.tmp"}
	}
	Xcludes = append(Xcludes, "tmp")
	if len(args) == 0 {
		dbg.Warn("arguments missing")
		opts.Usage()
		dbg.Exits("usage")
	}
	if Skip && Once {
		dbg.Fatal("can't skip the current dump and dump once now")
	}
	nt := 0
	ec := make(chan bool)
	for i := 0; i < len(args); i++ {
		al := strings.SplitN(args[i], "!", 2)
		if len(al) == 1 {
			al = append(al, al[0])
			al[0] = path.Base(al[0])
		}
		t, err := lfs.New(al[0], al[1], lfs.RO)
		if err != nil {
			dbg.Warn("%s: %s", al[0], err)
			continue
		}
		t.ReadAttrs(true)
		nt++
		go dump(Dump, t, ec)
	}
	if nt == 0 {
		dbg.Fatal("no trees to dump")
	}
	for nt > 0 {
		<-ec
		nt--
	}
}
