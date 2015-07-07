/*
	Caching zx FS

	Keeps an in-memory cache for another zx.Tree.

	Non-updating operations (including stats) are fully
	served from the in-memory cache.

	Updating operations are first performed on the zx.Tree
	and then applied to the in-memory cache (writhe-through).

	No cached content is ever evicted. This might change in the future if
	our assumption that memory is plenty to keep file trees fully cached
	proves false.

	It is wise not to use cfs to cache a dump file tree.
*/
package ocfs

// REFERENCE(x): clive/cmd/zxfs, fuse driver with optional cache

// REFERENCE(x): clive/cmd/zx, zx file server with optional cache

/*
	The FS keeps one Dir per known file, in a tree.

	A Dir is created when the file is first seen.
	Its data[]/child[] slices are nil until we read the whole
	data or children entries from the server.

	When modifying the data or creating/removing child files,
	the operation is performed on-disk and then the on-memory
	image is updated.

	By design, there can be no pending operations for the underlying tree.

	To survive, old entries are checked out on the underlying tree to
	invalidate the cached data if it was changed from outside.

	Errors or any other problem on the remote tree might lead to
	discards of cached data.

	LOCKING:

	When we can, it is done one file at a time.
	Otherwise, the order is parent, then child.

	CACHING:

	A dir may be:
	- unread	(eg., retrieved from parent's readdir)
		In this case its child/data fields are nil
	- read	its data (or list of children) has been retrieved
		In this case its child/data fields are not nil (might be empty)
	- ghost	it's been removed (or tried to) and no longer exists in the tree.
		Anyone who sees a ghost entry may remove it from the
		children list.

	After the cfs has been created, the server code calls cfs.ServerFor() for each
	client, which returns a copy of the Cfs structure sharing everything but for
	the client info.
	In this case the cache to post invalidations for changes made by
	each client to all clients.
	Sum attributes are computed everytime the data is updated.
	External changes made to the underlying fs are posted by the initial cfs,
	which uses a null client info.

	Cfs servers using a non-invalidating tree, that serve readers of /Chgs, poll
	the underlying tree for changes from time to time.
*/

import (
	"clive/bufs"
	"clive/dbg"
	"clive/net/auth"
	"clive/zx"
	"clive/zx/lfs"
	"clive/zx/pred"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"sync"
	"time"
	"os"
)

// changes made
type chg  {
	d  zx.Dir         // what changed (dup of zd.d)
	ci *zx.ClientInfo // who changed it
}

// request to get changes
type getchgreq  {
	getc chan chg       // where to post them
	ci   *zx.ClientInfo // who issued the get changes request
}

// Debug fields that can be set by the user.
type CfsDebug  {
	IOstats *zx.IOstats // set to &zx.IOstats{} to account

	// calls made to cfs
	Debug bool       // requested ops and general debug
	Zdebug bool // operations performed on the remote tree
	// cache operations
	Cdebug bool
}

/*
	Implementation of a caching FS for a zx.Tree
	Keeps an in-memory cache of another zx.Tree to serve requests.

	When auth info is supplied, it performs permission checking
	according to clive's rules. In such case, the underlying file tree
	should make the Uid/Gid/Wuid attributes persist (eg., like zx/lfs when
	lfs.SaveAttrs is set to true).

	Those files with no recorded uid/gid/muid have dbg.Usr set for them.
*/
type Cfs  {
	Tag  string
	fs   zx.Tree
	root *Dir
	*sync.RWMutex
	ai *auth.Info
	ci *zx.ClientInfo

	wholk *sync.Mutex
	who   map[string]bool

	ronly bool

	epoch   *int           // # of opens of the underlying tree's /Chg (0 if none)
	chgc    chan chg       // to post changes to readers of /Chg
	getchgc chan getchgreq // to ask for changes for readers of /Chg
	pollc   chan int       // new/gone /Chg readers notify here
	*CfsDebug

	dprintf func(ts1, ts2 string, args ...interface{})
	zprintf func(ts1, ts2 string, args ...interface{})
	cprintf func(ts1, ts2 string, args ...interface{})
}

