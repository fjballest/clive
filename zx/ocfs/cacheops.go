package ocfs

import (
	"clive/bufs"
	"clive/dbg"
	"clive/zx"
	"clive/zx/lfs"
	"crypto/sha1"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

var zsum string

func init() {
	h := sha1.New()
	sum := h.Sum(nil)
	zsum = fmt.Sprintf("%040x", sum)
}

// Drop the subtree at zd and make it a ghost
func (zd *Dir) kill(why string) {
	zd.cprintf("Kill", why)
	zd.ghost = true
	zd.mct = zt
	zd.dct = zt
	ochild := zd.child
	zd.child = nil
	zd.data = nil
	for _, c := range ochild {
		c.Lock()
		c.kill(why)
		c.Unlock()
	}
}

func (zd *Dir) invalAll() {
	zd.cprintf("Inval", "all")
	zd.mct = zt
	zd.epoch = 0
	zd.data = nil
	zd.dct = zt
	zd.mct = zt
}

func (zd *Dir) invalData() {
	zd.cprintf("Inval", "data")
	zd.data = nil
	zd.dct = zt
}

func (zd *Dir) refreshMeta() {
	now := time.Now()
	zd.mct = now
	zd.epoch = *zd.z.epoch
	if !zd.z.Cdebug {
		return
	}
	h := ""
	if zd.d["Sum"] != "" {
		h = zd.d["Sum"][:6] + "..."
	}
	zd.cprintf("Refresh", "meta %s 0%o %s %s\n", zd.d["type"], zd.mode, zd.d["size"], h)
}

func (zd *Dir) refreshData() {
	now := time.Now()
	zd.dct = now
	zd.epoch = *zd.z.epoch
	if !zd.z.Cdebug {
		return
	}
	h := ""
	if zd.d["Sum"] != "" {
		h = zd.d["Sum"][:6] + "..."
	}
	zd.cprintf("Refresh", "data %s 0%o %s %s\n", zd.d["type"], zd.mode, zd.d["size"], h)
}

func (zd *Dir) staleMeta() bool {
	if zd.path=="/Ctl" || zd.path=="/Chg" {
		return false
	}
	if zd.ghost {
		zd.cprintf("mstale", "ghost")
		return true
	}
	if *zd.z.epoch != 0 {
		if *zd.z.epoch!=zd.epoch || zd.mct==zt {
			zd.cprintf("mstale", "invalidated")
			return true
		}
		return false
	}
	if zd.mct == zt {
		zd.cprintf("mstale", "invalidated")
		return true
	}
	if time.Since(zd.mct) > CacheMetaTout {
		zd.cprintf("mstale", "timed out")
		return true
	}
	return false
}

// see comment in pollProc.
func (zd *Dir) poll(force bool) {
	if zd.ghost || *zd.z.epoch!=0 || zd.child==nil {
		return
	}
	zd.Lock()
	if zd.ghost || *zd.z.epoch!=0 || zd.child==nil {
		zd.Unlock()
		return
	}
	if time.Since(zd.dct)>=CachePollIval || force {
		zd.getDir()
	}
	ds := zd.children()
	zd.Unlock()
	for _, zcd := range ds {
		zcd.poll(force)
	}
}

// make sure metadata is updated
// called with zd rlocked (in which case it's runlocked, locked, unlocked, and rlocked)
// or called with rlocked set to false, in which case it does no locking.
func (zd *Dir) updmeta(rlocked bool) (bool, bool, error) {
	if !zd.staleMeta() {
		return false, false, nil
	}
	if rlocked {
		zd.RUnlock()
		zd.Lock()
	}
	zd.zprintf("Lstat")
	dc := zd.z.fs.Stat(zd.path)
	d := <-dc
	mchg, dchg, err := zd.stated("updmeta", d, cerror(dc))
	if rlocked {
		zd.Unlock()
		zd.RLock()
	}
	return mchg || err!=nil, dchg || err!=nil, err
}

func (zd *Dir) updMeta(rlocked bool) error {
	mchg, dchg, err := zd.updmeta(rlocked)
	if mchg || dchg {
		zd.z.changed(zd)
	}
	return err
}

// make sure data is updated.
// called with zd rlocked (in which case it's runlocked, locked, unlocked, and rlocked)
// or called with rlocked set to false, in which case it does no locking.
func (zd *Dir) updData(rlocked bool) error {
	mchg, dchg, err := zd.updmeta(rlocked)
	if err != nil {
		zd.z.changed(zd)
		return err
	}
	isdir := zd.d["type"] == "d"
	mustread := dchg ||
		isdir && (zd.child==nil || zd.dct==zt) ||
		!isdir && (zd.data==nil || zd.dct==zt)

	// dirs might have stale children, yet the dir might be up-to-date
	// (the dir mtime might be ok  because inodes keep metadata.
	// so we must checkout all the children.
	if isdir && !mustread {
		for _, c := range zd.child {
			c.RLock()
			if !c.ghost {
				c.updMeta(true)
			}
			c.RUnlock()
		}
	}

	if mustread {
		if zd.child==nil && zd.data==nil {
			zd.cprintf("upddata", "unread")
		} else {
			zd.cprintf("upddata", "invalidated")
		}
		inval := zd.child!=nil || zd.data!=nil
		if rlocked {
			zd.RUnlock()
			zd.Lock()
		}
		if isdir {
			err = zd.getDir()
		} else {
			err = zd.getFile()
		}
		if rlocked {
			zd.Unlock()
			zd.RLock()
		}
		if inval || err!=nil {
			zd.z.changed(zd)
			return err
		}
	}
	if mchg || dchg {
		zd.z.changed(zd)
	}
	return err
}

// Compare d with the previous info in zd, and refresh its
// metadata, perhaps invalidating the data. Returns true
// if there is any change in data or metadata.
// this does not can changed(), the caller should do that.
func (zd *Dir) stated(from string, d zx.Dir, err error) (mchanged, dchanged bool, rerr error) {
	pzd := zd.parent
	if zd.path=="/Ctl" || zd.path=="/Chg" {
		return false, false, nil
	}
	if d==nil || d["rm"]!="" {
		if zd.ghost {
			zd.d["rm"] = "y" // safety first, but it's ok
			return false, false, dbg.ErrNotExist
		}
		zd.kill(from)
		if pzd!=nil && zd.epoch==0 {
			pzd.invalData() // cause a re-read.
		}
		return true, true, err
	}
	dchanged = zd.ghost
	zd.ghost = false
	mchanged = dchanged
	if mchanged {
		zd.cprintf(zd.path, "mchanged by data (%s)\n", from)
	} else {
		for k, v := range zd.d {
			nv := d[k]
			if nv != v {
				nv := d[k]
				if len(nv) > 6 {
					nv = nv[:6] + "..."
				}
				zd.cprintf(zd.path, "mchanged %s = %s (%s)\n", k, nv, from)
				mchanged = true
				break
			}
		}
	}
	if !mchanged {
		zd.cprintf("stated", "no changes (%s)\n", from)
		return dchanged, dchanged, nil
	}

	ot, nt := zd.d["type"], d["type"]
	overs, nvers := zd.d.Int("vers"), d.Int("vers")
	if nt==ot && nt!="d" && nvers!=0 && nvers<overs {
		zd.cprintf("stated", "old update ignored v %d (%s)\n", nvers, from)
		return dchanged, dchanged, nil
	}
	switch {
	case ot!=nt && (nt=="d" || ot=="d"):
		// file became dir or dir became file
		zd.child = nil
		dchanged = true
		zd.cprintf(zd.path, "dchanged type %s (%s)\n", nt, from)
	case zd.d["mtime"] != d["mtime"]:
		zd.cprintf(zd.path, "dchanged mtime %s (%s)\n", d["mtime"], from)
		dchanged = true
	case d["Sum"]!="" && zd.d["Sum"]!=d["Sum"]:
		ns := d["Sum"]
		if len(ns) > 6 {
			ns = ns[:6] + "..."
		}
		zd.cprintf(zd.path, "dchanged Sum %s (%s)\n", ns, from)
		dchanged = true
	case nt!="d" && zd.d["size"]!=d["size"]:
		zd.cprintf(zd.path, "dchanged size %s (%s)\n", d["size"], from)
		dchanged = true
	}
	if d["Sum"] == "" {
		d["Sum"] = zd.d["Sum"]
	}
	if d["vers"] == "" {
		d["vers"] = zd.d["vers"]
	}
	zd.d = d
	zd.mode = uint(zd.d.Uint64("mode")&0777)
	zd.refreshMeta()
	if dchanged {
		zd.invalData()
	}
	return mchanged, dchanged, nil
}

// Lfs does not provide up Uid, Gid, Wuid, and Sum attributes,
// this updates those when the underlying file system is just Lfs.
func (zd *Dir) wstatAttrs(names ...string) {
	if zd.path=="/Ctl" || zd.path=="/Chg" || zd.path=="/" {
		return
	}
	wt, ok := zd.z.fs.(*lfs.Lfs)
	if !ok {
		return
	}
	xd := zx.Dir{}
	for _, n := range names {
		xd[n] = zd.d[n]
	}
	zd.zprintf("wstat", strings.Join(names, " "))
	if err := <-wt.Wstat(zd.path, xd); err != nil {
		zd.dprintf("wstat", err)
	}
}

// Read the file data from the server.
// The hash attribute of the file is updated with the sha1 for the data,
// but it's not written to the underlying file.
// Locking must be done by the caller.
func (zd *Dir) getFile() error {
	zd.zprintf("Lget", "read")
	fs := zd.z.fs
	datac := fs.Get(zd.path, 0, zx.All, "")
	if zd.data == nil {
		zd.data = &bufs.Blocks{}
	} else {
		zd.data.Truncate(0)
	}
	tot, err := zd.data.RecvFrom(datac)
	if err != nil {
		zd.invalAll()
		zd.data = nil
		return err
	}

	h := sha1.New()
	zd.data.WriteTo(h)
	osum := zd.d["Sum"]
	sum := h.Sum(nil)
	zd.d["Sum"] = fmt.Sprintf("%040x", sum)
	zd.refreshData()
	zd.zprintf("lget", "tot %d sum %20.20s\n", tot, zd.d["Sum"])
	if osum != zd.d["Sum"] {
		zd.wstatAttrs("Sum")
	}
	return nil
}

// read (or re-read) dir data and update children stats.
// The hash attribute of the file is updated with the sha1 for the data,
// but it's not written to the underlying file.
/// zd is locked by caller.
func (zd *Dir) getDir() error {
	zd.zprintf("Lget", "readdir")
	ds, err := zx.GetDir(zd.z.fs, zd.path)
	if err != nil {
		zd.dprintf("get", err)
		return err
	}
	names := []string{}
	nchild := make(map[string]*Dir, len(ds))
	firstread := zd.child == nil
	for i := 0; i < len(ds); i++ {
		cd := ds[i]
		cname := cd["name"]
		if cname=="" || cname=="." || cname==".." || cname==".#zx" {
			continue
		}
		zcd, ok := zd.child[cname]
		if ok && !zcd.ghost {
			zcd.Lock()
			// and must post an update only if zcd changes
			mchg, dchg, _ := zcd.stated("getdir", cd, nil)
			if mchg || dchg {
				zd.z.changed(zcd)
			}
			zcd.Unlock()
			delete(zd.child, cname)
		} else {
			zcd = zd.z.newDir(cd)
			zcd.parent = zd
			zcd.refreshMeta()
			if !firstread {
				zcd.RLock()
				zd.z.changed(zcd)
				zcd.RUnlock()
			}
		}
		if !zcd.ghost {
			nchild[cname] = zcd
			names = append(names, cname)
		}
	}
	for _, zcd := range zd.child {
		zcd.Lock()
		zcd.kill("readdir")
		zd.z.changed(zcd)
		zcd.Unlock()
	}
	zd.child = nchild

	osum := zd.d["Sum"]
	sort.Sort(sort.StringSlice(names))
	h := sha1.New()
	for _, n := range names {
		h.Write([]byte(n))
	}
	sum := h.Sum(nil)
	zd.d["Sum"] = fmt.Sprintf("%040x", sum)
	zd.refreshData()
	if osum != zd.d["Sum"] {
		zd.wstatAttrs("Sum")
	}
	return nil
}

func (zd *Dir) write(d zx.Dir, off int64, datc <-chan []byte) (int, int64, error) {
	wfs := zd.z.fs.(zx.RWTree)
	zd.Lock()
	defer zd.Unlock()
	if zd.d["type"] == "d" {
		return 0, 0, dbg.ErrIsDir
	}
	if err := zd.updData(false); err != nil {
		return 0, 0, err
	}
	for k, v := range d {
		zd.d[k] = v
	}
	if off < 0 {
		off = int64(zd.data.Len())
	}
	xdatc := make(chan []byte)
	zd.zprintf("Lput", "dir %v", d)
	xdc := wfs.Put(zd.path, d, off, xdatc, "")
	var tot int64
	var err error
	nm := 0

	for m := range datc {
		nm++
		if len(m) == 0 {
			continue
		}
		_, err = zd.data.WriteAt(m, off)
		if err != nil {
			close(datc, err)
			break
		}
		off += int64(len(m))
		tot += int64(len(m))
		if ok := xdatc <- m; !ok {
			err = cerror(xdatc)
			close(datc, err)
			break
		}
	}
	close(xdatc, err)
	xd := <-xdc
	if xd != nil {
		for k, v := range xd {
			zd.d[k] = v
		}
		if xd["vers"] == "" {
			zd.d["vers"] = strconv.Itoa(zd.d.Int("vers") + 1)
		}
	} else {
		err = cerror(xdc)
	}
	if err != nil {
		zd.data = nil
		zd.invalAll()
	} else {
		osum := zd.d["Sum"]
		h := sha1.New()
		zd.data.WriteTo(h)
		sum := h.Sum(nil)
		zd.d["Sum"] = fmt.Sprintf("%040x", sum)
		zd.refreshMeta()
		zd.refreshData()
		if osum != zd.d["Sum"] {
			zd.wstatAttrs("Sum")
		}
	}
	return nm, tot, err

}
