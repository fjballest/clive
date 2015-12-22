package cfs

import (
	"clive/dbg"
	"clive/zx"
	"clive/zx/pred"
	"errors"
	"fmt"
	"path"
	"time"
)

// see cache/cache.go for chaching handling and cfile.go for locking rules

// walk in the cache to the given rid and return its cFile r/wlocked unless there's an error.
// does not update anything.
func (fs *Cfs) cwalk(rid string, isrlock bool) (*cFile, error) {
	fs.lockc(rid, isrlock)
	f, err := fs.cfile(rid)
	if err != nil {
		fs.unlockc(rid, isrlock)
	}
	return f, err
}

// Walk to rid, and return it r/wlocked (unless there's an error) with at least its meta updated.
func (fs *Cfs) walk(rid string, isrlock bool) (f *cFile, left []string, e error) {
	fs.dprintf("walk %s...\n", rid)
	els := zx.Elems(rid)
	left = els
	defer func() {
		if e != nil {
			fs.dprintf("walkerr %s left %v\n", rid, left)
		} else {
			fs.dprintf("walked %s left %v\n", rid, left)
		}
		if e == nil && len(left) > 0 {
			panic("cfs walk: left and no error")
		}
	}()
	path := "/"
	for {
		isr := len(els) > 0 || isrlock
		nf, err := fs.cwalk(path, isr)
		if err != nil {
			return f, left, err
		}
		left = els
		f = nf
		if len(els) > 0 {
			path = zx.Path(path, els[0])
			els = els[1:]
		}
		if len(left) == 0 {
			if err := f.needMeta(isr); err != nil {
				// lock has been released
				if dbg.IsNotExist(err) {
					// discovered in needMeta that the file is gone
					// we raced, so try again
					f, left, err = fs.walk(rid, isrlock)
				}
				return f, left, err
			}
			return f, left, nil
		}
		if err := f.needData(isr, nil); err != nil {
			// lock has been released
			if dbg.IsNotExist(err) {
				// discovered in needMeta that the file is gone
				// we raced, so try again
				f, left, err = fs.walk(rid, isrlock)
			}
			return f, left, err
		}
		if !fs.NoPermCheck && !f.d.CanWalk(fs.ai) {
			f.unlockc(isr)
			return f, left, fmt.Errorf("%s: %s", f, dbg.ErrPerm)
		}
		f.unlockc(isr)
	}
}

func (fs *Cfs) stat(rid string) (zx.Dir, error) {
	rid, err := zx.AbsPath(rid)
	if err != nil {
		return nil, err
	}
	if rid == "/Ctl" {
		d := ctldir.Dup()
		d["tpath"] = fs.tpath
		return d, nil
	}
	if rid == "/Chg" {
		d := chgdir.Dup()
		d["tpath"] = fs.tpath
		return d, nil
	}
	f, _, err := fs.walk(rid, RO)
	if err != nil {
		return nil, err
	}
	f.unlockc(RO)

	nocattrs(f.d)

	/* do not add /Chg to the count, it's hidden now
	if id == "/" {	// adjust size for /Chg
		nd["size"] = strconv.Itoa(f.d.Int("size") + 1)
	}
	*/
	return f.d, nil
}

func (fs *Cfs) Stat(rid string) chan zx.Dir {
	fs.dprintf("stat %s\n", rid)
	cs := fs.IOstats.NewCall(zx.Sstat)
	dc := make(chan zx.Dir, 1)
	go func() {
		defer fs.lktrz.NoLocks()
		d, err := fs.stat(rid)
		if err != nil {
			cs.End(true)
			fs.dprintf("stat %s: %s\n", rid, err)
			close(dc, err)
			return
		}
		dc <- d
		cs.End(false)
		fs.dprintf("stat %s: %s\n", rid, d)
		close(dc)
	}()
	return dc
}

func addctl(ds []zx.Dir) []zx.Dir {
	// /Chg is not added the list, so things like tar do not read it by accident.
	for i := 0; i < len(ds); i++ {
		if ds[i]["name"] == "Ctl" {
			ds[i] = ctldir.Dup()
			return ds
		}
	}
	ds = append(ds, ctldir.Dup())
	zx.SortDirs(ds)
	return ds
}