// These should not be changed after calling any function.
var (
	// Interval for polling the fs (see comment in pollProc).
	// When a file is used by a client we check out to see
	// if it was changed externally.
	// But, clients receiving invalidations won't receive
	// invalidations for files changed externally unless we
	// poll the underlying tree for changes, and this is the
	// polling interval.
	// Each interval, we stat the entire tree.
	CachePollIval = 5*time.Minute

	// Timeout in cache before refreshing meta-data
	CacheMetaTout = 5*time.Minute
)

var (
	// make sure we implement the right interfaces
	_fs  *Cfs
	_t   zx.RWTree     = _fs
	_fn  zx.Finder     = _fs
	_r   zx.Recver     = _fs
	_snd zx.Sender     = _fs
	_g   zx.Getter     = _fs
	_w   zx.Walker     = _fs
	_s   zx.Stater     = _fs
	_st  zx.ServerTree = _fs
	_a   zx.AuthTree   = _fs
)

// Set to enable debug on all trees; Already created trees are not affected.
var Debug, Zdebug, Cdebug bool

// Nop. But implemented to make cfs implement the Finder interface.
func (z *Cfs) Fsys(name string) <-chan error {
	c := make(chan error, 1)
	close(c)
	return c
}

func (z *Cfs) String() string {
	return z.Tag
}

// Arguments to New.
const (
	RO = true
	RW = false
)

func mkDprintf(tag string, flag *bool) func(ts1, ts2 string, args ...interface{}) {
	return func(ts1, ts2 string, args ...interface{}) {
		ts := tag + ":" + ts1 + " " + ts2
		fmt.Fprintf(os.Stderr, ts, args...)
	}
}

// Create a new cached tree for the given tree .
// Operations are performed on behalf of each file owner.
func New(tag string, fs zx.Tree, rdonly bool) (*Cfs, error) {
	dc := fs.Stat("/")
	d := <-dc
	if d == nil {
		return nil, cerror(dc)
	}
	epoch := 0
	z := &Cfs{
		Tag:   tag,
		fs:    fs,
		ronly: rdonly,
		root: &Dir{
			d:    d,
			path: "/",
		},
		RWMutex: &sync.RWMutex{},
		epoch:   &epoch,
		chgc:    make(chan chg, 15),
		getchgc: make(chan getchgreq),
		who:     make(map[string]bool),
		wholk:   &sync.Mutex{},
		CfsDebug: &CfsDebug{
			Debug:  Debug,
			Zdebug: Zdebug,
			Cdebug: Cdebug,
		},
		pollc: make(chan int, 50),
	}
	if tag == "" {
		z.Tag = "cfs!" + fs.Name()
	}
	z.dprintf = mkDprintf(z.Tag, &z.Debug)
	z.zprintf = mkDprintf("    >"+z.Tag, &z.Zdebug)
	z.cprintf = mkDprintf("    "+z.Tag, &z.Cdebug)
	z.root.z = z
	z.root.mode = uint(d.Uint64("mode")&0777)
	z.root.refreshMeta()
	go z.chgProc()
	go z.pollProc()
	getc := z.fs.Get("/Chg", 0, zx.All, "")
	x := <-getc
	if len(x) == 0 {
		z.dprintf("/Chg", "no invalidations", cerror(getc))
	} else {
		*z.epoch++
		z.root.epoch = *z.epoch
		go z.invalProc(getc)
	}
	return z, nil
}

// Implement the rfs LogInOutTree interface.
func (z *Cfs) LogIn(who string) {
	if z==nil || z.who==nil {
		return
	}
	z.wholk.Lock()
	z.who[who] = true
	z.wholk.Unlock()
}

// Implement the rfs LogInOutTree interface.
func (z *Cfs) LogOut(who string) {
	if z==nil || z.who==nil {
		return
	}
	z.wholk.Lock()
	delete(z.who, who)
	z.wholk.Unlock()
}

// Return a Cfs sharing everything with cfs, but performing its requests
// on behalf of the given auth info. Implements the zx.AuthTree interface.
func (z *Cfs) AuthFor(ai *auth.Info) (zx.Tree, error) {
	ncfs := &Cfs{}
	*ncfs = *z
	ncfs.ai = ai
	if ai != nil {
		z.dprintf("/", "auth", "for %s\n", ai.Uid)
	}
	return ncfs, nil
}

