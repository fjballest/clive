/*
	Lfs implements the zx Finder, the zx Tree,  and File interfaces for local file systems.

	It adapts underlying file trees from the underlying OS to the Clive interfaces.
*/
package lfs

// REFERENCE: clive/zx(x): for zx interfaces and basic data types.

import (
	"bytes"
	"clive/dbg"
	"clive/nchan"
	"clive/net/auth"
	"clive/zx"
	"clive/zx/pred"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"crypto/sha1"
	"strings"
	"sync"
	"strconv"
	"time"
)

/*
	A local file system implementation using the underlying OS implementation
*/
type Lfs  {
	name string
	path string
	rdonly   bool

	readattrs, saveattrs bool
	ai                   *auth.Info

	*zx.Flags
}

/*
	TODO: Rewrite this to require fewer operations on the underlying OS.
	In many cases, a ZX call issues many more calls than required.
*/


// Arguments to New.
const (
	RO = true
	RW = false
)

var (
	trees   = map[string]*Lfs{}
	treeslk sync.RWMutex

	// When DoSum is set, lfs computes the Sum attribute for
	// files and directories. This can slow down things quite a bit.
	DoSum = false

	// make sure we implement the right interfaces
	_fs  *Lfs
	_t   zx.RWTree   = _fs
	_r   zx.Recver   = _fs
	_snd zx.Sender   = _fs
	_g   zx.Getter   = _fs
	_w   zx.Walker   = _fs
	_s   zx.Stater   = _fs
	_a   zx.AuthTree = _fs
	_d zx.Dumper = _fs
	_D zx.Debugger = _fs
)

var ctldir = zx.Dir{
		"path": "/Ctl",
		"spath": "/Ctl",
		"name": "Ctl",
		"mode": "0644",
		"proto": "lfs",
		"size": "0",
		"type": "c",
		"Uid": dbg.Usr,
		"Gid": dbg.Usr,
		"Wuid": dbg.Usr,
	//	"Sum": zx.Zsum(),
	}

func (t *Lfs) String() string {
	return t.name
}

func init() {
	zx.DefProto("lfs", dial)
}

func dial(d zx.Dir) (zx.Tree, error) {
	tpath := d["tpath"]
	if tpath == "" {
		return nil, errors.New("no tpath")
	}

	treeslk.RLock()
	r := trees[tpath]
	treeslk.RUnlock()
	if r != nil {
		return r, nil
	}

	treeslk.Lock()
	defer treeslk.Unlock()
	r = &Lfs{name: tpath, path: tpath, Flags: &zx.Flags{}}

	trees[tpath] = r
	return r, nil
}

func (t *Lfs) DirFile(df zx.Dir) zx.File {
	return zx.File{t, df}
}

func (t *Lfs) Name() string {
	return t.name
}

func (t *Lfs) dprintf(fs string, args ...interface{}) {
	if t != nil && t.Dbg {
		fmt.Fprintf(os.Stderr, t.name+": " + fs, args...)
	}
}

func New(name, path string, rdonly bool) (*Lfs, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(p)
	if err != nil {
		return nil, err
	}
	l := &Lfs{
		name: name,
		path: p,
		rdonly:   rdonly,
		Flags: &zx.Flags{},
	}
	l.Flags.Add("debug", &l.Dbg)
	l.Flags.AddRO("rdonly", &l.rdonly)
	l.Flags.AddRO("noperm", &l.NoPermCheck)
	l.Flags.Add("clear", func(...string)error{
		l.IOstats.Clear()
		return nil
	})
	return l, nil
}

// Instruct the tree to read user-defined attributes and uids or not.
// SaveAttrs implies this when set.
func (t *Lfs) ReadAttrs(yn bool) {
	t.readattrs = yn
}

// Instruct the tree to save user-defined attributes and uids or not.
// Implies ReadAttrs when set.
// If the tree is read only, attributes are not saved and the call is just like ReadAttrs.
// There is no locking on the attributes file, so at most one zx lfs may be in use
// at a time.
func (t *Lfs) SaveAttrs(yn bool) {
	t.saveattrs = yn
	if t.saveattrs {
		t.readattrs = true
	}
	if t.rdonly {
		t.saveattrs = false
	}
}

// Ask the tree to perform auth checks on behalf of ai.
// Implies ReadAttrs.
func (t *Lfs) AuthFor(ai *auth.Info) (zx.Tree, error) {
	nlfs := &Lfs{}
	*nlfs = *t
	if ai != nil {
		t.dprintf("auth for %s %v\n", ai.Uid, ai.Gids)
	}
	nlfs.ai = ai
	nlfs.readattrs = true
	return nlfs, nil
}