func (fs *Cfs) get(rid string, off, count int64, pred string, datc chan<- []byte, cs *zx.CallStat) (int, error) {
	rid, err := zx.AbsPath(rid)
	if err != nil {
		return 0, err
	}
	if rid == "/Ctl" {
		return fs.getCtl(off, count, pred, datc, cs)
	}
	if rid == "/Chg" {
		return fs.getChg(off, count, pred, datc, cs)
	}

	f, _, err := fs.walk(rid, RO)
	if err != nil {
		return 0, err
	}
	if !fs.NoPermCheck && !f.d.CanRead(fs.ai) {
		f.unlockc(RO)
		return 0, fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
	}
	if err := f.needData(RO, nil); err != nil {
		// already unlocked
		return 0, err
	}
	cs.Sending()
	if f.d["type"] == "d" {
		ds, err := zx.GetDir(fs.lfs, f.d["path"])
		f.unlockc(RO)
		if err != nil {
			return 0, err
		}
		tot := 0
		nm := 0
		if f.d["path"] == "/" { // adjust for /Ctl (/Chg is not added)
			ds = addctl(ds)
		}
		for _, d := range ds {
			if len(d) == 0 {
				break
			}
			nocattrs(d)
			d["proto"] = "proc"
			d["tpath"] = fs.tpath
			d["spath"] = d["path"]
			delete(d, "addr")
			n, err := d.Send(datc)
			if err != nil {
				cs.Sends(int64(nm), int64(n))
				return tot, err
			}
			tot += n
			nm++
		}
		cs.Sends(int64(nm), int64(tot))
		return tot, nil
	}
	defer f.unlockc(RO) // could release the lock to accept further changes during get
	ldatc := fs.lfs.Get(rid, off, count, pred)

	nm := 0
	tot := 0
	for dat := range ldatc {
		if ok := datc <- dat; !ok {
			close(ldatc, "done")
		}
		nm++
		tot += len(dat)
	}
	cs.Sends(int64(nm), int64(tot))
	err = cerror(ldatc)
	cs.End(err != nil)
	return tot, err
}

func (fs *Cfs) Get(rid string, off, count int64, pred string) <-chan []byte {
	fs.dprintf("get %s %d %d %q\n", rid, off, count, pred)
	cs := fs.IOstats.NewCall(zx.Sget)
	c := make(chan []byte)
	go func() {
		defer fs.lktrz.NoLocks()
		n, err := fs.get(rid, off, count, pred, c, cs)
		cs.End(err != nil)
		if err != nil {
			fs.dprintf("get %s: [%d] %s\n", rid, n, err)
		} else {
			fs.dprintf("get %s: [%d]\n", rid, n)
		}
		close(c, err)
	}()
	return c
}

var ErrNoMatch = errors.New("false")

func (fs *Cfs) matchDir(rid string, d zx.Dir, fpred string) error {
	if fpred == "" {
		return nil
	}
	if d == nil {
		d = zx.Dir{
			"path": rid,
			"name": path.Base(rid),
			"type": "-",
		}
	}
	p, err := pred.New(fpred)
	if err != nil {
		return err
	}
	match, _, err := p.EvalAt(d, 0)
	if err != nil {
		return err
	}
	if !match {
		return ErrNoMatch
	}
	return nil
}

