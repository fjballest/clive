/*
	Clean and print path or path elements
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	fpath "path"
	"path/filepath"
)

var (
	ux, dflag, bflag bool

	dir  string
	opts = opt.New("{name}")
)

// Run path in the current app context.
func main() {
	cmd.UnixIO("err")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("r", "dir: print paths relative to dir", &dir)
	opts.NewFlag("d", "print parent directories", &dflag)
	opts.NewFlag("b", "print base names", &bflag)
	args := opts.Parse()
	if ux {
		cmd.UnixIO()
	}
	if dir != "" && (dflag || bflag) {
		cmd.Warn("incompatible flags")
		opts.Usage()
	}
	if dflag && bflag {
		cmd.Warn("incompatible flags")
		opts.Usage()
	}
	if dir != "" {
		dir = cmd.AbsPath(dir)
	}
	var sts error
	for _, n := range args {
		n = cmd.AbsPath(n)
		switch {
		case dir != "":
			r, err := filepath.Rel(dir, n)
			if err != nil {
				sts = err
				cmd.Warn("%s: %s", n, err)
			} else {
				n = r
			}
		case dflag:
			n = fpath.Dir(n)
		case bflag:
			n = fpath.Base(n)
		}
		cmd.Printf("%s\n", n)
	}
	cmd.Exit(sts)
}
