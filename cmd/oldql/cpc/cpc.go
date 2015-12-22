/*
	Ql builtin and external cpc command.
	copy zx files

	named cpc in honor of CPC 6128

*/
package cpc

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/dbg"
	"clive/zx"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

type xCmd struct {
	*cmd.Ctx
	*opt.Flags
	src, dst         string
	debug, verb, dry bool
	many, aflag      bool
	dprintf, vprintf dbg.PrintFunc
}

// This sets x.dst to abs(dst), but let's the user rely on find
// to specifiy a predicate for dst so that it returns a single matching file
func (x *xCmd) setdst(dst string) error {
	var err error
	toks := strings.SplitN(dst, ",", 2)
	if toks[0] == "" {
		toks[0] = "."
	}
	toks[0], _ = filepath.Abs(toks[0])
	if len(toks) == 1 {
		x.dst = toks[0]
		return nil
	}

	if toks[1] == "" {
		toks[1] = "depth<1"
	}
	dstds := []zx.Dir{}
	dstc := cmd.Ns.Find(toks[0], toks[1], "/", "/", 0)
	for d := range dstc {
		if d["err"] != "" {
			err = fmt.Errorf("%s: %s", d["path"], d["err"])
			continue
		}
		dstds = append(dstds, d)
	}
	if err == nil {
		err = cerror(dstc)
	}
	if err != nil {
		dbg.Warn("%s", err)
		return errors.New("errors")
	}
	if len(dstds) != 1 {
		return errors.New("too many destinations")
	}
	x.dst = dstds[0]["path"]
	return nil
}

func (x *xCmd) RunFile(d zx.Dir, dc <-chan []byte) error {
	name := d["path"]
	rel := name[len(x.src):]
	dst := zx.Path(x.dst, rel)
	x.dprintf("cpc name '%s' isdir %v dst '%s'\n", name, dc == nil, dst)
	nd := zx.Dir{"mode": d["mode"]}
	if x.aflag {
		nd = d.UsrAttrs()
	}
	_, ts, spaths, err := cmd.ResolveTree(dst)
	if err != nil {
		x.dprintf("resolve: %s: %s\n", dst, err)
		return err
	}
	t := ts[0]
	spath := spaths[0]
	if dc == nil {
		x.vprintf("mkdir -p %s\n", dst)
		x.dprintf("\tspath %s\n", spath)
		if x.dry {
			return nil
		}
		errc := t.Mkdir(spath, nd)
		err := <-errc
		if dbg.IsExists(err) {
			x.dprintf("\tignored: %s\n", err)
			err = nil
		}
		return err
	}
	x.vprintf("cpc %s %s\n", name, dst)
	x.dprintf("\tspath %s\n", spath)
	if x.dry {
		tot := 0
		if dc != nil {
			for x := range dc {
				tot += len(x)
			}
		}
		err := cerror(dc)
		x.dprintf("%d bytes sts %v\n", tot, err)
		return err
	}
	rc := t.Put(spath, nd, 0, dc, "")
	rd := <-rc
	err = cerror(rc)
	x.dprintf("put: sts %v d= %s\n", err, rd)
	return err
}

func (x *xCmd) inconsistent(src, dst string) bool {
	src, _ = filepath.Abs(src)
	return zx.HasPrefix(dst, src)
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("{file} file")
	x.Argv0 = argv[0]
	x.NewFlag("D", "debug", &x.debug)
	x.NewFlag("v", "report operations made", &x.verb)
	x.NewFlag("n", "dry run; implies -v", &x.dry)
	x.NewFlag("a", "preserve attributes", &x.aflag)
	x.NewFlag("m", "behave like when copying many files", &x.many)
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
	if len(args) < 2 {
		x.Usage(x.Stderr)
		return errors.New("too few arguments")
	}

	dst := args[len(args)-1]
	if err := x.setdst(dst); err != nil {
		return err
	}

	srcs := args[:len(args)-1]
	var sts error
	x.many = x.many || len(srcs) > 1
	for _, src := range srcs {
		stoks := strings.SplitN(src, ",", 2)
		x.src = path.Clean(stoks[0])
		if x.many && len(stoks) == 1 {
			x.src = path.Dir(x.src)
		}
		if x.src == "." {
			x.src = ""
		}
		if x.inconsistent(x.src, x.dst) {
			x.Warn("inconsistent copy: %s into %s", src, x.dst)
			sts = errors.New("errors")
		} else if err := cmd.RunFiles(x, src); err != nil {
			sts = errors.New("errors")
		}
	}
	return sts
}