func (fs *Cfs) put(rid string, d zx.Dir, off int64, dc <-chan []byte, pred string) (zx.Dir, int, error) {
	rid, err := zx.AbsPath(rid)
	if err != nil {
		return nil, 0, err
	}
	if rid == "/" || rid == "/Chg" {
		return nil, 0, fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
	}
	if rid == "/Ctl" {
		err := fs.putCtl(dc)
		return zx.Dir{"size": "0"}, 0, err
	}
	if fs.rdonly {
		return nil, 0, fmt.Errorf("%s: %s", fs.Tag, dbg.ErrRO)
	}
	nocattrs(d)
	noinherit := false
	if m := d["Mode"]; m != "" {
		d["mode"] = m
		delete(d, "Mode")
		noinherit = true
	}
	// if the file exists then it's a truncate, otherwise it's a create (if d["mode"] is set)
	// either way, the parent must have the data fetched to check for permissions
	// and file existence and, if the file exists, we are going to change its status in the cache.
	f, left, err := fs.walk(rid, RW)
	if len(left) > 1 || (err != nil && !dbg.IsNotExist(err)) {
		return nil, 0, err // parent does not exist or another error.
	}
	wf := f
	var pmode uint64
	var pgid string
	if len(left) == 0 {
		prid := path.Dir(rid)
		pf, _, err := fs.walk(prid, RO)
		if err != nil {
			return nil, 0, err
		}
		fs.unlockc(prid, RO)
		pmode = pf.d.Mode()
		pgid = pf.d["Gid"]
	}
	if len(left) == 1 {
		if d["mode"] == "" {
			return nil, 0, fmt.Errorf("%s: %s", rid, dbg.ErrNotExist)
		}
		pmode = d.Mode()
		pgid = d["Gid"]
		fs.lockc(rid, RW)
		od, err := zx.Stat(f.t.lfs, rid)
		if err == nil {
			// just created, assume we walked.
			f = &cFile{d: od, t: fs}
			left = left[1:]
		} else if !dbg.IsNotExist(err) {
			// just created, but failed.
			fs.unlockc(rid, RW)
			return nil, 0, err
		} else {
			// brand new
			u := dbg.Usr
			if fs.ai != nil {
				u = fs.ai.Uid
			}
			od = zx.Dir{
				"name":  left[0],
				"type":  "-",
				"mode":  d["mode"],
				"path":  rid,
				"spath": rid,
				"tpath": fs.tpath,
				"proto": "proc",
				"Uid":   u,
				"Gid":   pgid,
				"Wuid":  u,
			}
			if err := fs.matchDir(rid, od, pred); err != nil {
				fs.unlockc(rid, RW)
				return nil, 0, err
			}
			f = &cFile{d: od, t: fs}
		}
	} else if err != nil {
		panic("cfs put bug err is " + err.Error())
	} else if err := fs.matchDir(rid, f.d, pred); err != nil {
		fs.unlockc(rid, RW)
		return nil, 0, err
	}
	// else f is the file to be written and it's w locked.
	if f.d["path"] != rid {
		panic("cfs put rid bug")
	}

	if f.d["type"] == "d" || d["type"] == "d" {
		fs.unlockc(rid, RW)
		return nil, 0, fmt.Errorf("%s: %s", rid, dbg.ErrIsDir)
	}

	// must be able to write the file (if it's there) or the parent (if it was not)
	if !fs.NoPermCheck && !wf.d.CanWrite(fs.ai) {
		fs.unlockc(rid, RW)
		return nil, 0, fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
	}
	ai := fs.ai
	if fs.NoPermCheck {
		ai = nil
	}
	if !fs.WstatAll || fs.ai != nil {
		if !noinherit && d["mode"] != "" {
			f.d.Inherit(pmode)
			d["mode"] = f.d["mode"]
		}
		if err := f.d.CanWstat(ai, d); err != nil {
			fs.unlockc(rid, RW)
			return nil, 0, err
		}
	}
	// If it's not a full rewrite and the file is unread, we need the data.
	if d["mode"] != "" && len(left) == 0 {
		if err := f.needData(RW, nil); err != nil {
			// already unlocked
			return nil, 0, err
		}
	}

	// now we can put the data
	// flag the file as data & busy, and clear busy once we are done.
	if len(left) == 1 {
		fs.cache.CreatedBusy(fs.cfs, fs.rfs, f.d)
	} else {
		fs.cache.DirtyDataBusy(fs.cfs, fs.rfs, f.d)
	}

	f.setRtime()
	addattrs(f.d, d, true)
	if d == nil {
		d = zx.Dir{}
	}
	d["Rtime"] = f.d["Rtime"]
	if c, ok := f.d["Cache"]; ok {
		d["Cache"] = c
	}
	fs.unlockc(rid, RW)

	// Don't put the entire f.d: d["mode"] != "" means creation and
	// d["size"] means truncation, but this put might be a plain pwrite().
	if d["mode"] != "" && noinherit {
		d["Mode"] = d["mode"] // no inherit in lfs
	}
	n := 0
	datc := dc
	if fs.Flags.Dbg {
		// TODO: get rid of this when the bug is fixed.
		// There's a bug causing a file to be truncated at 4108 bytes
		// after two puts with 2054 bytes.
		// This is to let the debug show how many actual bytes are put
		// to see if it's a bug on the fuse part or within cfs.
		xc := make(chan []byte)
		datc = xc
		go func() {
			for x := range dc {
				n += len(x)
				if ok := xc <- x; !ok {
					close(dc, cerror(xc))
					break
				}
			}
			close(xc, cerror(dc))
		}()
	}
	rc := fs.lfs.Put(rid, d, off, datc, pred)
	rd := <-rc
	f.lockc(RW)
	// because we released the lock, we must stat the file again.
	fs.cache.NotBusy(f.d)
	nf, err := f.t.cfile(f.d["path"])
	f.unlockc(RW)
	if err == nil {
		err = cerror(rc)
	}
	if err != nil {
		return nil, n, err
	}
	f.d = nf.d
	f.changed()
	for k := range rd {
		rd[k] = nf.d[k]
	}
	return rd, n, nil
}

