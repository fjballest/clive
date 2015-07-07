package cfs

import (
	"clive/work"
	"clive/zx"
	"crypto/sha1"
	"fmt"
	"time"
	"clive/zx/cfs/cache"
	"clive/dbg"
	"path"
)

/*
	Locking:

	The cache fs (cfs.lfs) is responsible for most locking, as operations
	are made directly to it and it should handle concurrency on its own.
	However, the usage of the cache requires extra locking so we do not
	get races while pulling or pushing changes.
	Cfs.locks are used to RW lock cache state entries using the data locks for the paths.

	For updating the cache we use cfs.cfs instead of cfs.lfs to be able to write
	uids including wuid (cfs.cfs has no ai).

	cFile structures used in most of the code are temporary in-memory
	dirs for the files and are discarded when operations complete.

	see cache/cache.go for details on cache handling.
*/


// a file as far as cfs is concerned, taken from lfs.
type cFile {
	d zx.Dir
	t *Cfs
	dirty bool	// d must be updated in lfs
}

var zsum string

func init() {
	h := sha1.New()
	sum := h.Sum(nil)
	zsum = fmt.Sprintf("%040x", sum)
}

func nocattrs(d zx.Dir) {
	if d != nil {
		delete(d, "Rtime")
		delete(d, "Cache")
	}
}

// Add attrs to a file dir, so that we can do a single wstat later
func addattrs(fd, attrs zx.Dir, sizeok bool) {
	if attrs == nil {
		return
	}
	if !sizeok {
		delete(attrs, "size")
	}
	for k, v := range attrs {
		if zx.IsUsr(k) {
			fd[k] = v
		}
	}
}

// Locks for the cache.
// They lock the cache info for R or W it, not the file.
// File locking is handled by lfs on its own.

func (fs *Cfs) lockc(rid string, isrlock bool) {
	fs.lktrz.Locking(rid, 1)
	fs._lockc(rid, isrlock)
}

func (fs *Cfs) _lockc(rid string, isrlock bool) {
	if isrlock {
		fs.cache.RLockData(rid)	// locks cache info; not file data
	} else {
		fs.cache.LockData(rid)	// locks cache info; not file data
	}
}

func (fs *Cfs) unlockc(rid string, isrlock bool) {
	fs.lktrz.Unlocking(rid)
	fs._unlockc(rid, isrlock)
}

func (fs *Cfs) _unlockc(rid string, isrlock bool) {
	if isrlock {
		fs.cache.RUnlockData(rid)	// unlocks cache info; not file data
	} else {
		fs.cache.UnlockData(rid)	// unlocks cache info; not file data
	}
}

func (f *cFile) lockc(isrlock bool) {
	if f != nil {
		p := f.d["path"]
		f.t.lktrz.Locking(p, 1)
		f.t._lockc(p, isrlock)
	}
}

func (f *cFile) unlockc(isrlock bool) {
	if f != nil {
		p := f.d["path"]
		f.t.lktrz.Unlocking(p)
		f.t._unlockc(p, isrlock)
	}
}

func (f *cFile) cstate() cache.State {
	if f == nil || f.d == nil {
		return cache.CClean
	}
	return f.t.cache.State(f.d)
}

// return a cfile for the given rid
// locking fully handled by the caller
func (fs *Cfs) cfile(rid string) (*cFile, error) {
	cd, err := zx.Stat(fs.cfs, rid)
	if err != nil {
		return &cFile{d: zx.Dir{"path": rid}, t: fs}, err
	}
	cd["proto"] = "proc"
	cd["tpath"] = fs.tpath
	cd["spath"] = cd["path"]
	delete(cd, "addr")
	return &cFile{d: cd, t: fs}, nil
}

// only a file unread or clean can have stale meta
func (f *cFile) staleMeta() bool {
	if f.d["path"] == "/Ctl" || f.d["path"] == "/Chg" {
		return false
	}
	
	if st := f.cstate(); st != cache.CUnread && st != cache.CClean {
		return false
	}
	if f.t.epoch != "" {
		return f.d["Rtime"] != f.t.epoch
	}
	now := time.Now()
	rt := f.d.Time("Rtime")
	return now.Sub(rt) > CacheTout
}

