package main

import (
	"clive/cmd/picky/comp"
	"clive/cmd/picky/paminstr"
	"flag"
	"fmt"
	"os"
	"runtime"
)

func main() {
	defer func() {
		if e := recover(); e != nil {
			comp.Goodbye(e)
		}
	}()
	flag.Parse()
	if comp.V {
		fmt.Printf("%s: version %s\n", os.Args[0], VERS) //TODO
		os.Exit(0)
	}
	comp.SetYYDebug()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	if runtime.GOOS == "windows" {
		paminstr.EOL = "\r\n"
	}
	args := flag.Args()
	comp.Scanner = comp.NewScanner(nil, "", -1)
	comp.Pushenv()
	comp.Syminit()
	comp.Typeinit()
	comp.Builtininit()
	for _, name := range args {
		e := comp.Processfile(name)
		if e != nil {
			comp.Goodbye(e)
		}
	}

	if comp.Sflag {
		comp.Dumpstats()
	}
	comp.Checkundefs()
	if comp.Nerrors > 0 {
		fmt.Fprint(os.Stderr, "errors\n")
		os.Exit(1)
	}
	nm := comp.Oname
	fout, out := comp.Mkout(nm)
	comp.Gen(out, nm)
	out.Flush()
	fout.Close()
	os.Chmod(nm, os.FileMode(0755))
	os.Exit(0)
}
