/*
	Ql builtin and external pc command.
	print files
*/
package pc

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/dbg"
	"clive/zx"
)

type xCmd  {
	*cmd.Ctx
	*opt.Flags
	debug   bool
	dprintf dbg.PrintFunc
}

func (x *xCmd) RunFile(d zx.Dir, dc <-chan []byte) error {
	name := d["path"]
	if dc == nil {
		return nil
	}
	x.dprintf("pz runfile %s\n", name)
	for data := range dc {
		x.dprintf("pz runfile: [%d]\n", len(data))
		_, err := x.Stdout.Write(data)
		if err != nil {
			x.dprintf("pz write: %s\n", err)
			close(dc, err)
			return err
		}
	}
	err := cerror(dc)
	x.dprintf("pz %s sts %v\n", name, err)
	return err
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("{file}")
	x.Argv0 = argv[0]
	x.dprintf = dbg.FlagPrintf(x.Stderr, &x.debug)
	x.NewFlag("D", "debug", &x.debug)
	cmd.Debug = x.debug
	args, err := x.Parse(argv)
	if err != nil {
		x.Usage(x.Stderr)
		return err
	}
	if cmd.Ns == nil {
		cmd.MkNS()
	}
	return cmd.RunFiles(x, args...)
}