// caller must lock f
// this is for files gone from the server, not those locally removed.
func (f *cFile) gone() error {
	if f.d["path"] == "" || f.d["path"] == "/" {
		panic("gone bug: removing /")
	}
	f.dprintf("gone\n")
	err := <-f.t.cfs.RemoveAll(f.d["path"])
	if err != nil {
		f.dprintf("%s\n", err)
	}
	f.d.SetTime("mtime", time.Now())
	f.t.cache.Gone(f.d)
	f.dirty = false
	return err
}

// caller must lock f
func (f *cFile) came() error {
	f.dprintf("came %s\n", f.d)
	f.t.cache.InvalData(f.d)
	delete(f.d, "rm")
	f.setRtime()
	var err error
	if f.d["type"] == "d" {
		err = <-f.t.cfs.Mkdir(f.d["path"], f.d)
	} else {
		err = zx.PutAll(f.t.cfs, f.d["path"], f.d, []byte{})
	}
	f.dirty = false
	if err != nil {
		f.dprintf("%s\n", err)
	}
	return err
}

// f is r/wlocked by the caller
// might be released and temporarily wlocked to refresh it
// f is r/wlocked and updated upon return, unless there's an error in which case
// no locks on f are held.
func (f *cFile) needMeta(isrlock bool) error {
	for {
		if !f.staleMeta() {
			return nil
		}
		if isrlock {
			f.unlockc(RO)
			f.lockc(RW)
			// check again in case we raced with another process.
			nf, err := f.t.cfile(f.d["path"])
			if err != nil {
				f.unlockc(RW)
				return err
			}
			f.d = nf.d
			if !f.staleMeta() {
				// try again with the right lock
				f.unlockc(RW)
				f.lockc(RO)
				continue
			}
		}
		// now we have a w lock on the cache and have stale meta

		nd, err := zx.Stat(f.t.rfs, f.d["path"])
		if err != nil {
			f.dprintf("refresh meta %s: %s\n", f.d["path"], err)
			if dbg.IsNotExist(err) && f.d["path"] != "/" {
				f.gone()
			}
			f.unlockc(RW)
			return err
		}
		f.gotMeta(nd)
		if f.dirty {
			f.dprintf("refreshed meta %s\n", f.d)
			f.writeMeta()
		}
		if !isrlock {
			return nil
		}
		// try again with the right lock
		f.unlockc(RW)
		f.lockc(RO)
	}
}

func (fs *Cfs) setRtime(d zx.Dir) {
	if fs.epoch != "" {
		d["Rtime"] = fs.epoch
	} else {
		d.SetTime("Rtime", time.Now())
	}
}

func (f *cFile) setRtime() {
	f.t.setRtime(f.d)
}

// for gotMeta results: what changed
type fchg int
const (
	cNothing fchg = iota
	cMeta
	cData
)

var cnames = map[fchg]string {
	cNothing: "nothing",
	cMeta: "meta",
	cData: "data",
}
func (fc fchg) String() string {
	return cnames[fc]
}