func (t *Lfs) Stats() *zx.IOstats {
	return t.IOstats
}

// return dir size, without considering "/Ctl" in "/"
// return also its Sum value if DoSum
func dirsz(p string) (int, string) {
	fd, err := os.Open(p)
	if err != nil {
		return 0, ""
	}
	defer fd.Close()
	ds, _ := fd.Readdirnames(-1)
	tot := 0
	if !DoSum {
		for _, d := range ds {
			if d != afname {
				tot++
			}
		}
		return tot, ""
	}
	h := sha1.New()
	for _, d := range ds {
		if d != afname {
			tot++
			fmt.Fprintf(h, "%s\n", d)
		}
	}
	sum := h.Sum(nil)
	return tot, fmt.Sprintf("%040x", sum)
}

func (t *Lfs) stat(rid string) (zx.Dir, error) {
	rid, err := zx.AbsPath(rid)
	if err != nil {
		return nil, err
	}
	if rid == "/Ctl" {
		d := ctldir.Dup()
		d["tpath"] = t.path
		return d, nil
	}
	path := zx.Path(t.path, rid)
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	nd := 0
	sum := ""
	if st.IsDir() {
		nd, sum = dirsz(path)
		if rid == "/" {
			nd++
		}
	}
	d := zx.NewDir(st, nd)
	if t.readattrs {
		t.fileAttrs(path, d)
	}
	if sum != "" {
		d["Sum"] = sum
	}
	d["tpath"] = t.path
	d["path"] = rid
	d["spath"] = rid
	d["proto"] = "lfs"
	if rid == "/" {
		d["name"] = "/"
	}
	if d["Uid"] == "" {
		d["Uid"] = dbg.Usr
	}
	if d["Gid"] == "" {
		d["Gid"] = dbg.Usr
	}
	if d["Wuid"] == "" {
		d["Wuid"] = dbg.Usr
	}
	return d, nil
}

func (t *Lfs) Stat(rid string) chan zx.Dir {
	t.dprintf("stat %s\n", rid)
	cs := t.IOstats.NewCall(zx.Sstat)
	c := make(chan zx.Dir, 1)
	rid, err := zx.AbsPath(rid)
	var d zx.Dir
	if err == nil {
		d, err = t.stat(rid)
	}
	if err==nil && t.ai!=nil && !t.NoPermCheck {
		_, _, err = t.canWalkTo(rid, 0)
	}
	if err != nil {
		t.dprintf("stat %s: %s\n", rid, err)
		close(c, err)
		cs.End(true)
		return c
	}
	t.dprintf("stat %s: ok\n", rid)
	c <- d
	close(c)
	cs.End(false)
	return c
}

func (t *Lfs) Close(e error) {
	t.dprintf("close sts %v\n", e)
}

func (t *Lfs) getCtl(off, count int64, dc chan<- []byte, cs *zx.CallStat) (int64, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s:\n", t.Name())
	fmt.Fprintf(&buf, "%s", t.Flags)
	t.IOstats.Averages()
	fmt.Fprintf(&buf, "%s\n", t.IOstats.String())

	resp := buf.Bytes()
	o := int(off)
	if o >= len(resp) {
		o = len(resp)
	}
	resp = resp[o:]
	n := int(count)
	if n>len(resp) || n<0 {
		n = len(resp)
	}
	resp = resp[:n]
	tot := int64(len(resp))
	cs.Send(tot)
	dc <- resp
	return tot, nil
}