// Return a Cfs sharing everything with cfs, but performing its requests
// on behalf of the given client info. Implements the zx/rfs.ServerTree interface.
func (z *Cfs) ServerFor(ci *zx.ClientInfo) (zx.Tree, error) {
	ncfs := &Cfs{}
	*ncfs = *z
	if ci != nil {
		uid := "none"
		if ci.Ai != nil {
			uid = ci.Ai.Uid
			ncfs.ai = ci.Ai
		}
		z.dprintf("/", "serve", "for %s %d %s\n", ci.Tag, ci.Id, uid)
	}
	ncfs.ci = ci
	return ncfs, nil
}

func (z *Cfs) Stats() *zx.IOstats {
	return z.IOstats
}

// Dump a debug representation of the state of z into w, with lvl indent.
func (z *Cfs) DumpTo(w io.Writer, lvl int) {
	zd := z.root
	zd.DumpTo(w, lvl)
}

// walk to the given dir
func (z *Cfs) walk(path string) (*Dir, error) {
	zd := z.root
	elems := zx.Elems(path)
	for len(elems) > 0 {
		zd.RLock()
		if zd.d["type"] != "d" {
			zd.RUnlock()
			return nil, fmt.Errorf("%s: %s", zd.path, dbg.ErrNotDir)
		}
		if err := zd.updData(true); err != nil {
			zd.RUnlock()
			return nil, err
		}
		if !zd.canWalk(z.ai) {
			zd.RUnlock()
			return nil, fmt.Errorf("%s: %s", zd.path, dbg.ErrPerm)
		}
		zcd, ok := zd.child[elems[0]]
		if !ok {
			zd.RUnlock()
			return nil, fmt.Errorf("%s: %s: %s", zd.path, elems[0], dbg.ErrNotExist)
		}
		elems = elems[1:]
		zd.RUnlock()
		zd = zcd
	}
	return zd, nil
}

func (z *Cfs) Name() string {
	return z.String()
}

func (z *Cfs) setwuid(d zx.Dir) zx.Dir {
	if d == nil {
		d = zx.Dir{}
	}
	d["Wuid"] = dbg.Usr
	if z.ai != nil {
		d["Wuid"] = z.ai.Uid
	}
	return d
}

// called with zd rlocked to check out that zx has Uid, Gid, and Muid.
// If it does not, it defines them and wstats the underlying file,
// (replacing temporarily the rlock with a lock).
// If the parent file has a Uid/Gid set, it is used to initialize them,
// otherwise, if the file path starts with /usr/<name>, <name> is used
// as the Uid and Gid,
// otherwise, the user running Cfs is used as a default.
func (z *Cfs) setUids(zd *Dir) {
	nd := zd.d
	if nd["Uid"] != "" {
		return
	}
	zd.RUnlock()
	zd.Lock()
	defer func() {
		zd.Unlock()
		zd.RLock()
	}()
	if nd["Uid"] != "" { // race
		return
	}
	uid, gid := "", ""
	if zx.HasPrefix(zd.path, "/usr") {
		els := zx.Elems(zd.path)
		if len(els) >= 2 {
			uid, gid = els[1], els[1]
		}
	}
	if p := zd.parent; p!=nil && (uid=="" || gid=="") {
		uid, gid = p.d["Uid"], p.d["Gid"]
	}
	if uid == "" {
		uid = dbg.Usr
	}
	if gid == "" {
		gid = dbg.Usr
	}
	nd["Uid"] = uid
	nd["Gid"] = gid
	nd["Wuid"] = dbg.Usr
	zd.d = nd
	zd.wstatAttrs("Uid", "Gid", "Wuid")
}

func (z *Cfs) stat(path string) (zx.Dir, error) {
	path, err := zx.AbsPath(path)
	if err != nil {
		return nil, err
	}
	if path == "/Ctl" {
		return zx.Dir{
			"path": "/Ctl",
			"name": "Ctl",
			"size": "0",
			"type": "-",
			"Uid":  dbg.Usr,
			"Gid":  dbg.Usr,
			"Wuid": dbg.Usr,
			"Sum":  zsum,
			"mode": "0644",
		}, nil
	}
	zd, err := z.walk(path)
	if err != nil {
		return nil, err
	}
	zd.RLock()
	defer zd.RUnlock()
	if err := zd.updMeta(true); err != nil {
		return nil, err
	}
	z.setUids(zd)
	nd := zd.d.Dup()
	return nd, nil

}

