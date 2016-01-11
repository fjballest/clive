package zux

import (
	"os"
	"clive/zx"
	"fmt"
	fpath "path"
)

func (fs *Fs) chkWalk(p string, mkall bool) error {
	if fs.ai == nil {
		return nil
	}
	els := zx.Elems(p)
	rp := "/"
	for len(els) > 0 {
		d, err := fs.stat(rp, false)
		rp = fpath.Join(rp, els[0])
		if err != nil {
			if !mkall || !zx.IsNotExist(err) {
				return err
			}
			path := fpath.Join(fs.root, rp)
			if e := os.Mkdir(path, 0755); e != nil {
				return err
			}
		}
		if !d.CanWalk(fs.ai) {
			return fmt.Errorf("%s: %s", d["path"], zx.ErrPerm)
		}
		if len(els) == 1 {
			return nil
		}
		els = els[1:]
	}
	return nil
}

func (fs *Fs) chkGet(p string) error {
	if fs.ai == nil {
		return nil
	}
	if err := fs.chkWalk(p, false); err != nil {
		return err
	}
	d, err := fs.stat(p, false)
	if err != nil {
		return err
	}
	if !d.CanGet(fs.ai) {
		return fmt.Errorf("%s: %s", p, zx.ErrPerm)
	}
	return nil
}

func (fs *Fs) chkPut(p string, mkall bool) error {
	if fs.ai == nil {
		return nil
	}
	if err := fs.chkWalk(p, mkall); err != nil {
		return err
	}
	d, err := fs.stat(p, false)
	if err != nil {
		return err
	}
	if !d.CanPut(fs.ai) {
		return fmt.Errorf("%s: %s", p, zx.ErrPerm)
	}
	return nil
}

func (fs *Fs) chkWstat(p string, nd zx.Dir) error {
	if fs.ai == nil {
		return nil
	}
	if err := fs.chkWalk(p, false); err != nil {
		return err
	}
	d, err := fs.stat(p, false)
	if err != nil {
		return err
	}
	if err := d.CanWstat(fs.ai, nd); err != nil {
		return fmt.Errorf("%s: %s", p, err)
	}
	return nil
}
