/*
	Ql builtin and external lf command.
	List files
*/
package lf

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/dbg"
	"clive/zx"
	"errors"
	"path"
	"path/filepath"
	"strings"
)

type xCmd struct {
	*cmd.Ctx
	*opt.Flags
	rflag, aflag, vflag, lflag, dflag, pflag bool
	debug                                    bool
	dprintf                                  dbg.PrintFunc
}

func (x *xCmd) ld(d zx.Dir) {
	if x.pflag {
		d["path"] = path.Base(d["path"])
	}
	switch {
	case x.aflag:
		x.Printf("%s\n", d)
	case x.vflag:
		x.Printf("%s\n", d.Verb())
	case x.lflag:
		x.Printf("%s\n", d.Long())
	default:
		x.Printf("%s\n", d["path"])
	}
}

func (x *xCmd) run(name string) error {
	x.dprintf("lz %s\n", name)
	toks := strings.SplitN(name, ",", 2)
	if toks[0] == "" {
		toks[0] = "."
	}
	nilpred := len(toks) == 1
	if nilpred {
		toks = toks[:1]
		if x.rflag {
			toks = append(toks, "")
		} else {
			toks = append(toks, "depth<2")
		}
	}
	name, _ = filepath.Abs(toks[0])
	dc := cmd.Ns.Find(name, toks[1], "/", "/", 0)
	i := 0
	doselect {
	case <-x.Intrc:
		close(dc, "interrupted")
		return errors.New("interrupted")
	case d := <-dc:
		if d == nil {
			break
		}
		if toks[0] != name && zx.HasPrefix(d["path"], name) {
			u := zx.Path(toks[0], zx.Suffix(d["path"], name))
			d["path"] = u
		}
		if d["err"] != "" {
			if d["err"] != "pruned" {
				x.Eprintf("%s: %s", d["path"], d["err"])
				continue
			}
		}
		if nilpred {
			if i == 0 && !x.dflag {
				i++
				continue
			}
			if i > 0 && x.dflag {
				close(dc, "done")
				return nil
			}
		}
		x.ld(d)
		i++
	}
	if err := cerror(dc); err != nil {
		return err
	}
	close(dc)
	return nil
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("{file}")
	x.Argv0 = argv[0]
	x.dprintf = dbg.FlagPrintf(x.Stderr, &x.debug)
	x.NewFlag("D", "debug", &x.debug)
	x.NewFlag("a", "list all attributes", &x.aflag)
	x.NewFlag("r", "recur through directories", &x.rflag)
	x.NewFlag("l", "long listing", &x.lflag)
	x.NewFlag("p", "print just the name, not the full path", &x.pflag)
	x.NewFlag("v", "verbose listing, show all user attributes", &x.vflag)
	x.NewFlag("d", "list dirs, not their contents", &x.dflag)
	args, err := x.Parse(argv)
	if err != nil {
		x.Usage(x.Stderr)
		return err
	}
	if len(args) == 0 {
		args = append(args, ".")
	}
	if cmd.Ns == nil {
		cmd.MkNS()
	}
	var sts error
	for _, a := range args {
		if err := x.run(a); err != nil {
			x.Warn("%s: %s", a, err)
			sts = errors.New("errors")
		}
	}
	return sts
}