func (z *Cfs) Stat(path string) chan zx.Dir {
	z.dprintf(path, "Stat")
	cs := z.IOstats.NewCall(zx.Sstat)
	dc := make(chan zx.Dir, 1)
	go func() {
		d, err := z.stat(path)
		if err != nil {
			cs.End(true)
			z.dprintf(path, "stat", err)
			close(dc, err)
			return
		}
		dc <- d
		close(dc)
		z.dprintf(path, "stat", d.TestFmt())
		cs.End(false)
	}()
	return dc
}

func (z *Cfs) Close(e error) {
	z.dprintf("/", "close")
	z.root.wait()
	close(z.chgc, "closing")

}

var ErrNoMatch = errors.New("false")

func (z *Cfs) get(path string, off, count int64, c chan []byte, cs *zx.CallStat, fpred string) (int, int64, error) {
	path, err := zx.AbsPath(path)
	if err != nil {
		return 0, 0, err
	}
	if path == "/Ctl" {
		return z.getctl(off, count, c, cs)
	}
	if path == "/Chg" {
		cs.End(false) // don't account for this; will block.
		return z.getchg(off, count, c, cs)
	}
	zd, err := z.walk(path)
	if err != nil {
		return 0, 0, err
	}
	zd.RLock()
	if err := zd.updData(true); err != nil {
		zd.RUnlock()
		return 0, 0, err
	}
	if err==nil && !zd.canRead(z.ai) {
		err = dbg.ErrPerm
	}
	if err == nil {
		err = zd.match(fpred)
	}
	if err != nil {
		zd.RUnlock()
		return 0, 0, err
	}
	if off < 0 {
		off = 0
	}
	if zd.d["type"] != "d" {
		if count < 0 {
			count = int64(zd.data.Len())
		}
		cs.Sending()
		nb, nm, err := zd.data.SendTo(off, count, c)
		zd.RUnlock()
		cs.Sends(int64(nm), nb)
		return nm, nb, err
	}

	ds := zd.children()
	zd.RUnlock()
	sort.Sort(byName(ds))
	nzcd := 0
	// the cached fs is in the end a lfs or someone else with a /Ctl file,
	// no need to add an entry for /Ctl.
	for _, zcd := range ds {
		if zcd.ghost {
			continue
		}
		if count == 0 {
			break
		}
		count--
		zcd.RLock()
		z.setUids(zcd)
		msg := zcd.d.Pack()
		zcd.RUnlock()
		cs.Send(0)
		if ok := c <- msg; !ok {
			return nzcd, 0, cerror(c)
		}
		nzcd++
	}
	return nzcd, 0, nil

}

func (z *Cfs) Get(path string, off, count int64, pred string) <-chan []byte {
	z.dprintf(path, "Get", "%d %d %q\n", off, count, pred)
	cs := z.IOstats.NewCall(zx.Sget)
	c := make(chan []byte)
	go func() {
		nm, nb, err := z.get(path, off, count, c, cs, pred)
		if err != nil {
			z.dprintf(path, "get", err)
		} else {
			z.dprintf(path, "get", "%d msgs %d bytes\n", nm, nb)
		}
		cs.End(err != nil)
		close(c, err)
	}()
	return c
}

// XXX: BUG: somehow this fails when spref != dpref. Must check this out.
func (z *Cfs) find(path, fpred, spref, dpref string, depth int, dc chan zx.Dir, cs *zx.CallStat) (int, error) {
	path, err := zx.AbsPath(path)
	if err != nil {
		return 0, err
	}
	zd, err := z.walk(path)
	if err != nil {
		return 0, fmt.Errorf("find: walk: %s", err)
	}
	zd.RLock()
	if zd.d["type"] == "d" {
		if err := zd.updData(true); err != nil {
			zd.RUnlock()
			return 0, err
		}
	} else {
		if err := zd.updMeta(true); err != nil {
			zd.RUnlock()
			return 0, err
		}
	}
	p, err := pred.New(fpred)
	if err != nil {
		zd.RUnlock()
		return 0, fmt.Errorf("pred: %s", err)
	}
	// find unlocks zd
	cs.Sending()
	d := zd.d.Dup()
	if spref != dpref {
		suff := zx.Suffix(d["path"], spref)
		d["path"] = zx.Path(dpref, suff)
	}
	return zd.find(d, p, spref, dpref, depth, dc, z.ai)
}