func (t *Lfs) get(rid string, off, count int64, dc chan<- []byte, cs *zx.CallStat) (int64, error) {
	rid, err := zx.AbsPath(rid)
	if err != nil {
		return 0, err
	}
	if rid == "/Ctl" {
		return t.getCtl(off, count, dc, cs)
	}
	path := zx.Path(t.path, rid)
	fd, err := os.Open(path)
	if err==nil && t.ai!=nil && !t.NoPermCheck {
		_, _, err = t.canWalkTo(rid, 0444)
	}
	if err != nil {
		return 0, err
	}
	defer fd.Close()
	st, err := fd.Stat()
	if err != nil {
		return 0, err
	}
	if off!=0 && !st.IsDir() {
		if _, err := fd.Seek(off, 0); err != nil {
			return 0, err
		}
	}
	if st.IsDir() {
		ds, err := ioutil.ReadDir(path)
		nd := 0
		tot := 0
		ctlsent := false
		var xds map[string]zx.Dir
		if t.readattrs {
			xds, _ = t.dirAttrs(path)
		}
	Dloop:
		for i := 0; i < len(ds); {
			if off > 0 {
				off--
				if !ctlsent && rid=="/" {
					ctlsent = true
				} else {
					i++
				}
				continue
			}
			switch count {
			case zx.All:
				break
			case 0:
				break Dloop
			default:
				count--
			}
			if !ctlsent && rid=="/" {
				ctlsent = true
				nd++
				d := ctldir
				n, _ := d.Send(dc)
				nd++ // but not i++
				tot += n
				continue
			}
			fi := ds[i]
			if fi.Name() == afname {
				if i == len(ds)-1 {
					break
				}
				ds = append(ds[:i], ds[i+1:]...)
				fi = ds[i]
			}
			cpath := zx.Path(path, fi.Name())
			n := 0
			sum := ""
			if fi.IsDir() {
				n, sum = dirsz(cpath)
			}
			d := zx.NewDir(fi, n)
			d["path"] = zx.Path(rid, fi.Name())
			d["spath"] = d["path"]
			d["tpath"] = t.path
			if t.readattrs {
				if xd, ok := xds[fi.Name()]; ok {
					for k, v := range xd {
						if zx.IsUpper(k) {
							d[k] = v
						}
					}
				} else {
					d["Uid"] = dbg.Usr
					d["Gid"] = dbg.Usr
				}
			}
			if sum != "" {
				d["Sum"] = sum
			}
			cs.Send(0)
			n, err := d.Send(dc)
			if err != nil {
				return int64(tot), err
			}
			nd++
			i++
			tot += n
		}
		return int64(tot), err
	}
	if count == zx.All {
		cs.Sending()
		nm, n, err := nchan.ReadBytesFrom(fd, dc)
		cs.Sends(nm, n)
		return n, err
	}
	rr := io.LimitReader(fd, count)
	cs.Sending()
	nm, n, err := nchan.ReadBytesFrom(rr, dc)
	cs.Sends(nm, n)
	return n, err
}

var ErrNoMatch = errors.New("false")

