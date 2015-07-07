/*
	Ql builtin and external rmc command.
	remove zx files
*/
package rmc

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/dbg"
	"clive/zx"
	"errors"
)

type xCmd  {
	*cmd.Ctx
	*opt.Flags
	src, dst                string
	debug, verb, dry, force bool
	dprintf, vprintf        dbg.PrintFunc
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("{file}")
	x.Argv0 = argv[0]
	x.NewFlag("D", "debug", &x.debug)
	x.NewFlag("v", "report operations made", &x.verb)
	x.NewFlag("f", "force ignore of errors", &x.force)
	x.NewFlag("n", "dry run; implies -v", &x.dry)
	x.dprintf = dbg.FlagPrintf(x.Stderr, &x.debug)
	x.vprintf = dbg.FlagPrintf(x.Stdout, &x.verb)
	cmd.Debug = x.debug
	args, err := x.Parse(argv)
	if err != nil {
		x.Usage(x.Stderr)
		return err
	}
	x.verb = x.verb || x.dry
	if cmd.Ns == nil {
		cmd.MkNS()
	}
	if len(args) == 0 {
		x.Usage(x.Stderr)
		return errors.New("too few arguments")
	}
	// TODO: If arg is "foo,", then issue a RemoveAll() call for foo, using
	// resolve to locate its server.
	var sts error
	for _, arg := range args {
		dc := cmd.Files(arg)
		dirs := []zx.Dir{}
		for d := range dc {
			if d["err"] != "" {
				if !x.force {
					x.Warn("%s: %s", d["path"], err)
					sts = errors.New("errors")
				}
				continue
			}
			dirs = append(dirs, d)
		}
		if err := cerror(dc); err != nil {
			if !x.force {
				x.Warn("%s: %s", arg, err)
				sts = errors.New("errors")
			}
			continue
		}
		rcs := make([]chan error, 0, len(dirs))
		for i := len(dirs) - 1; i >= 0; i-- {
			d := dirs[i]
			ts, err := zx.RWDirTree(d)
			if err != nil {
				if !x.force {
					x.Warn("%s: %s", d["path"], err)
					sts = err
				}
				continue
			}
			x.vprintf("rm %s\n", d["path"])
			if !x.dry {
				rcs = append(rcs, ts.Remove(d["spath"]))
			}
		}
		for _, rc := range rcs {
			if err := <-rc; err!=nil && !x.force {
				x.Warn("%s", err)
				sts = err
			}
		}
	}
	return sts
}