func (fs *Cfs) Put(rid string, d zx.Dir, off int64, dc <-chan []byte, pred string) chan zx.Dir {
	d = d.Dup()
	fs.dprintf("put %s %s %d '%s'\n", rid, d, off, pred)
	cs := fs.IOstats.NewCall(zx.Sput)
	c := make(chan zx.Dir, 1)
	go func() {
		defer fs.lktrz.NoLocks()
		d, n, err := fs.put(rid, d, off, dc, pred)
		cs.End(err != nil)
		if err != nil {
			fs.dprintf("put %s: %s\n", rid, err)
		} else {
			fs.dprintf("put %s: %d at %d dir %s\n", rid, n, off, d)
			c <- d
		}
		close(c, err)
	}()
	return c
}

func (fs *Cfs) mkdir(rid string, d zx.Dir) error {
	rid, err := zx.AbsPath(rid)
	if err != nil {
		return err
	}
	if fs.rdonly {
		return fmt.Errorf("%s: %s", fs.Tag, dbg.ErrRO)
	}
	if rid == "/" || rid == "/Ctl" || rid == "/Chg" {
		return fmt.Errorf("%s: %s", rid, dbg.ErrExists)
	}

	wf, left, err := fs.walk(rid, RO)
	if err == nil {
		wf.unlockc(RO)
		return fmt.Errorf("%s: %s", rid, dbg.ErrExists)
	}
	if len(left) > 1 || !dbg.IsNotExist(err) {
		return err
	}
	fs.lockc(rid, RW)
	defer fs.unlockc(rid, RW)
	if _, err := zx.Stat(fs.lfs, rid); err == nil {
		return fmt.Errorf("%s: %s", rid, dbg.ErrExists)
	}
	d = d.Dup()
	nocattrs(d)
	if d == nil {
		d = zx.Dir{}
	}
	u := dbg.Usr
	if fs.ai != nil {
		u = fs.ai.Uid
	}
	if d["Mode"] != "" {
		d["mode"] = d["Mode"]
	}
	if d["mode"] == "" {
		d["mode"] = "0755"
	}
	nd := zx.Dir{
		"path": rid,
		"mode": d["mode"],
		"type": "d",
		"Uid":  u,
		"Gid":  wf.d["Gid"],
	}
	if !fs.NoPermCheck && !wf.d.CanWrite(fs.ai) && (!fs.WstatAll || fs.ai != nil) {
		if err := nd.CanWstat(fs.ai, d); err != nil {
			return fmt.Errorf("%s: %s", rid, err)
		}
	}
	rc := fs.lfs.Mkdir(rid, d)
	err = <-rc
	if err != nil {
		return err
	}
	f, err := fs.cfile(rid) // must stat to notify clients in changed
	if err != nil {
		return err
	}
	fs.cache.Created(fs.cfs, fs.rfs, f.d)
	f.changed()
	return nil

}

func (fs *Cfs) Mkdir(rid string, d zx.Dir) chan error {
	fs.dprintf("mkdir %s %s \n", rid, d)
	cs := fs.IOstats.NewCall(zx.Smkdir)
	c := make(chan error, 1)
	go func() {
		defer fs.lktrz.NoLocks()
		err := fs.mkdir(rid, d)
		cs.End(err != nil)
		if err != nil {
			fs.dprintf("mkdir %s: %s\n", rid, err)
		} else {
			fs.dprintf("mkdir %s: ok\n", rid)
		}
		c <- err
		close(c, err)
	}()
	return c
}