func (z *Cfs) Find(path, fpred, spref, dpref string, depth int) <-chan zx.Dir {
	z.dprintf(path, "Find", "pred %q", fpred)
	cs := z.IOstats.NewCall(zx.Sfind)
	dc := make(chan zx.Dir)
	go func() {
		n, err := z.find(path, fpred, spref, dpref, depth, dc, cs)
		if err != nil {
			z.dprintf(path, "find", err)
		} else {
			z.dprintf(path, "find", "%d msgs\n", n)
		}
		cs.End(err != nil)
		close(dc, err)
	}()
	return dc
}

func (z *Cfs) FindGet(path, fpred, spref, dpref string, depth int) <-chan zx.DirData {
	z.dprintf(path, "Findget", "pred %q", fpred)
	cs := z.IOstats.NewCall(zx.Sfindget)
	gc := make(chan zx.DirData)
	go func() {
		dc := z.Find(path, fpred, spref, dpref, depth) // BUG: will stat a Sfind
		for d := range dc {
			g := zx.DirData{Dir: d}
			var datac chan []byte
			if d["err"]=="" && d["type"]=="-" {
				datac = make(chan []byte)
				g.Datac = datac
			}
			if ok := gc <- g; !ok {
				close(dc, cerror(gc))
				break
			}
			if datac != nil {
				if d["spath"] == "" {
					d["spath"] = d["path"]
				}
				_, _, err := z.get(d["spath"], 0, zx.All, datac, nil, "")
				close(datac, err)
			}
		}

		z.dprintf(path, "find", "done")
		err := cerror(dc)
		cs.End(err != nil)
		close(gc, err)
	}()
	return gc
}

func (z *Cfs) create(fpath string, d zx.Dir, fpred string) (*Dir, error) {
	ppath, elem := path.Dir(fpath), path.Base(fpath)
	pzd, err := z.walk(ppath)
	if err != nil {
		return nil, err
	}
	pzd.Lock()
	defer pzd.Unlock()
	if pzd.d["type"] != "d" {
		return nil, dbg.ErrNotDir
	}
	if err := pzd.updData(false); err != nil {
		return nil, err
	}
	if !pzd.canWrite(z.ai) {
		return nil, dbg.ErrPerm
	}
	d["Uid"] = d["Wuid"]
	if m := d.Int64("mode"); m!=0 && pzd.mode!=0 {
		pmode := int64(pzd.mode)
		m &= pmode
		if pmode&020!=0 && m&0200!=0 {
			m |= 020
		}
		if pmode&010!=0 && m&0100!=0 {
			m |= 010
		}
		if pmode&040!=0 && m&0400!=0 {
			m |= 040
		}
		d["mode"] = "0" + strconv.FormatInt(m, 8)
	}
	if pg := pzd.d["Gid"]; pg!="" && d["Gid"]=="" {
		d["Gid"] = pg
	}
	zd, ok := pzd.child[elem]
	if !ok || zd.ghost {
		cd := zx.Dir{}
		for k, v := range d {
			cd[k] = v
		}
		cd["name"] = elem
		cd["path"] = fpath
		cd["type"] = "-"
		zd = z.newDir(cd)
		zd.Lock()
		defer zd.Unlock()
		zd.refreshMeta()
		if err := zd.match(fpred); err != nil {
			return nil, err
		}

		cd["size"] = "0"
		if z.ai != nil {
			cd["Uid"] = z.ai.Uid
		} else {
			cd["Uid"] = dbg.Usr
		}
		pzd.addChild(elem, zd)
	} else {
		zd.Lock()
		defer zd.Unlock()
		if err := zd.updMeta(false); err != nil {
			return nil, err
		}
		if !zd.canWrite(z.ai) {
			return nil, dbg.ErrPerm
		}
		if err := zd.match(fpred); err != nil {
			return nil, err
		}
	}
	if zd.d["type"] == "d" {
		return nil, dbg.ErrIsDir
	}
	zd.data = &bufs.Blocks{}
	zd.d["Sum"] = zsum
	zd.refreshData()
	return zd, nil
}

