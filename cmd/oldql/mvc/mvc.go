/*
	Ql builtin and external mvz command.
	move zx files

*/
package mvc

/*
	Must work with these paths like cpc
		mvc -Dn /zx/sys/src/clive/cmd/ql/../ql/mvc, /tmp/a
		mvc -Dn ./, /tmp/a
		mvc -Dn , .
		mvc -Dn ../mvc, /tmp/a
		mvc -Dn mvc.go /tmp/a/mvc.go
		mvc -Dn mvc.go /tmp/a

	BUT: it is an error to specify names such that they match
	subtrees (i.e., if a/b/c is matched, it is an error if a/b/c/d is matched,
	because it is already moved).
*/

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

type xCmd  {
	*cmd.Ctx
	*opt.Flags
	src, dst                      string
	debug, verb, dry, aflag, cpok bool
	many                          bool
	dprintf, vprintf              dbg.PrintFunc
}

// This sets x.dst to abs(dst), but lets the user rely on find
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

func (x *xCmd) inconsistent(src, dst string) bool {
	src, _ = filepath.Abs(src)
	return zx.HasPrefix(dst, src)
}

func isCrossDevice(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "cross") && strings.Contains(msg, "device")
}

func (x *xCmd) cprm(sfs, dfs zx.RWTree, spath, dpath string, d zx.Dir) error {
	if d["type"] == "" {
		xd, err := zx.Stat(sfs, spath)
		if err != nil {
			dbg.Warn("%s", err)
			return err
		}
		d = xd
	}
	nd := zx.Dir{"mode": d["mode"]}
	if x.aflag {
		nd = d.UsrAttrs()
	}
	if d["type"] == "d" {
		x.vprintf("mkdir %s\n", dpath)
		if err := <-sfs.Mkdir(dpath, nd); err!=nil && !dbg.IsExists(err) {
			dbg.Warn("%s", err)
			return err
		}
		cds, err := zx.GetDir(sfs, spath)
		if err != nil {
			dbg.Warn("%s", err)
			return err
		}
		var sts error
		for _, cd := range cds {
			cspath := zx.Path(spath, cd["name"])
			cdpath := zx.Path(dpath, cd["name"])
			if err := x.cprm(sfs, dfs, cspath, cdpath, cd); err != nil {
				dbg.Warn("%s", err)
				sts = err
				continue
			}
		}
		if sts == nil {
			x.vprintf("rmc %s\n", spath)
			sts = <-sfs.Remove(spath)
			if sts != nil {
				dbg.Warn("remove: %s", err)
			}
		}
		return sts
	}
	x.vprintf("cpc %s %s\n", spath, dpath)
	dc := sfs.Get(spath, 0, zx.All, "")
	if err := cerror(dc); err != nil {
		dbg.Warn("%s", err)
		return err
	}
	rc := dfs.Put(dpath, nd, 0, dc, "")
	<-rc
	err := cerror(rc)
	if err != nil {
		dbg.Warn("put: %s", err)
	} else {
		x.vprintf("rmc %s\n", spath)
		if err = <-sfs.Remove(spath); err != nil {
			dbg.Warn("remove: %s", err)
		}
	}
	return err
}

func (x *xCmd) mv(d zx.Dir) error {
	name := d["path"]
	rel := name[len(x.src):]
	dst := zx.Path(x.dst, rel)
	x.vprintf("mvc %s %s\n", d["path"], dst)
	name, _ = filepath.Abs(name)

	var dstfs, srcfs zx.RWTree
	var dpref, dstspath, spref, srcspath string
	if pref, ts, ps, err := cmd.ResolveTree(dst); err != nil {
		dbg.Warn("resolve: %s", err)
		return err
	} else {
		dpref = pref
		dstfs = ts[0]
		dstspath = ps[0]
	}

	if pref, ts, ps, err := cmd.ResolveTree(name); err != nil {
		return err
	} else {
		spref = pref
		srcfs = ts[0]
		srcspath = ps[0]
	}
	x.dprintf("spref %s spath %s dpref %s spath %s\n",
		spref, srcspath, dpref, dstspath)
	_, _ = dstfs, srcfs
	if x.dry {
		return nil
	}

	err := <-srcfs.Move(srcspath, dstspath)
	if err != nil {
		if isCrossDevice(err) && x.cpok {
			return x.cprm(srcfs, dstfs, srcspath, dstspath, d)
		}
		dbg.Warn("%s", err)
	}
	return err
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
	x.NewFlag("m", "behave like when moving many files", &x.many)
	x.NewFlag("c", "copy and remove to cross devices", &x.cpok)
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
	x.many = x.many || len(srcs)>1
	for _, src := range srcs {
		select {
		case <-x.Intr():
			return errors.New("interrupted")
		default:
		}
		stoks := strings.SplitN(src, ",", 2)
		x.src = path.Clean(stoks[0])
		if x.many && len(stoks)==1 {
			x.src = path.Dir(x.src)
		}
		if x.src == "." {
			x.src = ""
		}
		if x.inconsistent(x.src, x.dst) {
			x.Warn("inconsistent move: %s into %s", src, x.dst)
			sts = errors.New("errors")
			continue
		}
		if len(stoks) == 1 {
			d := zx.Dir{"path": stoks[0]}
			if err := x.mv(d); err != nil {
				sts = errors.New("errors")
			}
			continue
		}
		dc := cmd.Files(src)
		doselect {
		case d, ok := <-dc:
			if !ok || d==nil {
				break
			}
			if err := x.mv(d); err != nil {
				sts = errors.New("errors")
			}
		case <-x.Intr():
			close(dc, "interrupted")
			break
		}
		if err := cerror(dc); err != nil {
			x.Warn("%s: %s", src, err)
			sts = errors.New("errors")
		}
	}
	return sts
}