// Update f's meta and flag data as unread if it's invalid.
// Report if nothing, or just meta, or data changed.
// caller must wlock f
// This is called after the file is known to be out of date and
// after an inval has been received for the file.
// The file cannot be with dirty data
func (f *cFile) gotMeta(nd zx.Dir) (fchg, string) {
	od := f.d
	if nd["rm"] != "" {
		if f.d["path"] == "/" {
			return cNothing, ""
		}
		f.gone()
		return cData, "gone"
	}
	if (nd["type"] == "d" || od["type"] == "d") && nd["type"] != od["type"] {
		if f.d["path"] == "/" {
			return cNothing, ""
		}
		f.gone()
		f.d = nd
		f.came()
		return cData, "came"
	}

	cmeta := false
	cdata := false
	why := ""
	// update new attributes
	for k, v := range nd {
		switch {
		case od[k] == v:
			// ignore
		case k == "Rtime" || k == "Cache":
			// ignore
		case k == "Sum":
			if od[k] == "" || nd[k] == "" {
				continue
			}
			od[k] = v
			if od["Cache"] != "unread" {
				cdata = true
			}
			// else ignore, we can't sum no data
		case k == "mtime":
			if od["type"] != "d" {
				od[k] = v
				if od["Cache"] != "unread" {	// else no data
					cdata = true
				} else {
					cmeta = true
				}
			}
		case k == "size":
			od[k] = v
			if od["Cache"] != "unread" {	// else no data
				cdata = true
			} else if od["type"] != "d" {
				cmeta = true
			}
		case zx.IsUpper(k) || k == "mode":	// includes all Uids
			od[k] = v
			cmeta = true
		}
		if why == "" && (cdata || cmeta) {
			why = k
		}
	}

	// remove deleted attributes
	for k := range od {
		if !zx.IsUpper(k) {
			continue
		}
		if _, ok := nd[k]; ok {
			continue
		}
		if k == "Wuid" || k == "Sum" || k == "Rtime" || k == "Cache" {
			continue
		}
		delete(od, k)
		cmeta = true
		if why == "" {
			why = k + " deleted"
		}
	}

	f.d = od
	rt := od["Rtime"]
	f.setRtime()
	f.dirty = f.dirty || cdata || cmeta || rt != f.d["Rtime"]
	if cdata {
		f.t.cache.InvalData(f.d)
		return cData, why
	}
	if cmeta {
		return cMeta, why
	}
	return cNothing, ""
}

// an unread file is always stale for data and a file with changed data is never stale for data.
func (f *cFile) staleData() bool {
	if f.d["path"] == "/Ctl" || f.d["path"] == "/Chg" {
		return false
	}
	switch st := f.cstate(); st {
	case cache.CData, cache.CNew:
		return false
	case cache.CUnread, cache.CUnreadMeta:
		return true
	}
	// can be "clean" or "meta" and we must check for times
	if f.t.epoch != "" {
		return f.d["Rtime"] != f.t.epoch
	}
	now := time.Now()
	rt := f.d.Time("Rtime")
	return now.Sub(rt) > CacheTout
}

func findDir(ds []zx.Dir, name string) (zx.Dir, []zx.Dir) {
	for i, d := range ds {
		if d["name"] == name {
			ds[i] = nil
			return d, ds
		}
	}
	return nil, ds
}

// f w locked by the caller
// this must only update children and report any error.
// If we want to, we might release the wlock here on f while we update
// the children and then re-acquire the lock in needData if needed.
// If chgfn is not nil, it is called if there's a change for the file.
func (f *cFile) getDirData(datc <-chan []byte, chgfn func(...zx.Dir)) error {
	nds, err := zx.RecvDirs(datc)
	if err != nil {
		return err
	}

	ods, err := zx.GetDir(f.t.cfs, f.d["path"])
	if err != nil {
		return err
	}
	for _, d := range ods {
		if d == nil || d["path"] == "/Ctl" || d["path"] == "/Chg" {
			continue
		}
		f.t.lockc(d["path"], RW)
		cf, err := f.t.cfile(d["path"])
		if err != nil {
			f.t.unlockc(d["path"], RW)
			continue
		}
		if st := cf.cstate(); st != cache.CUnread && st != cache.CClean  {
			// have changes pending, disregard
			f.t.unlockc(d["path"], RW)
			continue
		}
		nd, _ := findDir(nds, d["name"])
		if nd == nil {
			cf.gone()
			if chgfn != nil {
				chgfn(cf.d)
			}
			cf.unlockc(RW)
			continue
		}
		chg, why := cf.gotMeta(nd)
		if chg != cNothing {
			f.dprintf("refreshed child meta: %s: %s\n", why, nd)
			if chgfn != nil {
				chgfn(cf.d)
			}
			cf.writeMeta()
		}
		cf.unlockc(RW)
	}
	for _, d := range nds {
		if d == nil || d["path"] == "/Ctl" || d["path"] == "/Chg" {
			continue
		}
		od, _ := findDir(ods, d["name"])
		if od != nil {
			continue
		}
		if f.t.cache.State(d) == cache.CDel {
			// if the file was locally removed and is not yet synced, it's not a came().
			continue
		}
		f.t.lockc(d["path"], RW)
		// if the file was created while we didn't have the lock, it's not a came()
		cf := &cFile{d: d, t: f.t}
		if od, err := zx.Stat(f.t.cfs, d["path"]); err == nil {
			cf.d = od
			cf.gotMeta(d)
			cf.writeMeta()
			// don't post the inval; wait until the next time
		} else {
			cf.came()
			if chgfn != nil {
				chgfn(cf.d)
			}
		}
		cf.unlockc(RW)
	}
	return nil
}