func noRootPath(path string) (string, error) {
	p, err := zx.AbsPath(path)
	if err==nil && p=="/" {
		return p, errors.New("won't do on /")
	}
	return p, err
}

func (z *Cfs) Put(fpath string, d zx.Dir, off int64, datc <-chan []byte, pred string) chan zx.Dir {
	z.dprintf(fpath, "Put", "off %d dir %v\n", off, d)
	cs := z.IOstats.NewCall(zx.Sput)
	dc := make(chan zx.Dir, 1)
	go func() {
		fpath, err := noRootPath(fpath)
		if err != nil {
			z.dprintf(fpath, "put", err)
			cs.End(err != nil)
			close(datc, err)
			close(dc, err)
			return
		}
		if fpath == "/Ctl" {
			err := z.putctl(datc, dc)
			cs.End(err != nil)
			return
		}
		_, ok := z.fs.(zx.RWTree)
		if !ok || z.ronly {
			z.dprintf(fpath, "put", dbg.ErrRO)
			close(datc, dbg.ErrRO)
			close(dc, dbg.ErrRO)
			cs.End(err != nil)
			return
		}
		var sz int64
		var nm int
		var zd *Dir
		if d["Uid"]!="" && !z.ai.InGroup("sys") {
			delete(d, "Uid")
		}
		if d["Gid"]!="" && !z.ai.InGroup("sys") && !z.ai.InGroup(d["Gid"]) {
			delete(d, "Gid")
		}
		d = z.setwuid(d)

		if d["mode"] != "" {
			// create checks perms on zd and checks out fpred.
			zd, err = z.create(fpath, d, pred)
		} else {
			zd, err = z.walk(fpath)
			if err==nil && !zd.canWrite(z.ai) {
				err = dbg.ErrPerm
			}
			if err == nil {
				err = zd.match(pred)
			}
		}
		if err == nil {
			cs.Sending()
			nm, sz, err = zd.write(d, off, datc)
			if err != nil {
				zd.kill("put")
			}
			cs.Sends(int64(nm), sz)
		}

		if err != nil {
			z.dprintf(fpath, "put", err)
		} else {
			dc <- zx.Dir{
				"size":  zd.d["size"],
				"mtime": zd.d["mtime"],
				"Sum":   zd.d["Sum"],
				"vers":  zd.d["vers"],
			}
			sv := zd.d["Sum"]
			if len(sv) > 6 {
				sv = sv[:6] + "..."
			}
			z.dprintf(fpath, "put", "[%d] sz %s v %s sum %s\n",
				sz, zd.d["size"], zd.d["vers"], sv)
		}
		close(dc, err)
		close(datc, err)
		cs.End(err != nil)
		z.changed(zd)
	}()
	return dc
}

