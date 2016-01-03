/*
	Create a ZX dump for UNIX files.
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/u"
	"clive/dbg"
	"clive/zx/zux"
	fpath "path"
	"strings"
)

var (
	Debug, Verbose bool
	Dump           string
	Xcludes        []string
	Once           bool
	Skip           bool

	opts    = opt.New("{file|name!file}")
	vprintf = dbg.FlagPrintf(&Verbose)
	dprintf = dbg.FlagPrintf(&Debug)
)

func main() {	
	cmd.UnixIO()
	dfltdump := fpath.Join(u.Home, "dump")
	opts.NewFlag("s", "don't dump right now, wait until next at 5am", &Skip)
	opts.NewFlag("1", "dump once and exit", &Once)
	opts.NewFlag("v", "verbose", &Verbose)
	opts.NewFlag("D", "debug", &Debug)
	opts.NewFlag("x", "expr: files excluded (.*, tmp.* if none given); tmp always excluded.", &Xcludes)
	Dump = dfltdump
	opts.NewFlag("d", "dir: where to keep the dump, or empty if none", &Dump)
	args, err := opts.Parse()
	if err != nil {
		cmd.Warn("%s", err)
		opts.Usage()
	}
	if len(Xcludes) == 0 {
		Xcludes = []string{".*", "tmp.*", "*.tmp"}
	}
	Xcludes = append(Xcludes, "tmp")
	if len(args) == 0 {
		cmd.Warn("arguments missing")
		opts.Usage()
	}
	if Skip && Once {
		cmd.Fatal("can't skip the current dump and dump once now")
	}
	nt := 0
	ec := make(chan bool)
	for i := 0; i < len(args); i++ {
		al := strings.SplitN(args[i], "!", 2)
		if len(al) == 1 {
			al = append(al, al[0])
			al[0] = fpath.Base(al[0])
		}
		t, err := zux.NewZX(al[1])
		if err != nil {
			dbg.Warn("%s: %s", al[0], err)
			continue
		}
		t.Tag = al[0]
		t.Flags.Set("rdonly", true)
		nt++
		go dump(t.Tag, Dump, t, ec)
	}
	if nt == 0 {
		cmd.Fatal("no trees to dump")
	}
	for nt > 0 {
		<-ec
		nt--
	}
}
