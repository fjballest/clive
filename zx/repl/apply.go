package repl

import (
	"clive/zx"
	"clive/cmd"
	fpath "path"
	"errors"
)

// Apply a series of changes from local/remote/both replicas
// to the other and update the dbs accordingly.
// If a change has errors noted in it, it's ignored.
func (t *Tree) ApplyAll(cc <-chan Chg, from Where, appliedc chan<- Chg) error {
	var err error
	for c := range cc {
		// t.Ldb.Dprintf("apply %s\n", c)
		if from == Both || c.At == from {
			err2 := t.Apply(c)
			if err2 != nil {
				t.Ldb.Dprintf("apply err %s\n", err2)
			}
			if err == nil {
				err = err2
			}
			if appliedc != nil {
				c.D = c.D.Dup()
				if c.D["err"] == "" && err2 != nil {
					c.D["err"] = err2.Error()
				}
				appliedc <- c
			}
		}
	}
	if err == nil {
		err = cerror(cc)
	}
	close(appliedc, err)
	return err
}

// Apply a single change and update the dbs accordingly.
// If the change has errors noted in it, it's ignored.
func (t *Tree) Apply(c Chg) error {
	if c.D["err"] != "" {
		return nil
	}
	ldb, rdb := t.Ldb, t.Rdb
	defer func(ldb, rdb *DB) {
		t.Ldb, t.Rdb = ldb, rdb
	}(ldb, rdb)
	if c.At == Local {
		ldb, rdb = rdb, ldb
	}
	if isExcl(c.D["path"], ldb.Excl...) || isExcl(c.D["path"], rdb.Excl...) {
		return nil
	}
	// ldb is the target and ldb is the source

	switch c.Type {
	case zx.None:
		return nil
	case zx.Meta:
		return ldb.applyMeta(c, rdb)
	case zx.Data:
		return ldb.applyData(c, rdb)
	case zx.Add:
		return ldb.applyAdd(c, rdb)
	case zx.Del:
		return ldb.applyDel(c, rdb)
	case zx.DirFile:
		// get rid of the old and add the new
		nc := c
		nc.Type = zx.Del
		err := ldb.applyDel(nc, rdb)
		if err2 := ldb.applyAdd(c, rdb); err == nil {
			err = err2;
		}
		return err
	default:
		panic("unknown change")
	}
}

func (db *DB) applyMeta(c Chg, rdb *DB) error {
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
	err := db.Add(rd)
	if err == nil {
		rdb.Add(rd)
	}
	return err
}

func (db *DB) applyDel(c Chg, rdb *DB) error {
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
	c.D["rm"] = "y"
	err = db.Add(c.D)
	if zx.IsNotExist(err) {
		err = nil
	}
	if err == nil {
		rdb.Add(c.D)
	}
	return err
}

struct pfile {
	fs zx.Putter
	d zx.Dir
	dc chan<- []byte
	rc <-chan zx.Dir
	ldb, rdb *DB
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

func (pf *pfile) add() error {
	if pf.d == nil {
		return nil
	}
	if isExcl(pf.d["path"], pf.ldb.Excl...) {
		return nil
	}
	if err := pf.ldb.Add(pf.d); err == nil {
		pf.rdb.Add(pf.d)
	}
	pf.d = nil
	return nil
}

func (pf *pfile) done() error {
	if pf.dc == nil {
		return pf.add()
	}
	close(pf.dc)
	rd := <-pf.rc
	err := cerror(pf.rc)
	pf.dc = nil
	pf.rc = nil
	for k, v := range rd {
		if k != "path" && k != "name" {
			pf.d[k] = v
		}
	}
	if err != nil && pf.d["err"] == "" {
		pf.d["err"] = err.Error()
	}
	return pf.add()
}

func (db *DB) applyAdd(c Chg, rdb *DB) error {
	// NB: We won't remove local files/dir before trying to add a dir/file.
	// If this happens, the apply will fail and the user can always remove
	// the file/dir causing the error and then retry
	fs := rdb.Fs
	rpath := rdb.rpath
	gfs, ok := fs.(zx.FindGetter)
	if !ok {
		return errors.New("fs can't findget")
	}
	pfs, ok := db.Fs.(zx.Putter)
	if !ok {
		return errors.New("fs can't put")
	}
	fc := gfs.FindGet(fpath.Join(rpath, c.D["path"]), "", rpath, "/", 0)
	pf := &pfile{ldb: db, rdb: rdb}
	for m := range fc {
		switch d := m.(type) {
		case zx.Dir:
			d = d.Dup()
			db.Dprintf("add %s\n", d.Fmt())
			if err := pf.done(); err != nil {
				cmd.Warn("add %s: %s", pf.d["path"], err)
			}
			pf.start(pfs, db.rpath, d)
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
				pf.d["err"] = err.Error()
				pf.done()
			}
		}
	}
	err := pf.done()
	if err != nil {
		cmd.Warn("add %s: %s", pf.d["path"], err)
	}
	return cerror(fc)
}

func (db *DB) applyData(c Chg, rdb *DB) error {
	fs := rdb.Fs
	rpath := rdb.rpath
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
	err := db.Add(c.D)
	if err == nil {
		rdb.Add(c.D)
	}
	return err
}
