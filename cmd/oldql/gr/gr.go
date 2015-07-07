/*
	Ql builtin and external lz command.
	grep zx files
*/
package gr

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/dbg"
	"clive/nchan"
	"clive/sre"
	"clive/zx"
	"errors"
)

type xCmd  {
	*cmd.Ctx
	*opt.Flags
	debug, sflag, nflag, vflag bool
	found                      bool
	re                         *sre.ReProg
	printf                     dbg.PrintFunc
	dprintf                    dbg.PrintFunc
}

func (x *xCmd) RunFile(d zx.Dir, dc <-chan []byte) error {
	name := d["path"]
	if dc == nil {
		return nil
	}
	re := x.re
	rc := nchan.Lines(dc, '\n')
	nln := 0
	doselect {
	case <-x.Intrc:
		close(rc, "interrupted")
		return errors.New("interrupted")
	case s, ok := <-rc:
		if !ok {
			return cerror(rc)
		}
		nln++
		rg := re.ExecStr(s, 0, -1)
		if rg==nil && !x.vflag || rg!=nil && x.vflag {
			continue
		}
		x.found = true
		if x.nflag {
			x.printf("%s", s)
		} else {
			x.printf("%s:%d: %s", name, nln, s)
		}
	}
	if err := cerror(rc); err != nil {
		return err
	}
	return nil
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("rexp {file}")
	x.Argv0 = argv[0]
	x.dprintf = dbg.FlagPrintf(x.Stderr, &x.debug)
	x.NewFlag("D", "debug", &x.debug)
	x.NewFlag("s", "just status", &x.sflag)
	x.NewFlag("n", "no file/line print", &x.nflag)
	x.NewFlag("v", "invert match", &x.vflag)
	args, err := x.Parse(argv)
	if err != nil {
		x.Usage(x.Stderr)
		return err
	}
	t := !x.sflag
	x.printf = dbg.FlagPrintf(x.Stdout, &t)
	if len(args) < 1 {
		x.Usage(x.Stderr)
		return errors.New("usage")
	}
	re, err := sre.CompileStr(args[0], sre.Fwd)
	if err != nil {
		return err
	}
	args = args[1:]
	var sts error
	if cmd.Ns == nil {
		cmd.MkNS()
	}
	x.re = re
	sts = cmd.RunFiles(x, args...)
	if !x.found && sts==nil {
		return errors.New("no match")
	}
	return sts
}