func (t *Lfs) matchDir(d zx.Dir, fpred string) error {
	if fpred == "" {
		return nil
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
func (t *Lfs) match(rid string, fpred string) error {
	if fpred == "" {
		return nil
	}
	d, err := t.stat(rid)
	if err != nil {
		if dbg.IsNotExist(err) {
			d := zx.Dir{
				"path": rid,
				"name": path.Base(rid),
				"type": "-",
			}
			return t.matchDir(d, fpred)
		}
		return err
	}
	return t.matchDir(d, fpred)
}

func (t *Lfs) Get(rid string, off, count int64, pred string) <-chan []byte {
	t.dprintf("get %s %d %d %q\n", rid, off, count, pred)
	cs := t.IOstats.NewCall(zx.Sget)
	c := make(chan []byte)
	go func() {
		var n int64
		err := t.match(rid, pred)
		if err == nil {
			n, err = t.get(rid, off, count, c, cs)
		}
		cs.End(err != nil)
		if err != nil {
			t.dprintf("get %s: [%d] %s\n", rid, n, err)
		} else {
			t.dprintf("get %s: [%d]\n", rid, n)
		}
		close(c, err)
	}()
	return c
}

type dontflush  {
	io.Writer
}

func (f dontflush) DontFlush() {}

func (t *Lfs) put(rid string, d zx.Dir, off int64, dc <-chan []byte, pred string) (int64, int64, error) {
	noinherit := !t.readattrs
	if m := d["Mode"]; m != "" {
		d["mode"] = m
		delete(d, "Mode")
		noinherit = true
	}
	if t.rdonly {
		return 0, 0, fmt.Errorf("%s: %s", t.name, dbg.ErrRO)
	}
	if rid == "/" || path.Base(rid)==afname {
		return 0, 0, fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
	}
	prid := path.Dir(rid)
	pgid, pmode, perr := t.canWalkTo(prid, 0)
	if perr != nil {
		return 0, 0, perr
	}
	fd, err := t.stat(rid)
	if err != nil {
		if d["mode"] == "" {
			return 0, 0, fmt.Errorf("%s: %s", rid, dbg.ErrNotExist)
		}
		if !t.NoPermCheck && t.readattrs && t.ai != nil {
			if pd, err := t.stat(prid); err == nil && !pd.CanWrite(t.ai) {
				return 0, 0, fmt.Errorf("%s: %s", prid, dbg.ErrPerm)
			}
		}
	}
	if err == nil && fd["type"] == "d" || d["type"] == "d" {
		return 0, 0, fmt.Errorf("%s: %s", rid, dbg.ErrIsDir)
	}
	if err == nil && !t.NoPermCheck && t.readattrs && !fd.CanWrite(t.ai) {
		return 0, 0, fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
	}
	u := dbg.Usr
	if t.ai != nil {
		u = t.ai.Uid
	}
	ai := t.ai
	if t.NoPermCheck {
		ai = nil
	}
	if d == nil {
		d = zx.Dir{}
	}
	if d["mode"] == "" || err == nil {	// truncate or rewrite
		if err := t.matchDir(d, pred); err != nil {
			return 0, 0, err
		}
		if !t.WstatAll {
			if err := fd.CanWstat(ai, d); err != nil {
				return 0, 0, err
			}
		}
	} else {
		xd := zx.Dir{
			"path": rid,
			"name": path.Base(rid),
			"type": "-",
		}
		if err := t.matchDir(xd, pred); err != nil {
			return 0, 0, err
		}
	}
	flg := os.O_RDWR	// in case we resize it
	whence := 0
	if d["mode"] == "" {
		if off < 0 {
			off = 0
			whence  = 2
			flg |= os.O_APPEND
		}
		if !t.WstatAll || d["Wuid"] == "" {
			d["Wuid"] = u
		}
	} else {
		// truncate existing file or create it
		flg |= os.O_CREATE
		if d.Int("size") == 0 {
			d["size"] = "0"
		}
		if err != nil && t.readattrs && !t.WstatAll {
			nd := zx.Dir{"mode": d["mode"], "type": "-", "Uid": dbg.Usr, "Gid": pgid}
			if err := nd.CanWstat(ai, d); err != nil {
				return 0, 0, err
			}
		}
		if err != nil {		// created
			if d["Uid"] == "" {
				d["Uid"] = u
			}
			if d["Gid"] == "" {
				d["Gid"] = pgid
			}
		}
		if !noinherit && (!t.WstatAll || t.ai != nil) {
			d.Inherit(pmode)
		}
		if !t.WstatAll || d["Wuid"] == "" {
			d["Wuid"] = u
		}
	}
	if d["size"] != "" && d.Int("size") == 0 {
		flg |= os.O_TRUNC
	}

	var sz int64 = -1
	if d["size"] != "" {
		sz = d.Int64("size")
	}
	delete(d, "size")
	path := zx.Path(t.path, rid)
	mode := int64(0644)
	if d["mode"] != "" {
		mode = d.Int64("mode")
	} else if fd["mode"] != "" {
		mode = fd.Int64("mode")
	}
	// if the file existed, openfile won't update the mode.
	if err != nil {
		delete(d, "mode")
	}
	osfd, cerr := os.OpenFile(path, flg, os.FileMode(mode))
	if cerr != nil {
		return 0, 0, err
	}
	if sz != -1 {
		osfd.Truncate(sz)
	}
	osfd.Seek(off, whence)
	nm, n, err := nchan.WriteBytesTo(osfd, dc)
	osfd.Close()
	mt := d["mtime"]
	if mt != "" {
		delete(d, "mtime")
		ux, _ := strconv.ParseInt(mt, 0, 64)
		ft := time.Unix(ux/1e9, ux%1e9)
		os.Chtimes(path, ft, ft)
	}
	if len(d) > 0 && t.saveattrs {
		if DoSum {
			d["Sum"] = newFileSum(path)
		}
		t.wstat(rid, d)
	}
	return nm, n, err
}

func (t *Lfs) putCtl(datc <-chan []byte) (int, error) {
	if t.ai!=nil && !t.NoPermCheck {
		_, _, err := t.canWalkTo("/", 0444)
		if err != nil {
			return 0, err
		}
		if !ctldir.CanWrite(t.ai) {
			return 0, fmt.Errorf("/Ctl: %s", dbg.ErrPerm)
		}
	}
	ctl, err := nchan.String(datc)
	if err != nil {
		return 0, fmt.Errorf("/Ctl: %s", err)
	}
	if err := t.Ctl(ctl); err != nil {
		return 0, err
	}
	return len(ctl), nil
}

func (t *Lfs) Put(rid string, d zx.Dir, off int64, dc <-chan []byte, pred string) chan zx.Dir {
	d = d.Dup()
	t.dprintf("put %s %v %d '%s'\n", rid, d, off, pred)
	cs := t.IOstats.NewCall(zx.Sput)
	c := make(chan zx.Dir, 1)
	go func() {
		cs.Sending()
		rid, err := zx.AbsPath(rid)
		var nm, n int64
		var nd zx.Dir
		if err == nil && rid == "/Ctl" {
			nc, xerr := t.putCtl(dc)
			if xerr == nil {
				nd = zx.Dir{"size": "0"}
				nd.SetTime("mtime", time.Now())
				nm = 1
				n = int64(nc)
			}
			err = xerr
		} else if err == nil {
			nm, n, err = t.put(rid, d, off, dc, pred)
			cs.Sends(nm, n)
			if err == nil {
				nd, err = t.stat(rid)
			}
		}
		if err == nil {
			t.dprintf("put %s: %s\n", rid, d)
			xd := zx.Dir{"size": nd["size"], "mtime": nd["mtime"]}
			if DoSum && nd["Sum"] != "" {
				xd["Sum"] = nd["Sum"]
			}
			c <- xd
		} else {
			t.dprintf("put %s: %s\n", rid, err)
			close(dc, err)
		}
		close(dc, err)
		close(c, err)
		cs.End(err != nil)
	}()
	return c
}

func (t *Lfs) Mkdir(rid string, d zx.Dir) chan error {
	t.dprintf("mkdir %s %s \n", rid, d)
	cs := t.IOstats.NewCall(zx.Smkdir)
	c := make(chan error, 1)
	rid, err := zx.AbsPath(rid)
	if err == nil && t.rdonly {
		err = fmt.Errorf("%s: %s", t.name, dbg.ErrRO)
	}
	u := dbg.Usr
	if t.ai != nil {
		u = t.ai.Uid
	}
	gid := dbg.Usr
	pmode := uint64(0777)
	if err==nil && t.ai!=nil {
		prid := path.Dir(rid)
		gid, pmode, err = t.canWalkTo(prid, 0222)
	}
	if err == nil && (rid == "/" || rid == "/Ctl" || path.Base(rid)==afname) {
		err = fmt.Errorf("%s: %s", rid, dbg.ErrExists)
	}
	ud := d.UsrAttrs()
	delete(ud, "size")
	noinherit := !t.readattrs
	if m := ud["Mode"]; m != "" {
		ud["mode"] = m
		delete(ud, "Mode")
		noinherit = true
	}
	if ud["mode"] == "" {
		ud["mode"] = "0775"
	}
	if (!t.WstatAll || t.ai != nil) && !noinherit {
		ud["type"] = "d"
		ud.Inherit(pmode&0777)
		delete(ud, "type")
	}
	if ud["Uid"] == "" {
		ud["Uid"] = u
	}
	if ud["Gid"] == "" {
		ud["Gid"] = gid
	}
	ud["Wuid"] = d["Wuid"]
	if !t.WstatAll || t.ai != nil || ud["Wuid"] == "" {
		ud["Wuid"] = u
	}
	if err == nil && !t.WstatAll {
		ai := t.ai
		if t.NoPermCheck {
			ai = nil
		}
		nd := zx.Dir{
			"mode": ud["mode"],
			"type": "d",
			"Uid": u,
			"Gid": gid,
		}
		err = nd.CanWstat(ai, ud)
	}
	if err != nil {
		cs.End(true)
		t.dprintf("mkdir %s: %s\n", rid, err)
		c <- err
		close(c, err)
		return c
	}
	path := zx.Path(t.path, rid)
	mode := ud.Int64("mode")
	err = os.Mkdir(path, os.FileMode(mode))
	// BUG: we don't update wuid in the parent dir.
	delete(ud, "size")
	delete(ud, "mode")
	delete(ud, "Sum")
	if err==nil && len(ud)>0 {
		err = t.wstat(rid, ud)
	}
	cs.End(err != nil)
	if err == nil {
		t.dprintf("mkdir %s: ok\n", rid)
	} else {
		t.dprintf("mkdir %s: %s\n", rid, err)
	}
	c <- err
	close(c, err)
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

func (t *Lfs) Move(from, to string) chan error {
	t.dprintf("move %s %s \n", from, to)
	cs := t.IOstats.NewCall(zx.Smove)
	c := make(chan error, 1)
	from, err := zx.AbsPath(from)
	to, terr := zx.AbsPath(to)
	if terr != nil {
		err = terr
	}
	if err == nil && t.rdonly {
		err = fmt.Errorf("%s: %s", t.name, dbg.ErrRO)
	}
	if from == to {
		cs.End(true)
		t.dprintf("move %s: ok\n", from)
		c <- nil
		close(c)
		return c
	}
	if err == nil && inconsistent(from, to) {
		err = fmt.Errorf("%s: inconsistent move", from)
	}
	if err==nil && t.ai!=nil && !t.NoPermCheck {
		_, _, err = t.canWalkTo(from, 0222)
	}
	if err==nil && t.ai!=nil && !t.NoPermCheck {
		_, _, err = t.canWalkTo(path.Dir(to), 0222)
	}
	if err == nil && (from=="/Ctl" || path.Base(from)==afname) {
		err = fmt.Errorf("%s: %s", from, dbg.ErrPerm)
	}
	if err == nil && (to=="/Ctl" || path.Base(to)==afname) {
		err = fmt.Errorf("%s: %s", to, dbg.ErrPerm)
	}
	if err == nil && t.readattrs && !t.NoPermCheck {
		pd := path.Dir(from)
		fd, ferr := t.stat(pd)
		if err = ferr; err == nil && !fd.CanWrite(t.ai) {
			err = fmt.Errorf("%s: %s", pd, dbg.ErrPerm)
		}
	}
	if err == nil && t.readattrs && !t.NoPermCheck {
		pd := path.Dir(to)
		fd, ferr := t.stat(pd)
		if err = ferr; err == nil && !fd.CanWrite(t.ai) {
			err = fmt.Errorf("%s: %s", pd, dbg.ErrPerm)
		}
	}
	if err != nil {
		cs.End(true)
		t.dprintf("move %s: %s\n", from, err)
		c <- err
		close(c, err)
		return c
	}
	pfrom := zx.Path(t.path, from)
	pto := zx.Path(t.path, to)
	var d zx.Dir
	if t.readattrs {
		d = zx.Dir{"name": path.Base(from)}
		t.fileAttrs(pfrom, d)
		if len(d) > 1 && t.saveattrs {
			// clear all attributes for the old file
			t.writeFileAttrs(pfrom, zx.Dir{"name": path.Base(from)})
		}
	}
	err = os.Rename(pfrom, pto)
	cs.End(err != nil)
	if err == nil {
		if d != nil && t.saveattrs {
			t.writeFileAttrs(pto, d)
		}
		t.dprintf("move %s: ok\n", from)
	} else {
		t.dprintf("move %s: %s\n", from, err)
	}
	c <- err
	close(c, err)
	return c
}

func (t *Lfs) remove(rid string, all bool, cs *zx.CallStat) chan error {
	c := make(chan error, 1)
	rid, err := zx.AbsPath(rid)
	if err == nil && t.rdonly {
		err = fmt.Errorf("%s: %s", t.name, dbg.ErrRO)
	}
	if err == nil && (rid == "/" || rid == "/Ctl" || path.Base(rid)==afname) {
		err = fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
	}
	if err == nil && t.ai != nil && !t.NoPermCheck {
		_, _, err = t.canWalkTo(rid, 0222)
	}
	if err != nil {
		cs.End(true)
		t.dprintf("remove %s: %s\n", rid, err)
		c <- err
		close(c, err)
		return c
	}
	xpath := zx.Path(t.path, rid)
	if all {
		err = os.RemoveAll(xpath)
	} else {
		// Remove, getting rid of ".#zx" files lazily if a remove
		// of a non-empty dir fails because of that.
		err = os.Remove(xpath)
		if err!=nil && strings.Contains(err.Error(), "not empty") {
			afpath := path.Join(xpath, afname)
			os.Remove(afpath)
			err = os.Remove(xpath)
		}
	}
	cs.End(err != nil)
	if err == nil {
		t.dprintf("remove %s: ok\n", rid)
	} else {
		t.dprintf("remove %s: %s\n", rid, err)
	}
	c <- err
	close(c, err)
	return c
}

func (t *Lfs) Remove(rid string) chan error {
	t.dprintf("remove %s\n", rid)
	cs := t.IOstats.NewCall(zx.Sremove)
	return t.remove(rid, false, cs)
}

func (t *Lfs) RemoveAll(rid string) chan error {
	t.dprintf("removeall %s\n", rid)
	cs := t.IOstats.NewCall(zx.Sremoveall)
	return t.remove(rid, true, cs)
}

func orerr(e1, e2 error) error {
	if e1 != nil {
		return e1
	}
	return e2
}

// This one DOES write Wuid and Sum if given
func (t *Lfs) wstat(rid string, d zx.Dir) error {
	if len(d) == 0 {
		return nil
	}
	path := zx.Path(t.path, rid)
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		d["type"] = "-"
		if _, ok := d["size"]; ok {
			u := dbg.Usr
			if t.ai != nil {
				u = t.ai.Uid
			}
			nsz := d.Int64("size")
			nerr := os.Truncate(path, nsz)
			err = orerr(err, nerr)
			if nerr == nil && t.saveattrs {
				if !t.WstatAll || d["Wuid"] == "" {
					d["Wuid"] = u
				}
				if DoSum {
					if nsz == 0 {
						d["Sum"] = zx.Zsum()
					} else {
						d["Sum"] = newFileSum(path)
					}
				}
			}
		} else {
			d["size"] = strconv.FormatInt(fi.Size(), 10)
		}
	} else {
		d["type"] = "d"
	}
	if _, ok := d["mode"]; ok {
		mode := d.Int("mode")&0777
		nerr := os.Chmod(path, os.FileMode(mode))
		err = orerr(err, nerr)
	}
	if _, ok := d["mtime"]; ok {
		mt := d.Time("mtime")
		nerr := os.Chtimes(path, mt, mt)
		err = orerr(err, nerr)
	} else {
		d.SetTime("mtime", fi.ModTime())
	}
	if t.saveattrs {
		for k := range d {
			if zx.IsUpper(k) {
				err = orerr(err, t.writeFileAttrs(path, d))
				return err
			}
		}
	}
	return err
}

func (t *Lfs) Wstat(rid string, d zx.Dir) chan error {
	t.dprintf("wstat %s %v\n", rid, d)
	cs := t.IOstats.NewCall(zx.Swstat)
	c := make(chan error, 1)
	if rid == "/Ctl" {
		cs.End(false)
		t.dprintf("wstat %s: ok\n", rid)
		close(c)
		return c
	}
	var old zx.Dir
	rid, err := zx.AbsPath(rid)
	if err == nil && t.rdonly {
		err = fmt.Errorf("%s: %s", t.name, dbg.ErrRO)
	}
	if err == nil && !t.NoPermCheck {
		_, _, err = t.canWalkTo(rid, 0)
	}
	if err == nil && (rid == "/Ctl" || path.Base(rid)==afname) {
		err = fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
	}
	if err == nil {	
		old, err = t.stat(rid)
	}
	if err == nil && !t.WstatAll {
		ai := t.ai
		if t.NoPermCheck {
			ai = nil
		}
		err = old.CanWstat(ai, d)
	}
	if err == nil {
		ud := d.UsrAttrs()
		err = t.wstat(rid, ud)
	}
	cs.End(err != nil)
	if err != nil {
		t.dprintf("wstat %s: %s\n", rid, err)
	} else {
		t.dprintf("wstat %s: ok\n", rid)
	}
	c <- err
	close(c, err)
	return c
}

func (t *Lfs) GetDir(rid string) ([]zx.Dir, error) {
	t.dprintf("getdir %s\n", rid)
	rid, err := zx.AbsPath(rid)
	if err != nil {
		t.dprintf("getdir %s: %s\n", rid, err)
		return nil, err
	}
	path := zx.Path(t.path, rid)
	fi, err := os.Stat(path)
	if err != nil {
		t.dprintf("getdir %s: %s\n", rid, err)
		return nil, err
	}
	if !fi.IsDir() {
		t.dprintf("getdir %s: [%d]\n", rid, 0)
		return []zx.Dir{}, nil
	}
	fis, err := ioutil.ReadDir(path)
	if err != nil {
		t.dprintf("getdir %s: %s\n", rid, err)
		return nil, err
	}
	var xds map[string]zx.Dir
	if t.readattrs {
		xds, _ = t.dirAttrs(path)
	}
	ds := []zx.Dir{}
	if rid == "/" {
		d := ctldir.Dup()
		d["tpath"] = t.path
		ds = append(ds, d)
	}
	for _, fi := range fis {
		if fi.Name() == afname {
			continue
		}
		cpath := zx.Path(path, fi.Name())
		n := 0
		sum := ""
		if fi.IsDir() {
			n, sum = dirsz(cpath)
		}
		d := zx.NewDir(fi, n)
		d["path"] = zx.Path(rid, fi.Name())
		d["spath"] = zx.Path(rid, fi.Name())
		d["tpath"] = t.path
		d["proto"] = "lfs"
		if t.readattrs {
			if xd, ok := xds[fi.Name()]; ok {
				for k, v := range xd {
					if zx.IsUpper(k) {
						d[k] = v
					}
				}
			}
		}
		if sum != "" && DoSum {
			d["Sum"] = sum
		}
		ds = append(ds, d)
	}
	t.dprintf("getdir %s: [%d]\n", rid, len(ds))
	return ds, nil
}

func (t *Lfs) Fsys(name string) <-chan error {
	t.dprintf("fsys %s\n", name)
	c := make(chan error, 1)
	if name!="" && name!="main" {
		err := errors.New("fsys not supported for local trees")
		t.dprintf("fsys %s: %s\n", err)
		c <- err
		close(c, err)
	} else {
		t.dprintf("fsys %s: ok\n", name)
		close(c)
	}
	return c
}

// d is a dup and can be changed.
func (t *Lfs) find(d zx.Dir, p *pred.Pred, spref, dpref string, lvl int, c chan<- zx.Dir) {
	rid := d["spath"]
	match, pruned, err := p.EvalAt(d, lvl)
	t.dprintf("find at %v\n\t%v\n\t%v %v %v\n\n", d, p, match, pruned, err)
	if pruned {
		if match {
			d["proto"] = "lfs"
		} else {
			d["err"] = "pruned"
		}
		c <- d
		return
	}
	if err != nil {
		close(c, err)
		return
	}
	if d["rm"] != "" {
		return
	}
	var ds []zx.Dir
	if t.ai!=nil && !d.CanRead(t.ai) {
		err = dbg.ErrPerm
	}
	if err==nil && d["type"]=="d" {
		if t.ai!=nil && !d.CanWalk(t.ai) {
			err = dbg.ErrPerm
		} else {
			ds, err = t.GetDir(rid)
		}
	}
	if err != nil {
		d["err"] = err.Error()
		c <- d
		return
	}
	if match {
		d["proto"] = "lfs"
		if ok := c <- d; !ok {
			return
		}
	}
	for i := 0; i < len(ds); i++ {
		cd := ds[i]
		if cd["rm"] != "" {
			continue
		}
		if spref != dpref {
			cpath := cd["path"]
			suff := zx.Suffix(cpath, spref)
			cd["path"] = zx.Path(dpref, suff)
		}
		t.find(cd, p, spref, dpref, lvl+1, c)
	}
}

func (t *Lfs) Find(rid, fpred, spref, dpref string, depth int) <-chan zx.Dir {
	t.dprintf("find %s '%s' '%s' '%s' %d\n", rid, fpred, spref, dpref, depth)
	cs := t.IOstats.NewCall(zx.Sfind)
	dc := make(chan zx.Dir)
	go func() {
		rid, err := zx.AbsPath(rid)
		if err == nil && !t.NoPermCheck {
			_, _, err = t.canWalkTo(rid, 0)
		}
		if err != nil {
			cs.End(err != nil)
			t.dprintf("find %s: %s\n", rid, err)
			close(dc, err)
			return
		}
		d, err := t.stat(rid)
		if err != nil {
			cs.End(err != nil)
			t.dprintf("find %s: %s\n", rid, err)
			close(dc, err)
			return
		}
		p, err := pred.New(fpred)
		if err != nil {
			cs.End(err != nil)
			t.dprintf("find %s: %s\n", rid, err)
			close(dc, err)
			return
		}
		cs.Sending()
		if spref != dpref {
			suff := zx.Suffix(rid, spref)
			d["path"] = zx.Path(dpref, suff)
		}
		t.find(d, p, spref, dpref, depth, dc)
		cs.End(err != nil)
		t.dprintf("find %s: ok\n", rid)
		close(dc)
	}()
	return dc
}

func (t *Lfs) FindGet(rid, fpred, spref, dpref string, depth int) <-chan zx.DirData {
	t.dprintf("findget %s '%s' '%s' '%s' %d\n", rid, fpred, spref, dpref, depth)
	gc := make(chan zx.DirData)
	cs := t.IOstats.NewCall(zx.Sfindget)
	go func() {
		dc := t.Find(rid, fpred, spref, dpref, depth) // BUG: will stat a Sfind
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
				_, err := t.get(d["spath"], 0, zx.All, datac, nil)
				close(datac, err)
			}
		}
		err := cerror(dc)
		cs.End(err != nil)
		if err != nil {
			t.dprintf("find %s: %s\n", rid, err)
		} else {
			t.dprintf("find %s: ok\n", rid)
		}
		close(gc, err)
	}()
	return gc
}