func (fs *Cfs) wstat(rid string, d zx.Dir) error {
	if fs.rdonly {
		return fmt.Errorf("%s: %s", fs.Tag, dbg.ErrRO)
	}
	rid, err := zx.AbsPath(rid)
	if err != nil || rid == "/Ctl" { // make wstat(/ctl) a nop
		return err
	}
	if rid == "/Chg" {
		return fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
	}
	nocattrs(d)
	f, _, err := fs.walk(rid, RW)
	if err != nil {
		return err
	}
	defer f.unlockc(RW)
	ai := fs.ai
	if fs.NoPermCheck {
		ai = nil
	}
	if !fs.WstatAll || fs.ai != nil {
		if err := f.d.CanWstat(ai, d); err != nil {
			return err
		}
	}
	if d == nil {
		return nil
	}
	if d["size"] != "" {
		if err := f.needData(RW, nil); err != nil {
			return err
		}
	}
	addattrs(f.d, d, true)
	fs.setRtime(f.d)
	rc := fs.lfs.Wstat(rid, d)
	err = <-rc
	if err != nil {
		return err
	}
	if d["size"] != "" {
		fs.cache.DirtyData(fs.cfs, fs.rfs, f.d)
	} else {
		fs.cache.DirtyMeta(fs.cfs, fs.rfs, f.d)
	}
	nf, err := f.t.cfile(f.d["path"]) // for changed
	if err == nil {
		f.d = nf.d
		f.changed()
	}
	return nil
}

func (fs *Cfs) Wstat(rid string, d zx.Dir) chan error {
	fs.dprintf("wstat %s %v\n", rid, d)
	c := make(chan error, 1)
	cs := fs.IOstats.NewCall(zx.Swstat)
	go func() {
		defer fs.lktrz.NoLocks()
		err := fs.wstat(rid, d)
		cs.End(err != nil)
		if err != nil {
			fs.dprintf("wstat %s: %s\n", rid, err)
		} else {
			fs.dprintf("wstat %s: ok\n", rid)
		}
		c <- err
		close(c, err)
	}()
	return c
}

func inconsistent(from, to string) bool {
	if from == to {
		return false
	}
	// moves from inside to?
	// i.e. is from a prefix of to
	return zx.HasPrefix(to, from)
}

func (fs *Cfs) move(from, to string) error {
	if fs.rdonly {
		return fmt.Errorf("%s: %s", fs.Tag, dbg.ErrRO)
	}
	from, err := zx.AbsPath(from)
	if err != nil {
		return err
	}
	to, err = zx.AbsPath(to)
	if err != nil {
		return err
	}
	if from == to {
		return nil
	}
	if from == "/Ctl" || from == "/Chg" || from == "/" {
		return fmt.Errorf("%s: %s", from, dbg.ErrPerm)
	}
	if to == "/Ctl" || to == "/Chg" || to == "/" {
		return fmt.Errorf("%s: %s", to, dbg.ErrPerm)
	}
	if inconsistent(from, to) {
		return fmt.Errorf("%s: inconsistent move", from)
	}
	pfrom := path.Dir(from)
	pto := path.Dir(to)
	if !fs.NoPermCheck {
		pf, _, err := fs.walk(pfrom, RO)
		if err != nil {
			return err
		}
		pf.unlockc(RO)
		if !pf.d.CanWrite(fs.ai) {
			return fmt.Errorf("%s: %s", pf, dbg.ErrPerm)
		}
	}
	if !fs.NoPermCheck && pto != pfrom {
		pf, _, err := fs.walk(pto, RO)
		if err != nil {
			return err
		}
		pf.unlockc(RO)
		if !pf.d.CanWrite(fs.ai) {
			return fmt.Errorf("%s: %s", pf, dbg.ErrPerm)
		}
	}

	// Make sure we have in the cache the relevant files
	fromf, _, err := fs.walk(from, RO)
	if err != nil {
		return err
	}
	fromf.unlockc(RO)

	tof, toleft, err := fs.walk(to, RO)
	if err != nil && !dbg.IsNotExist(err) || len(toleft) > 1 {
		// can't have tof.dirty
		return err
	}
	if err == nil {
		tof.unlockc(RO)
		if tof.d["type"] == "d" {
			return fmt.Errorf("%s: %s", tof, dbg.ErrExists)
		}
		if fromf.d["type"] == "d" {
			return fmt.Errorf("%s: destination is a directory", tof)
		}
	}

	fs.cache.Sync(func() {
		err = <-fs.lfs.Move(from, to)
		if err != nil {
			// couldnt move
			return
		}
		if err == nil {
			err = <-fs.rfs.Move(from, to)
			if err != nil {
				// try to undo
				if nerr := <-fs.lfs.Move(to, from); nerr == nil {
					return
				}
			}
		}
		// did move or did and failed and couldn't undo.
		// invalidate the source and target dirs so clients re-read them.
		dfrom, err := zx.Stat(fs.lfs, pfrom)
		if err == nil {
			pf := &cFile{d: dfrom, t: fs}
			pf.changed()
		}
		if pto != pfrom {
			dto, err := zx.Stat(fs.lfs, pto)
			if err == nil {
				pf := &cFile{d: dto, t: fs}
				pf.changed()
			}
		}
	})

	return err
}

