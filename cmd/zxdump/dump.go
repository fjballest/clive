/*
	Create a ZX dump for UNIX files.
*/
package main

// zxdump -Dv -1 -d /tmp/dump /Users/nemo/gosrc/src/clive/cmd
// rm -rf /tmp/dump

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/dbg"
	"clive/u"
	"clive/zx/zux"
	fpath "path"
	"strings"
)

var (
	Dump    string
	Xcludes []string
	Once    bool
	Skip    bool

	opts    = opt.New("{file|name!file}")
	vprintf = cmd.VWarn
	dprintf = cmd.Dprintf
)

func main() {
	cmd.UnixIO()
	c := cmd.AppCtx()
	dfltdump := Path(u.Home, "dump")
	opts.NewFlag("s", "don't dump right now, wait until next at 5am", &Skip)
	opts.NewFlag("1", "dump once and exit", &Once)
	opts.NewFlag("v", "verbose", &c.Verb)
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("x", "expr: files excluded (.*, tmp.* if none given); tmp always excluded.", &Xcludes)
	Dump = dfltdump
	opts.NewFlag("d", "dir: where to keep the dump, ~/dump if none", &Dump)
	args := opts.Parse()
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
		go dump(Dump, t.Tag, t, ec)
	}
	if nt == 0 {
		cmd.Fatal("no trees to dump")
	}
	for nt > 0 {
		<-ec
		nt--
	}
}