func (z *Cfs) mkdir(fpath string, d zx.Dir) error {
	fpath, err := noRootPath(fpath)
	if err != nil {
		return err
	}
	wfs, ok := z.fs.(zx.RWTree)
	if !ok || fpath=="/Ctl" || z.ronly {
		return dbg.ErrRO
	}
	ppath, elem := path.Dir(fpath), path.Base(fpath)
	zd, err := z.walk(ppath)
	if err != nil {
		return err
	}
	zd.Lock()
	defer zd.Unlock()
	if zd.d["type"] != "d" {
		return dbg.ErrNotDir
	}
	if err := zd.updData(false); err != nil {
		return err
	}
	if !zd.canWrite(z.ai) || d==nil || d["mode"]=="" {
		return dbg.ErrPerm
	}
	if d["Uid"]!="" && !z.ai.InGroup("sys") {
		delete(d, "Uid")
	}
	if d["Gid"]!="" && !z.ai.InGroup("sys") && !z.ai.InGroup(d["Gid"]) {
		delete(d, "Gid")
	}
	if pg := zd.d["Gid"]; pg!="" && d["Gid"]=="" {
		d["Gid"] = pg
	}
	if z.ai != nil {
		d["Uid"] = z.ai.Uid
	} else {
		d["Uid"] = dbg.Usr
	}
	if d["Gid"] == "" {
		d["Gid"] = d["Uid"]
	}
	d["Wuid"] = d["Uid"]
	z.zprintf(fpath, "Lmkdir", "%v\n", d)
	err = <-wfs.Mkdir(fpath, d)
	if err != nil {
		return err
	}
	zcd, ok := zd.child[elem]
	if ok {
		zcd.Lock()
		defer zcd.Unlock()
		zcd.kill("mkdir")
		delete(zd.child, elem)
	}
	zd.zprintf("Lstat")
	d = <-wfs.Stat(fpath)
	if d == nil {
		zd.invalData()
	} else {
		zcd := z.newDir(d)
		zcd.d["Sum"] = zsum
		zcd.child = map[string]*Dir{}
		zcd.refreshMeta()
		zcd.refreshData()

		zd.addChild(elem, zcd)
		zd.refreshMeta()
		zd.refreshData()
	}
	z.changed(zd)
	return nil
}

func (z *Cfs) Mkdir(fpath string, d zx.Dir) chan error {
	z.dprintf(fpath, "Mkdir", "dir %v\n", d)
	cs := z.IOstats.NewCall(zx.Smkdir)
	ec := make(chan error, 1)
	go func() {
		err := z.mkdir(fpath, d)
		if err != nil {
			z.dprintf(fpath, "mkdir", err)
		} else {
			z.dprintf(fpath, "mkdir", "ok")
		}
		ec <- err
		cs.End(err != nil)
		close(ec, err)
	}()
	return ec
}

func (z *Cfs) move(from, to string) error {
	from, err := noRootPath(from)
	if err != nil {
		return err
	}
	to, err = noRootPath(to)
	if err != nil {
		return err
	}
	wfs, ok := z.fs.(zx.RWTree)
	if !ok || from=="/Ctl" || to=="/Ctl" || z.ronly {
		return dbg.ErrRO
	}
	pfrom, err := z.walk(from)
	if err != nil {
		return err
	}
	if !pfrom.canWrite(z.ai) {
		return fmt.Errorf("%s: %s", from, dbg.ErrPerm)
	}
	pto, err := z.walk(to)
	if err==nil && !pto.canWrite(z.ai) {
		return fmt.Errorf("%s: %s", to, dbg.ErrPerm)
	}

	ppfrom, err := z.walk(path.Dir(from))
	if err != nil {
		return err
	}
	if !ppfrom.canWrite(z.ai) {
		return fmt.Errorf("%s: %s", path.Dir(from), dbg.ErrPerm)
	}
	ppto, err := z.walk(path.Dir(to))
	if err != nil {
		return err
	}
	if !ppto.canWrite(z.ai) {
		return fmt.Errorf("%s: %s", path.Dir(to), dbg.ErrPerm)
	}
	if ppto.d["type"] != "d" {
		return fmt.Errorf("%s: %s", path.Dir(to), dbg.ErrNotDir)
	}

	ppfrom.z.zprintf(from, "Lmove")
	err = <-wfs.Move(from, to)

	pfrom.Lock()
	pfrom.child = nil
	pfrom.Unlock()

	ppfrom.Lock()
	ppfrom.getDir()
	ppfrom.Unlock()

	if ppfrom != ppto {
		ppto.Lock()
		ppto.getDir()
		ppto.Unlock()
	}
	return err
}

func (z *Cfs) Move(from, to string) chan error {
	z.dprintf(from, "Move", "to %s\n", to)
	cs := z.IOstats.NewCall(zx.Smove)
	ec := make(chan error, 1)
	go func() {
		err := z.move(from, to)
		if err != nil {
			z.dprintf(from, "move", err)
		} else {
			z.dprintf(from, "move", "ok")
		}
		ec <- err
		close(ec, err)
		cs.End(err != nil)
	}()
	return ec
}

