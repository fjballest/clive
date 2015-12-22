package sync

import (
	"clive/nchan"
	"clive/zx"
	"errors"
)

func perr(ec chan<- error, err error) error {
	if ec != nil && err != nil {
		ec <- err
	}
	return err
}

// Apply the given change to the local tree, using the remote tree
// to gather data if necessary.
// The db is not updated. Once everything is applied, it should be updated.
// If errorc is not nil, errors are posted there; anyway the first error is returned.
func (ch Chg) Apply(lfs zx.RWTree, rfs zx.Tree, pred string, ec chan<- error) error {
	if ch.D["err"] != "" {
		return nil
	}
	switch ch.Type {
	case None:
		return nil
	case Meta:
		return perr(ec, ch.applyMeta(lfs))
	case Data:
		return perr(ec, ch.applyData(lfs, rfs))
	case Del:
		return perr(ec, ch.applyDel(lfs))
	case Add:
		return ch.applyAdd(lfs, rfs, pred, ec)
	case DirFile:
		// get rid of the old and add the new
		nch := ch
		nch.Type = Del
		err := nch.Apply(lfs, rfs, pred, ec) // and ignore the error
		nch.Type = Add
		ne := nch.Apply(lfs, rfs, pred, ec)
		if err == nil {
			err = ne
		}
		return err
	default:
		panic("unknown change")
	}
}

func (ch Chg) applyMeta(lfs zx.RWTree) error {
	nd := ch.D.UsrAttrs()
	for _, k := range ignoredPutAttrs {
		delete(nd, k)
	}
	ec := lfs.Wstat(ch.D["path"], nd)
	return <-ec
}

func (ch Chg) applyData(lfs zx.RWTree, rfs zx.Tree) error {
	nd := ch.D.UsrAttrs()
	for _, k := range ignoredPutAttrs {
		delete(nd, k)
	}
	datc := rfs.Get(ch.D["path"], 0, zx.All, "")
	dc := lfs.Put(ch.D["path"], nd, 0, datc, "")
	<-dc
	return cerror(dc)
}

func (ch Chg) applyDel(lfs zx.RWTree) error {
	return <-lfs.RemoveAll(ch.D["path"])
}

func (ch Chg) applyAdd(lfs zx.RWTree, rfs zx.Tree, pred string, ec chan<- error) error {
	var err error
	gc := rfs.FindGet(ch.D["path"], pred, "", "", 0)
	for g := range gc {
		dprintf("get %s\n", g)
		d := g.Dir
		if d == nil {
			break
		}
		for _, k := range ignoredPutAttrs {
			delete(d, k)
		}
		if d["err"] != "" {
			e := errors.New(d["err"])
			perr(ec, e)
			dprintf("%s: %s\n", d["path"], d["err"])
			if err == nil {
				err = e
			}
			continue
		}
		if g.Datac == nil && d["type"] != "d" {
			g.Datac = nchan.Null
		}
		if d["type"] == "d" {
			e := <-lfs.Mkdir(d["path"], d)
			if e != nil {
				perr(ec, e)
				if err == nil {
					err = e
				}
			}
			continue
		}
		dc := lfs.Put(d["path"], d, 0, g.Datac, "")
		<-dc
		if e := cerror(dc); e != nil {
			dprintf("%s: put: %s\n", d["path"], e)
			perr(ec, e)
			if err == nil {
				err = e
			}
		}
	}
	close(gc)
	if e := cerror(gc); e != nil {
		dprintf("get: %s\n", e)
		perr(ec, e)
		if err == nil {
			err = e
		}
	}
	return err
}