// f is r/wlocked by the caller
// might be released and temporarily wlocked to refresh it
// f is r/wlocked and updated upon return, unless there's an error in which case
// no locks on f are held.
// If dchgfn is not nil, it is called with any Dir that changed (eg., to post invals).
func (f *cFile) needData(isrlock bool, chgfn func(...zx.Dir)) error {
	for {
		if !f.staleData() {
			return nil
		}
		if isrlock {
			f.unlockc(RO)
			f.lockc(RW)
			// check again in case we raced with another process.
			nf, err := f.t.cfile(f.d["path"])
			if err != nil {
				f.unlockc(RW)
				return err
			}
			f.d = nf.d
			if !f.staleData() {
				// try again with the right lock
				f.unlockc(RW)
				f.lockc(RO)
				continue
			}
		}
		// now we have a w lock on the cache and have stale data
	
		nd, err := zx.Stat(f.t.rfs, f.d["path"])
		if err != nil {
			f.dprintf("refresh data %s: %s\n", f.d["path"], err)
			if dbg.IsNotExist(err) {
				if f.d["path"] != "/" {
					f.gone()
				}
			}
			f.unlockc(RW)
			return err
		}
		chg, why := f.gotMeta(nd)
		if chgfn != nil && chg != cNothing {
			chgfn(nd)
		}
		st := f.cstate()
		// f.dprintf("got meta: %s cstate: %s\n", chg, st)
		// for dirs we must read contents or we'll miss file metadata changes,
		// but that's if we are not using invalidations
		forced := f.d["type"] == "d" && f.t.epoch == ""

		if !forced && chg != cData && st != cache.CUnread &&
				st != cache.CUnreadMeta {
			// data did not change, updating meta is enough.
			if f.dirty {
				f.dprintf("refreshed meta: %s: %s\n", why, f.d)
				f.writeMeta()
			}
			if !isrlock {
				return nil
			}
			// try again with the right lock
			f.unlockc(RW)
			f.lockc(RO)
			continue
		}

		datc := f.t.rfs.Get(f.d["path"], 0, zx.All, "")
		// clean now so put also updates the Dir, rtime was set by gotmeta
		f.t.cache.Clean(f.d)
		if f.d["type"] == "d" {
			if chg != cNothing || f.dirty {
				f.dprintf("refresh data %s\n", f.d)
				f.writeMeta()
			}
			err = f.getDirData(datc, chgfn)
		} else {
			f.dprintf("refresh data %s\n", f.d)
			dc := f.t.cfs.Put(f.d["path"], f.d, 0, datc, "")
			f.dirty = false
			if d := <-dc; d != nil {
				for k, v := range d {
					f.d[k] = v
				}
			}
			err = cerror(dc)
		}
		if err != nil {
			f.t.cache.InvalData(f.d)
			f.writeMeta()
			f.unlockc(RW)
			return err
		}
		if !isrlock {
			return nil
		}
		// try again with the right lock
		f.unlockc(RW)
		f.lockc(RO)
	}
}

func (f *cFile) writeMeta() error {
	f.dirty = false
	err := <-f.t.cfs.Wstat(f.d["path"], f.d)
	if dbg.IsPerm(err) {
		dbg.Warn("wstat: cache missing NoPermCheck|WstatAll?: %s", err)
	}
	return err
}