func (z *Cfs) remove(fpath string, all bool) error {
	fpath, err := noRootPath(fpath)
	if err != nil {
		return err
	}
	wfs, ok := z.fs.(zx.RWTree)
	if !ok || fpath=="/Ctl" || z.ronly {
		return dbg.ErrRO
	}
	zd, err := z.walk(fpath)
	if err != nil {
		return err
	}
	if !zd.canWrite(z.ai) {
		return fmt.Errorf("%s: %s", fpath, dbg.ErrPerm)
	}
	pzd := zd.parent
	pzd.Lock()
	defer pzd.Unlock()
	if !pzd.canWrite(z.ai) {
		return fmt.Errorf("%s: %s", path.Dir(fpath), dbg.ErrPerm)
	}
	zd.Lock()
	defer zd.Unlock()
	if all {
		zd.zprintf("Lremoveall")
		// BUG: does not fully chek permissions on the entire tree.
		err = <-wfs.RemoveAll(fpath)
	} else {
		zd.zprintf("Lremove")
		err = <-wfs.Remove(fpath)
	}
	if err != nil {
		return err
	}
	zd.kill("rm")
	z.changed(zd)
	pzd.newSumSize()
	pzd.refreshMeta()
	pzd.refreshData()
	z.changed(pzd)
	return err
}

func (z *Cfs) Remove(fpath string) chan error {
	z.dprintf(fpath, "Remove")
	cs := z.IOstats.NewCall(zx.Sremove)
	ec := make(chan error, 1)
	go func() {
		err := z.remove(fpath, false)
		if err != nil {
			z.dprintf(fpath, "remove", err)
		} else {
			z.dprintf(fpath, "remove", "ok")
		}
		ec <- err
		close(ec, err)
		cs.End(err != nil)
	}()
	return ec
}

func (z *Cfs) RemoveAll(fpath string) chan error {
	z.dprintf(fpath, "Removeall")
	cs := z.IOstats.NewCall(zx.Sremoveall)
	ec := make(chan error, 1)
	go func() {
		err := z.remove(fpath, true)
		if err != nil {
			z.dprintf(fpath, "removeall", err)
		} else {
			z.dprintf(fpath, "removeall", "ok")
		}
		ec <- err
		close(ec, err)
		cs.End(err != nil)
	}()
	return ec
}

func (z *Cfs) wstat(path string, d zx.Dir) error {
	path, err := zx.AbsPath(path)
	if err != nil {
		return err
	}
	wfs, ok := z.fs.(zx.RWTree)
	if !ok || z.ronly {
		return dbg.ErrRO
	}
	if path == "/Ctl" {
		return nil // ignore wstats in /Ctl
	}
	zd, err := z.walk(path)
	if err != nil {
		return err
	}
	zd.Lock()
	defer zd.Unlock()
	if err := zd.canWstat(z.ai, d); err != nil {
		return err
	}
	zd.zprintf("Lwstat")
	err = <-wfs.Wstat(path, d)
	if err == nil {
		for k, v := range d {
			if zd.d[k] == v {
				continue
			}
			zd.d[k] = v
			if k == "size" {
				if v == "0" {
					zd.data = &bufs.Blocks{}
					zd.d["Sum"] = zsum
					zd.refreshData()
					if _, ok := wfs.(*lfs.Lfs); ok {
						// lfs does not update vers
						nvers := zd.d.Int("vers") + 1
						zd.d["vers"] = strconv.Itoa(nvers)
					}
				} else {
					zd.invalData()
				}
			}
		}
		zd.mode = uint(zd.d.Uint64("mode")&0777)
		zd.refreshMeta()
		z.changed(zd)
	} else {
		zd.invalAll()
	}
	return err
}

func (z *Cfs) Wstat(path string, d zx.Dir) chan error {
	z.dprintf(path, "Wstat", "dir %v\n", d)
	cs := z.IOstats.NewCall(zx.Swstat)
	ec := make(chan error, 1)
	go func() {
		err := z.wstat(path, d)
		if err != nil {
			z.dprintf(path, "wstat", err)
		} else {
			z.dprintf(path, "wstat", "ok")
		}
		ec <- err
		close(ec, err)
		cs.End(err != nil)
	}()
	return ec
}