func (fs *Cfs) Move(from, to string) chan error {
	c := make(chan error, 1)
	fs.dprintf("move %s %s \n", from, to)
	cs := fs.IOstats.NewCall(zx.Smove)
	go func() {
		defer fs.lktrz.NoLocks()
		err := fs.move(from, to)
		cs.End(err != nil)
		if err != nil {
			fs.dprintf("move %s: %s\n", from, err)
		} else {
			fs.dprintf("move %s: ok\n", from)
		}
		c <- err
		close(c, err)
	}()
	return c
}

func (fs *Cfs) remove(rid string, all bool) error {
	if fs.rdonly {
		return fmt.Errorf("%s: %s", fs.Tag, dbg.ErrRO)
	}
	rid, err := zx.AbsPath(rid)
	if rid == "/" || rid == "/Ctl" || rid == "/Chg" {
		return fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
	}
	f, _, err := fs.walk(rid, RW)
	if err != nil {
		return err
	}
	defer f.unlockc(RW)
	if all {
		err = <-fs.lfs.RemoveAll(rid)
	} else {
		err = <-fs.lfs.Remove(rid)
	}
	if err != nil {
		return err
	}
	f.d.SetTime("mtime", time.Now())
	f.dirty = false
	f.t.cache.Removed(fs.cfs, fs.rfs, f.d)
	f.changed()
	return err

}

func (fs *Cfs) Remove(rid string) chan error {
	c := make(chan error, 1)
	fs.dprintf("remove %s\n", rid)
	cs := fs.IOstats.NewCall(zx.Sremove)
	go func() {
		defer fs.lktrz.NoLocks()
		err := fs.remove(rid, false)
		cs.End(err != nil)
		if err != nil {
			fs.dprintf("remove %s: %s\n", rid, err)
		} else {
			fs.dprintf("remove %s: ok\n", rid)
		}
		c <- err
		close(c, err)
	}()
	return c
}

func (fs *Cfs) RemoveAll(rid string) chan error {
	fs.dprintf("removeall %s\n", rid)
	c := make(chan error, 1)
	cs := fs.IOstats.NewCall(zx.Sremoveall)
	go func() {
		defer fs.lktrz.NoLocks()
		err := fs.remove(rid, true)
		cs.End(err != nil)
		if err != nil {
			fs.dprintf("removeall %s: %s\n", rid, err)
		} else {
			fs.dprintf("removeall %s: ok\n", rid)
		}
		c <- err
		close(c, err)
	}()
	return c
}

func fixpath(d zx.Dir, spref, dpref string) {
	if spref != dpref {
		cpath := d["path"]
		suff := zx.Suffix(cpath, spref)
		d["path"] = zx.Path(dpref, suff)
	}
}

