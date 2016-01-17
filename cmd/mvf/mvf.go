/*
	move files command.
	Not like the unix one.
	mvf a b -> moves a to b (*not* to b/a if b exists and is a dir)
	mvf d moves all dirs with paths .../name in stdin to be d/name, where d is the destination dir.
	mv a b... c takes c as the target parent dir for a, b... 
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx"
	fpath "path"
	"strings"
	"errors"
	"fmt"
)

var (
	opts = opt.New("[src] dst | src1 src2... dstdir")
	todir, dry, ux bool
)

func dst(name string) (zx.Dir, error) {
	if strings.Contains(name, ",") {
		return nil, errors.New("destination can't be a predicate")
	}
	d, err := cmd.Stat(name)
	if zx.IsNotExist(err) {
		path := cmd.AbsPath(name)
		ppath := fpath.Dir(path)
		pd, err := cmd.Stat(ppath)
		if err != nil {
			return nil, err
		}
		if pd["type"] != "d" {
			return nil, fmt.Errorf("%s: %s", pd["path"], zx.ErrNotDir)
		}
		base := fpath.Base(path)
		pd["path"] = fpath.Join(pd["path"], base)
		pd["Upath"] = name
		pd["Rpath"] = "/"
		if pd["addr"] != "" {
			pd["addr"] += "/" + base
		}
		return pd, nil
	}
	return d, nil
}

func mv1(src, dst zx.Dir) error {
	cmd.VWarn("%s %s", src["Upath"], dst["Upath"])
	cmd.Dprintf("mv1: %s %s %s %s\n", src.SAddr(), src["Rpath"], dst.SAddr(), dst["Rpath"])
	if dry {
		return nil
	}
	return cmd.Move(src["path"], dst["path"])
}

func mvf(in <-chan interface{}, dst zx.Dir, todir bool) error {
	var err error
	dpath := dst["path"]
	dupath := dst["Upath"]
	drpath := dst["Rpath"]
	daddr := dst.SAddr()
	for x := range in {
		switch d := x.(type) {
		case zx.Dir:
			cmd.Dprintf("got %T %s\n", d, d["Upath"])
			if daddr != d.SAddr() {
				cmd.Warn("%s: cross server move", d["Upath"])
				if err == nil {
					err = errors.New("cross server move")
				}
				continue
			}
			fd := dst.Dup()
			base := fpath.Base(d["Rpath"])
			if todir {
				base = fpath.Base(d["path"])
			}
			fd["path"] = fpath.Join(dpath, base)
			fd["Upath"] = fpath.Join(dupath, base)
			fd["Rpath"] = fpath.Join(drpath, base)
			if e := mv1(d, fd); e != nil {
				cmd.Warn("mv %s: %s", d["Upath"], e)
				if err == nil {
					err = e
				}
			}
		case error:
			cmd.Warn("%s", d)
		default:
			cmd.Dprintf("ignored %T\n", d)
		}
	}
	if err == nil {
		err = cerror(in)
	}
	if err != nil {
		cmd.Warn("%s", err)
	}
	return err
}

// Run mvf in the current app context.
func main() {
	c := cmd.AppCtx()
	cmd.UnixIO("err")
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("n", "dry run; report moves but do not make them", &dry)
	opts.NewFlag("v", "report moves", &c.Verb)
	opts.NewFlag("d", "take always destination as a parent dir", &todir)
	opts.NewFlag("u", "use unix output", &ux)
	args, err := opts.Parse()
	if err != nil {
		cmd.Warn("%s", err)
		opts.Usage()
	}
	if ux {
		cmd.UnixIO("out")
	}
	c.Verb = c.Verb || dry
	if len(args) == 0 {
		cmd.Warn("missing argument")
		opts.Usage()
	}
	dst, err := dst(args[len(args)-1])
	if err != nil {
		cmd.Fatal(err)
	}
	todir = todir || len(args) > 2
	if len(args) > 1 {
		args = args[:len(args)-1]
		cmd.SetIn("in", cmd.Dirs(args...))
	}
	err = mvf(cmd.In("in"), dst, todir)
	if err != nil {
		cmd.Exit(err)
	}
}
