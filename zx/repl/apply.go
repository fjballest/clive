package repl

import (
	"clive/zx"
	"clive/cmd"
	fpath "path"
	"errors"
)

// Apply a series of changes from where (perhaps both)
func ApplyAll(ldb, rdb *DB, cc <-chan Chg, from Where) error {
	var err error
	for c := range cc {
		if from == Both || c.At == from {
			if err2 := Apply(ldb, rdb, c); err == nil {
				err = err2
			}
		}
	}
	if err == nil {
		err = cerror(cc)
	}
	return err
}

// Apply a single change (either pull or push).
// If the change has errors noted in it, it's ignored.
func Apply(ldb, rdb *DB, c Chg) error {
	if c.D["err"] != "" {
		return nil
	}
	if isExcl(c.D["path"], ldb.Excl...) || isExcl(c.D["path"], rdb.Excl...) {
		return nil
	}
	if c.At == Local {
		ldb, rdb = rdb, ldb
	}
	// ldb is now the target and ldb is now the source; always.

	switch c.Type {
	case None:
		return nil
	case Meta:
		return ldb.applyMeta(c)
	case Data:
		return ldb.applyData(c, rdb.Fs, rdb.rpath)
	case Add:
		return ldb.applyAdd(c, rdb.Fs, rdb.rpath)
	case Del:
		return ldb.applyDel(c)
	case DirFile:
		// get rid of the old and add the new
		nc := c
		nc.Type = Del
		err := ldb.applyDel(nc)
		if err2 := ldb.applyAdd(c, rdb.Fs, rdb.rpath); err == nil {
			err = err2;
		}
		return err
	default:
		panic("unknown change")
	}
}

func (db *DB) applyMeta(c Chg) error {
	fs, ok := db.Fs.(zx.Wstater)
	if !ok {
		return errors.New("Fs can't wstat");
	}
	db.Dprintf("meta %s\n", c.D.Fmt())
	p := fpath.Join(db.rpath, c.D["path"])
	rdc := fs.Wstat(p, c.D)
	rd := <-rdc
	if rd == nil {
		return cerror(rdc)
	}
	rd["path"] = c.D["path"]
	rd["name"] = c.D["name"]
	return db.Add(rd)
}

func (db *DB) applyDel(c Chg) error {
	if c.D["path"] == "/" {
		return errors.New("won't del /")
	}
	fs, ok := db.Fs.(zx.Remover)
	if !ok {
		return errors.New("Fs can't remove");
	}
	p := fpath.Join(db.rpath, c.D["path"])
	db.Dprintf("del %s\n", c.D.Fmt())
	err := <- fs.RemoveAll(p)
	if err != nil && !zx.IsNotExist(err) {
		return err
	}
	f, err := db.Walk(fpath.Dir(c.D["path"]))
	if err == nil {
		err = f.Remove(c.D["name"])
	}
	if zx.IsNotExist(err) {
		err = nil
	}
	return err
}

struct pfile {
	fs zx.Putter
	d zx.Dir
	dc chan<- []byte
	rc <-chan zx.Dir
}

func (pf *pfile) start(pfs zx.Putter, rpath string, d zx.Dir) {
	pf.fs = pfs
	pf.d = d.Dup()
	dc := make(chan []byte)
	if pf.d["type"] != "-" {
		close(dc)
	}
	pf.dc = dc
	pf.rc = pfs.Put(fpath.Join(rpath, d["path"]), d, 0, dc)
}

func (pf *pfile) done() error {
	if pf.dc == nil {
		return nil
	}
	close(pf.dc)
	pf.dc = nil
	rd := <-pf.rc
	err := cerror(pf.rc)
	pf.rc = nil
	for k, v := range rd {
		if k != "path" && k != "name" {
			pf.d[k] = v
		}
	}
	if err != nil {
		pf.d["err"] = err.Error()
	}
	return err
}

func (db *DB) applyAdd(c Chg, fs zx.Fs, rpath string) error {
	gfs, ok := fs.(zx.FindGetter)
	if !ok {
		return errors.New("fs can't findget")
	}
	pfs, ok := db.Fs.(zx.Putter)
	if !ok {
		return errors.New("fs can't put")
	}
	fc := gfs.FindGet(fpath.Join(rpath, c.D["path"]), "", rpath, "/", 0)
	pf := &pfile{}
	for m := range fc {
		switch d := m.(type) {
		case zx.Dir:
			d = d.Dup()
			db.Dprintf("add %s\n", d.Fmt())
			if err := pf.done(); err != nil {
				cmd.Warn("add %s: %s", pf.d["path"], err)
			}
			if isExcl(d["path"], db.Excl...) {
				continue
			}
			if pf.d != nil {
				db.Add(pf.d)
			}
			pf.start(pfs, db.rpath, d)
			db.Add(pf.d)
		case []byte:
			if (pf.dc == nil) {
				continue
			}
			if ok := pf.dc <- d; !ok {
				err := cerror(pf.dc)
				if err == nil {
					err = errors.New("can't put")
				}
				cmd.Warn("add %s: %s", pf.d["path"], err)
				pf.done()
				pf.d["err"] = err.Error()
				db.Add(pf.d)
			}
		}
	}
	if err := pf.done(); err != nil {
		db.Add(pf.d)
	}
	return cerror(fc)
}

func (db *DB) applyData(c Chg, fs zx.Fs, rpath string) error {
	gfs, ok := fs.(zx.Getter)
	if !ok {
		return errors.New("fs can't get")
	}
	pfs, ok := db.Fs.(zx.Putter)
	if !ok {
		return errors.New("fs can't put")
	}
	db.Dprintf("data %s\n", c.D.Fmt())
	dc := gfs.Get(fpath.Join(rpath, c.D["path"]), 0, zx.All)
	pc := pfs.Put(fpath.Join(db.rpath, c.D["path"]), c.D, 0, dc)
	rd := <-pc
	if rd == nil {
		return cerror(pc)
	}
	for k, v := range rd {
		if k != "path" && k != "name" {
			c.D[k] = v
		}
	}
	return db.Add(c.D)
}