// debug
func (t *Lfs) Dump(w io.Writer) {
	if t == nil {
		fmt.Fprintf(w, "<nil tree>\n")
		return
	}
	old := t.Dbg
	defer func() {
		t.Dbg = old
	}()
	fmt.Fprintf(w, "tree [%s] path %s\n", t.name, t.path)
	d, err := zx.Stat(t, "/")
	if err != nil {
		fmt.Fprintf(w, "/: %s\n", err)
		return
	}
	t.dump(w, d, 0)
	fmt.Fprintf(w, "\n")
}

func (t *Lfs) dump(w io.Writer, d zx.Dir, lvl int) {
	tabs := strings.Repeat("    ", lvl)
	if d == nil {
		fmt.Fprintf(w, "%s<nil file>\n", tabs)
		return
	}
	if d["path"] == "/" {
		d["size"] = "0"
	}
	fmt.Fprintf(w, "%s%s\n", tabs, d.TestFmt())
	if d["type"] != "d"  {
		fi, err := os.Stat(zx.Path(t.path, d["path"]))
		if err == nil {
			fmt.Fprintf(w, "%s  %d bytes\n", tabs, fi.Size())
		} else {
			fmt.Fprintf(w, "%s: %s\n", d["path"], err)
		}
		return
	}
	ds, err := t.GetDir(d["path"])
	if err != nil {
		fmt.Fprintf(w, "%s: %s\n", d["path"], err)
		return
	}
	for _, cd := range ds {
		if cd["path"] != "/Ctl" {
			t.dump(w, cd, lvl+1)
		}
	}
}