// f is r locked and gets unlocked by find
func (f cFile) find(p *pred.Pred, spref, dpref string, lvl int, c chan<- zx.Dir) {
	if f.d["type"] == "d" {
		if err := f.needData(RO, nil); err != nil {
			f.d["err"] = err.Error()
			fixpath(f.d, spref, dpref)
			nocattrs(f.d)
			c <- f.d
			return
		}
	} else {
		if err := f.needMeta(RO); err != nil {
			f.d["err"] = err.Error()
			fixpath(f.d, spref, dpref)
			nocattrs(f.d)
			c <- f.d
			return
		}
	}
	f.unlockc(RO)
	if f.d["rm"] != "" {
		return
	}

	match, pruned, err := p.EvalAt(f.d, lvl)
	f.dprintf("find at %s:\n\t%v\n\t%v %v %v\n\n", f.d, p, match, pruned, err)
	if pruned {
		if !match {
			f.d["err"] = "pruned"
		}
		fixpath(f.d, spref, dpref)
		nocattrs(f.d)
		c <- f.d
		return
	}
	if err != nil {
		f.d["err"] = err.Error()
		fixpath(f.d, spref, dpref)
		nocattrs(f.d)
		c <- f.d
		return
	}
	if match {
		nd := f.d.Dup()
		fixpath(nd, spref, dpref)
		nocattrs(nd)
		if ok := c <- nd; !ok {
			return
		}
	}
	if f.d["type"] != "d" {
		return
	}
	f.lockc(RO)
	cds, err := zx.GetDir(f.t.lfs, f.d["path"])
	f.unlockc(RO)
	for _, cd := range cds {
		if cd["rm"] != "" {
			continue
		}
		cf, err := f.t.cwalk(cd["path"], RO)
		if dbg.IsNotExist(err) {
			continue
		} else if err != nil {
			cd["err"] = err.Error()
			fixpath(cd, spref, dpref)
			nocattrs(cd)
			c <- cd
			continue
		}
		// fmt.Printf("child %s\n", cd)
		cf.find(p, spref, dpref, lvl+1, c)
	}
}

// NB: We never find /Chg; feature, not bug
func (fs *Cfs) find(rid, fpred, spref, dpref string, depth int, c chan<- zx.Dir, cs *zx.CallStat) error {
	rid, err := zx.AbsPath(rid)
	if err != nil {
		return err
	}
	p, err := pred.New(fpred)
	if err != nil {
		return err
	}
	f, _, err := fs.walk(rid, RO)
	if err != nil {
		return err
	}
	cs.Sending()
	f.find(p, spref, dpref, depth, c)
	return nil
}

func (fs *Cfs) Find(rid, fpred, spref, dpref string, depth int) <-chan zx.Dir {
	fs.dprintf("find %s '%s' '%s' '%s' %d\n", rid, fpred, spref, dpref, depth)
	c := make(chan zx.Dir, 1)
	cs := fs.IOstats.NewCall(zx.Sfind)
	go func() {
		defer fs.lktrz.NoLocks()
		err := fs.find(rid, fpred, spref, dpref, depth, c, cs)
		cs.End(err != nil)
		if err != nil {
			fs.dprintf("find %s: %s\n", rid, err)
		} else {
			fs.dprintf("find %s: ok\n", rid)
		}
		close(c, err)
	}()
	return c
}

func (fs *Cfs) FindGet(rid, fpred, spref, dpref string, depth int) <-chan zx.DirData {
	fs.dprintf("findget %s '%s' '%s' '%s' %d\n", rid, fpred, spref, dpref, depth)
	gc := make(chan zx.DirData, 1)
	cs := fs.IOstats.NewCall(zx.Sfindget)
	go func() {
		defer fs.lktrz.NoLocks()
		dc := fs.Find(rid, fpred, spref, dpref, depth) // BUG: will stat a Sfind
		for d := range dc {
			g := zx.DirData{Dir: d}
			var datac chan []byte
			if d["err"] == "" && d["type"] != "d" {
				datac = make(chan []byte)
				g.Datac = datac
			}
			if ok := gc <- g; !ok {
				close(dc, cerror(gc))
				break
			}
			if datac != nil {
				_, err := fs.get(d["spath"], 0, zx.All, "", datac, nil)
				close(datac, err)
			}
		}
		err := cerror(dc)
		cs.End(err != nil)
		if err != nil {
			fs.dprintf("find %s: %s\n", rid, err)
		} else {
			fs.dprintf("find %s: ok\n", rid)
		}
		cs.End(err != nil)
		close(gc, err)
	}()
	return gc
}