// got a new inval from rfs, update our copy and post to all our clients
func (fs *Cfs) invalidate(d zx.Dir) {
	if len(d) == 0 {
		return
	}
	if d["path"] == "" {
		dbg.Warn("got inval: no path: %s", d)
		return
	}

	// If we don't have the file parent, or it's unread, we can disregard the update.
	ppath := path.Dir(d["path"])
	fs.lockc(ppath, RW)
	pf, err := fs.cfile(ppath)
	if err != nil {
		fs.unlockc(ppath, RW)
		fs.dprintf("got inval: ignored: no parent: %s\n", d)
		return
	}
	if st := pf.cstate(); st == cache.CUnread {
		fs.unlockc(ppath, RW)
		fs.dprintf("got inval: ignored: unread: %s\n", d)
		return
	}
	fs.unlockc(ppath, RW)

	fs.dprintf("got inval: %s\n", d)
	fs.lockc(d["path"], RW)
	f, err := fs.cfile(d["path"])
	if err != nil && d["rm"] == "" {
		// update files that came
		if !dbg.IsNotExist(err) {
			dbg.Warn("inval: cfile: %s", err)
		} else {
			f := &cFile{d: d, t: fs}
			if err := f.came(); err != nil {
				dbg.Warn("inval: came: %s", err)
			}
		}
		fs.unlockc(d["path"], RW)
		return
	}

	if st := f.cstate(); st != cache.CUnread && st != cache.CClean {
		fs.dprintf("got inval: %s ignored: locally changed: %v\n", f, st)
		// have local changes, wait for sync
		fs.unlockc(d["path"], RW)
		return
	}
	chg, _ := f.gotMeta(d)
	f.writeMeta()
	if chg != cNothing {
		f.changed()
	}
	fs.unlockc(d["path"], RW)
}

// Used when there's no /Chg in rfs.
// poll the given dir path for changes in rfs, by calling needData() for all dirs.
func (fs *Cfs) poll(p *work.Pool, fn string) {
	fs.lockc(fn, RW)
	f, err := fs.cfile(fn)
	if err != nil {
		fs.unlockc(fn, RW)
		dbg.Warn("poll %s: lfs: %s", fn, err)
		return
	}
	// f.dprintf("poll\n")
	if f.d["type"] != "d" || f.d["Cache"] == "unread" {
		fs.unlockc(fn, RW)
		return
	}
	// must read the dir before calling needdata, or we will
	// recur through all children and we want to stay within the cached tree.
	lds, err := zx.GetDir(fs.cfs, fn)
	if err != nil {
		dbg.Warn("poll %s: lfs: %s", fn, err)
	}
	if err := f.needData(RW, fs.gotInval); err != nil {
		dbg.Warn("poll %s: lfs: %s", fn, err)
		return
	}

	if f.d["type"] != "d" || f.d["Cache"] == "unread" {	// check if we raced
		fs.unlockc(fn, RW)
		return
	}
	fs.unlockc(fn, RW)
	if err != nil {
		dbg.Warn("poll: %s", err)
		return
	}
	rc := make(chan bool, len(lds))
	n := 0
	for i := range lds {
		d := lds[i]
		if d["type"] == "d" || d["Cache"] != "unread" {
			n++
			p.Call(rc, func() {
				fs.poll(p, d["path"])
			})
		}
	}
	for ; n > 0; n-- {
		<-rc
	}
}

// Max # of concurrent polls.
const (
	Npollers = 5
)

// runs with no fs.ai
func (fs *Cfs) pollproc() {
	fs.dprintf("pollproc started\n")
	p := work.NewPool(Npollers)
	doselect {
	case <- fs.closedc:
		fs.dprintf("pollproc terminated\n")
		p.Wait()
		close(fs.polldonec)
		break
	case <-time.After(PollIval):
		// fs.dprintf("poll\n")
		fs.poll(p, "/")
	}
}

