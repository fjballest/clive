package zxc

import (
	"bytes"
	"clive/dbg"
	"clive/mblk"
	"clive/zx"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// cache entry status.
//	->new			first time we make an entry
//	new -> clean		when we fech data
//	new -> newmeta		after wstat w/o data, sync pending
//	new -> data		after write/create, sync pending
//	newmeta -> new		after sync
//	newmeta ->data		after write, sync pending
//	clean -> meta		after wstat, sync pending
//	meta -> data		after update, sync pending
//	clean -> data		after write, sync pending
//	meta -> clean		after sync
//	data -> clean		after sync
//	* -> del			after remove, sync pending
//	* -> gone			after sync, if gone from server
//	del->			entry deleted after sync
type cStatus int

const (
	cNew     cStatus = iota // new entry, w/o data
	cNewMeta                // metadata is dirty, w/o data
	cClean                  // data & metadata are ok
	cMeta                   // metadata is dirty
	cData                   // metadata & data are dirty (or a dir was created)
	cDel                    // file was removed
	cGone                   // file is gone from server
)

var (
	cacheTout = time.Minute
	syncIval  = time.Minute
)

// operations for a zxc cached file
// All run with the file locked, but see getData
interface fsFile {
	Lock()
	Unlock()
	path() string   // file's path
	String() string // file's path
	dir() zx.Dir
	isDel() bool
	metaOk() bool // metadata is valid?
	dataOk() bool // data is valid?
	inval()
	gotMeta(d zx.Dir) error
	gotData(c <-chan []byte) error
	gotDir(cds []zx.Dir) error
	gone() // the file is gone from rfs
	walk1(el string) (fsFile, error)
	wstat(nd zx.Dir) error
	getDir() ([]zx.Dir, error)
	newFile(d zx.Dir, rfs zx.Fs) (fsFile, error)
	remove(all bool) error
	// releases the Lock before it returns
	getData(off, count int64, c chan<- []byte) error
	// releases the Lock before it returns
	putData(off int64, c <-chan []byte, umtime string) error
	sync(rfs zx.Fs) error
}

// basic cache entry for a file.
struct cFile {
	sync.Mutex
	d  zx.Dir
	wd zx.Dir // attributes to be updated
}

// cache file entry for a file
struct mFile {
	cFile
	c     *mCache
	sts   cStatus
	child map[string]*mFile
	data  *mblk.Buffer
	t     time.Time
}

var ctlfile = &mFile{cFile: cFile{d: ctldir}}

// operations for the zxc cache.
interface fsCache {
	setRoot(d zx.Dir) error
	root() fsFile
	sync(rfs zx.Fs) error
	inval()
	dump()
}

// In-memory cache including both data and metadata.
// if it's synchronous, meta never seems to be ok, so we stat the
// underlying fs all the times, and we sync right after every update operation.
struct mCache {
	dbg.Flag
	Verb  bool
	stats bool // synchronous cache
	slash *mFile
}

func (c cStatus) String() string {
	switch c {
	case cNew:
		return "new"
	case cNewMeta:
		return "newmeta"
	case cClean:
		return "clean"
	case cMeta:
		return "meta"
	case cData:
		return "data"
	case cDel:
		return "del"
	case cGone:
		return "gone"
	default:
		return fmt.Sprintf("BAD<%d>", c)
	}
}

func (cf *mFile) Dprintf(str string, args ...face{}) (n int, err error) {
	if cf == nil || cf.c == nil || !cf.c.Debug {
		return 0, nil
	}
	path := cf.d["path"]
	if path == "" {
		path = "noname"
	}
	return dbg.Printf("%s: %s", path, fmt.Sprintf(str, args...))

}

func (cf *mFile) vprintf(str string, args ...face{}) (n int, err error) {
	if cf == nil || cf.c == nil || !cf.c.Verb {
		return 0, nil
	}
	return dbg.Warn("%s", fmt.Sprintf(str, args...))

}

func (mf *mFile) stat() *cFile {
	return &mf.cFile
}

func (mf *mFile) String() string {
	return mf.d["path"]
}

func (mf *mFile) dir() zx.Dir {
	return mf.d
}

func (mf *mFile) isDel() bool {
	return mf.sts == cDel || mf.sts == cGone
}

func (mf *mFile) path() string {
	return mf.d["path"]
}

func (mf *mFile) walk1(el string) (fsFile, error) {
	c, ok := mf.child[el]
	if !ok || c.sts == cDel || c.sts == cGone {
		if ok && c.sts == cGone {
			delete(mf.child, el)
		}
		return nil, zx.ErrNotExist
	}
	return c, nil
}

func (mf *mFile) metaOk() bool {
	switch mf.sts {
	case cNewMeta, cMeta, cData, cDel, cGone:
		return true
	case cNew, cClean:
		ok := time.Since(mf.t) < cacheTout && !mf.c.stats
		if !ok {
			mf.Dprintf("meta not ok\n")
		}
		return ok
	default:
		panic("bad state")
	}
}

func (mf *mFile) dataOk() bool {
	switch mf.sts {
	case cMeta, cClean:
		if mf.c.stats && mf.d["type"] == "d" {
			return false
		}
		fallthrough // could time out data as well
	case cData, cDel, cGone:
		return true
	case cNew, cNewMeta:
		mf.Dprintf("data not ok\n")
		return false
	default:
		panic("bad state")
	}
}

func (mf *mFile) inval() {
	switch mf.sts {
	case cClean:
		mf.Dprintf("inval: cNew\n")
		mf.sts = cNew
		mf.data.Reset()
	case cMeta:
		mf.Dprintf("inval: cNewMeta\n")
		mf.sts = cNewMeta
		mf.data.Reset()
	case cNew, cNewMeta, cDel, cGone, cData:
		// as it was
	default:
		panic("bad state")
	}
}

func (mf *mFile) dirtyMeta() {
	switch mf.sts {
	case cNew:
		mf.Dprintf("dirty meta: cNewMeta\n")
		mf.sts = cNewMeta
	case cClean:
		mf.Dprintf("dirty meta: cMeta\n")
		mf.sts = cMeta
	case cData, cMeta, cNewMeta, cDel, cGone:
		// as it was
	}
}

func (mf *mFile) dirtyData() {
	if mf.sts != cData {
		mf.Dprintf("dirty data: cData\n")
	}
	mf.sts = cData
}

func (mf *mFile) gotMeta(d zx.Dir) error {
	mf.Dprintf("got meta\n")
	mf.t = time.Now()
	if d["type"] != mf.d["type"] {
		mf.gone()
		mf.sts = cNew
		mf.d = d
		if d["type"] == "d" {
			mf.child = map[string]*mFile{}
		} else {
			mf.data = &mblk.Buffer{}
		}
		return nil
	}
	if mf.sts == cGone {
		mf.sts = cNew
	}
	if mf.sts != cData && (!mf.d.Time("mtime").Equal(d.Time("mtime")) ||
		mf.d.Size() != d.Size()) {
		mf.inval()
	}
	mf.d = d.Dup()
	for k, v := range mf.wd {
		switch k {
		case "path", "addr", "type", "name":
			// ignored
		default:
			mf.d[k] = v
		}
	}
	return nil
}

func (mf *mFile) gotData(c <-chan []byte) error {
	switch mf.sts {
	case cNewMeta:
		mf.sts = cMeta
		mf.Dprintf("got data: cMeta\n")
	default:
		mf.sts = cClean
		mf.Dprintf("got data: cClean\n")
	}
	mf.data.Truncate(0)
	mf.d.SetSize(0)
	if mf.wd == nil {
		mf.wd = zx.Dir{}
	}
	mf.wd.SetSize(0)
	var tot int64
	for b := range c {
		n, err := mf.data.Write(b)
		tot += int64(n)
		if err != nil {
			close(c, err)
			mf.d.SetSize(tot)
			return err
		}
	}
	mf.d.SetSize(tot)
	if mf.wd != nil {
		delete(mf.wd, "size")
	}
	mf.Dprintf("got data: %d %d %d bytes\n", tot, mf.d.Size(), mf.data.Len())
	return cerror(c)
}

func (mf *mFile) wstat(nd zx.Dir) error {
	if mf.wd == nil {
		mf.wd = zx.Dir{}
	}
	if mf.d["type"] != "d" && nd["size"] != "" {
		if sz := nd.Size(); sz != int64(mf.data.Len()) {
			mf.dirtyData()
			mf.data.Truncate(sz)
		}
	}
	some := false
	for k, v := range nd {
		if mf.d[k] == v {
			continue
		}
		switch k {
		case "path", "addr", "type", "name":
			// ignored
		case "wuid":
			fallthrough
		case "mode", "size", "uid", "gid":
			fallthrough
		default:
			mf.d[k] = v
			mf.wd[k] = v
			some = true
		}
	}
	if some {
		mf.dirtyMeta()
	}
	return nil
}

func (mf *mFile) gotDir(cds []zx.Dir) error {
	switch mf.sts {
	case cNewMeta:
		mf.sts = cMeta
		mf.Dprintf("got dir: cMeta\n")
	default:
		mf.sts = cClean
		mf.Dprintf("got dir: cClean\n")
	}
	isnew := map[string]bool{}
	for _, cd := range cds {
		if cd["path"] == "/Ctl" {
			continue
		}
		nm := cd["name"]
		isnew[nm] = true
		cf, ok := mf.child[nm]
		if ok {
			cf.Lock()
			cf.gotMeta(cd)
			cf.Unlock()
		} else {
			var mc *mCache
			nf, _ := mc.newFile(cd)
			mf.child[nm] = nf
			nf.c = mf.c
		}
	}
	for nm, cf := range mf.child {
		if !isnew[nm] {
			cf.Lock()
			if cf.sts != cData && cf.sts != cGone {
				cf.gone()
				delete(mf.child, nm)
			}
			cf.Unlock()
		}
	}
	return nil
}

func (mf *mFile) gone() {
	mf.sts = cGone
	mf.Dprintf("gone\n")
	for nm, cf := range mf.child {
		cf.Lock()
		cf.gone()
		delete(mf.child, nm)
		cf.Unlock()
	}
	mf.data.Reset()
}

func (mf *mFile) del() {
	mf.sts = cDel
	mf.Dprintf("deleted\n")
	mf.wd = nil
	mf.data.Reset()
	for _, cf := range mf.child {
		cf.Lock()
		cf.del()
		cf.Unlock()
	}
}

func (mf *mFile) remove(all bool) error {
	if mf.d["path"] == "/" {
		return errors.New("remove /: too dangerous")
	}
	if mf.d["type"] == "d" && !all {
		for _, cf := range mf.child {
			if cf.sts != cDel && cf.sts != cGone {
				return fmt.Errorf("remove %s: %s",
					mf, zx.ErrNotEmpty)
			}
		}
	}
	mf.del()
	return nil
}

func (mf *mFile) newFile(d zx.Dir, rfs zx.Fs) (fsFile, error) {
	nm := d["name"]
	oc, ok := mf.child[nm]
	if ok {
		// must sync previous dels if we changed the file type or want
		// to create a different dir.
		fs, ok := rfs.(zx.RWFs)
		if !ok {
			return nil, errors.New("rfs does not support put/wstat/remove")
		}
		oc.Lock()
		must := oc.sts == cDel && (oc.d["type"] == "'d" || oc.d["type"] != d["type"])
		oc.Unlock()
		if must {
			oc.sync(fs)
		}
	}
	var mc *mCache
	nf, err := mc.newFile(d)
	if err != nil {
		return nil, err
	}
	nf.sts = cData
	mf.child[nm] = nf
	nf.c = mf.c
	mf.Dprintf("new file %s\n", d["path"])
	return nf, nil
}

// Caution: called with mf locked but must release the lock
func (mf *mFile) getData(off, count int64, c chan<- []byte) error {
	data := mf.data
	mf.Unlock()
	// Data is locked and we have GC, it can't just go
	n, nm, err := data.SendTo(off, count, c)
	mf.Dprintf("get sent %d bytes %d msgs sts %v\n", n, nm, err)
	if err == io.EOF {
		err = nil
	}
	return err
}

// Caution: called with mf locked but must release the lock
// Will set mtime at the end
func (mf *mFile) putData(off int64, c <-chan []byte, umtime string) error {
	data := mf.data
	mf.dirtyData()
	mf.Unlock()
	// Data is locked and we have GC, it can't just go
	n, nm, err := data.RecvFrom(off, c)
	mf.Dprintf("put data %d bytes %d msgs sts %v\n", n, nm, err)
	mf.Lock()
	mf.dirtyData()
	mf.d.SetSize(int64(mf.data.Len()))
	if mf.sts != cDel {
		if umtime != "" {
			mf.d["mtime"] = umtime
		} else {
			mf.d.SetTime("mtime", time.Now())
		}
		if mf.wd == nil {
			mf.wd = zx.Dir{}
		}
		mf.wd["mtime"] = mf.d["mtime"]
		mf.dirtyData()
	}
	mf.Unlock()
	return err
}

func (mf *mFile) xgetDir(all bool) ([]zx.Dir, error) {
	if len(mf.child) == 0 || (!all && (mf.sts == cDel || mf.sts == cGone)) {
		return nil, nil
	}
	ds := make([]zx.Dir, 0, len(mf.child))
	for _, cf := range mf.child {
		cf.Lock()
		if all || (cf.sts != cDel && cf.sts != cGone) {
			ds = append(ds, cf.d.Dup())
		}
		cf.Unlock()
	}
	zx.SortDirs(ds)
	return ds, nil
}

func (mf *mFile) getDir() ([]zx.Dir, error) {
	return mf.xgetDir(false)
}

// for sync: return a list of children files and dirs
func (mf *mFile) children() ([]*mFile, []*mFile) {
	if len(mf.child) == 0 {
		return nil, nil
	}
	ds := make([]*mFile, 0, len(mf.child))
	cs := make([]*mFile, 0, len(mf.child))
	for _, cf := range mf.child {
		cf.Lock()
		if cf.sts != cGone {
			if cf.d["type"] != "d" {
				cs = append(cs, cf)
			} else {
				ds = append(ds, cf)
			}
		}
		cf.Unlock()
	}
	return cs, ds
}

func (mf *mFile) invalAll() {
	mf.Lock()
	if mf.sts != cDel && mf.sts != cGone {
		mf.inval()
		for _, cf := range mf.child {
			cf.invalAll()
		}
	}
	mf.Unlock()
}

func (mf *mFile) sync(fs zx.Fs) error {
	rfs, ok := fs.(zx.RWFs)
	if !ok {
		return errors.New("rfs does not support put/wstat/remove")
	}
	var err error
	mf.Lock()
	switch mf.sts {
	case cDel: // try to del children first
		mf.sts = cGone // can't do anything else
		for _, cf := range mf.child {
			if e := cf.sync(rfs); e != nil && err == nil {
				err = e
			}
		}
		mf.vprintf("sync: remove %s", mf)
		mf.Dprintf("sync: rm, cGone\n")
		if e := <-rfs.Remove(mf.d["path"]); e != nil {
			if !zx.IsNotExist(e) {
				err = e
				dbg.Warn("sync: rm: %s", err)
			}
		}
		mf.Unlock()
		return err
	case cGone, cNew, cClean:
	case cNewMeta, cMeta:
		mf.sts = cNew
		mf.vprintf("sync: wstat %s", mf)
		mf.Dprintf("sync: wstat, cNew\n")
		wc := rfs.Wstat(mf.d["path"], mf.wd)
		wd := <-wc
		if err = cerror(wc); err != nil {
			dbg.Warn("sync: wstat: %s", err)
		}
		// else, we could update our stat with the returned one
		_ = wd
		mf.wd = nil
	case cData:
		mf.sts = cClean
		mf.vprintf("sync: put %s", mf)
		mf.Dprintf("sync: put, cClean\n")
		c := make(chan []byte)
		if mf.d["type"] == "d" {
			close(c)
		}
		rc := rfs.Put(mf.d["path"], mf.d, 0, c)
		if mf.d["type"] != "d" {
			// NB: we don't unlock to sync a single version.
			_, _, err = mf.data.SendTo(0, -1, c)
			close(c, err)
			if err != nil {
				dbg.Warn("sync: send: %s", err)
			}
		}
		pd := <-rc
		if err = cerror(rc); err != nil {
			dbg.Warn("sync: put: %s", err)
		}
		// else, we could update our stat with the returned one
		_ = pd
		mf.wd = nil
	}
	// We copy the children pointers to avoid locking the
	// entire tree while we sync
	cs, ds := mf.children()
	mf.Unlock()
	var wg sync.WaitGroup
	for _, c := range cs {
		c := c
		// We sync all files concurrently, perhaps
		// this is too much; we'll see.
		wg.Add(1)
		go func() {
			c.sync(rfs)
			wg.Done()
		}()
	}
	wg.Wait()
	for _, c := range ds {
		if e := c.sync(rfs); e != nil && err == nil {
			err = e
		}
	}
	return err
}

// retains a ref to d and may change it.
func (mc *mCache) newFile(d zx.Dir) (*mFile, error) {
	f := &mFile{
		cFile: cFile{
			d: d,
		},
		c:   mc,
		sts: cNew,
		t:   time.Now(),
	}
	if d["type"] == "d" {
		f.child = map[string]*mFile{}
	} else {
		f.data = &mblk.Buffer{}
	}
	return f, nil
}

func (mc *mCache) setRoot(d zx.Dir) (err error) {
	if mc.slash != nil {
		return errors.New("root already set")
	}
	mc.slash, err = mc.newFile(d)
	return err
}

func (mc *mCache) root() fsFile {
	return mc.slash
}

func (mc *mCache) sync(rfs zx.Fs) error {
	if mc.Debug {
		mc.Dprintf("pre sync\n")
		mc.dump()
	}
	fs, ok := rfs.(zx.RWFs)
	if !ok {
		return errors.New("rfs does not support put/wstat/remove")
	}
	return mc.slash.sync(fs)
}

func (mf *mFile) dump(w io.Writer, lvl int) {
	fmt.Fprintf(w, "%s\t%s\n", mf.d.Fmt(), mf.sts)
	if len(mf.wd) > 0 {
		fmt.Fprintf(w, "    wd %s\n", mf.wd)
	}
	if mf.d["type"] != "d" {
		fmt.Fprintf(w, "  data[%d]\n", mf.data.Len())
	}
	ds, _ := mf.xgetDir(true)
	for _, d := range ds {
		cf := mf.child[d["name"]]
		if cf != nil {
			cf.dump(w, lvl+1)
		}
	}
}

func (mc *mCache) inval() {
	mc.slash.invalAll()
}

func (mc *mCache) dump() {
	fmt.Fprintf(os.Stderr, "cache dump:\n")
	mc.slash.dump(os.Stderr, 0)
}

func (mc *mCache) contents() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "cache dump:\n")
	mc.slash.dump(&buf, 0)
	return buf.String()
}
