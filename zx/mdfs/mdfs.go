/*
	on-memory metadata, on-disk data fs.

	It's main usage is as a cache for remote zx trees.

	It's exactly as mfs but does not keep the data in memory.
	Instead, it writes-through and reads-behind of an underlying tree.

	See the new cfs.
*/
package mdfs

import (
	"bytes"
	"clive/dbg"
	"clive/nchan"
	"clive/net/auth"
	"clive/zx"
	"clive/zx/pred"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// REFERENCE: clive/zx(x): for zx interfaces and basic data types.

/*
	A ram Dir, disk data, file system
*/
type Fs struct {
	name string
	path string // fake tpath
	root *mFile
	ai   *auth.Info
	lfs  zx.RWTree
	*zx.Flags

	// keep it quiescent during moves, or there will be races
	mvlk sync.RWMutex
}

type mFile struct {
	name  string
	d     zx.Dir
	child []*mFile
	// We can't keep a ref to the tree here, because t.ai might
	// change for different clients.

	// lock order is mvlk, then meta and then children or lfs data
	mlk sync.Mutex // for metadata
	clk sync.Mutex // for children
}

var (
	trees    = map[string]*Fs{}
	tpathgen int
	treeslk  sync.RWMutex

	// make sure we implement the right interfaces
	_fs  *Fs
	_t   zx.RWTree   = _fs
	_r   zx.Recver   = _fs
	_snd zx.Sender   = _fs
	_g   zx.Getter   = _fs
	_w   zx.Walker   = _fs
	_s   zx.Stater   = _fs
	_a   zx.AuthTree = _fs
	_d   zx.Dumper   = _fs
	_D   zx.Debugger = _fs
)

func (t *Fs) String() string {
	return t.name
}

func init() {
	zx.DefProto("mfs", dial)
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
	return nil, errors.New("no such tree")
}

func (t *Fs) DirFile(df zx.Dir) zx.File {
	return zx.File{t, df}
}

func (t *Fs) Name() string {
	return t.name
}

func New(name string, lfs zx.RWTree) (*Fs, error) {
	treeslk.Lock()
	p := fmt.Sprintf("%d", tpathgen)
	tpathgen++
	treeslk.Unlock()
	r := &mFile{
		name: "/",
		d: zx.Dir{
			"name":  "/",
			"path":  "/",
			"spath": "/",
			"proto": "mfs",
			"tpath": p,
			"type":  "d",
			"mode":  "0755",
			"Uid":   dbg.Usr,
			"Gid":   dbg.Usr,
			"Wuid":  dbg.Usr,
			"Sum":   zx.Zsum(),
			"size":  "0",
		},
		child: []*mFile{},
	}
	r.d.SetTime("mtime", time.Now())
	l := &Fs{
		name:  name,
		path:  p,
		root:  r,
		lfs:   lfs,
		Flags: &zx.Flags{},
	}
	l.Flags.Add("debug", &l.Dbg)
	l.Flags.AddRO("noperm", &l.NoPermCheck)
	l.Flags.Add("clear", func(...string) error {
		l.IOstats.Clear()
		return nil
	})
	l.reload()
	treeslk.RLock()
	trees[p] = l
	treeslk.RUnlock()
	return l, nil
}

func (t *Fs) Close(e error) {
	t.dprintf("close sts %v\n", e)
	treeslk.RLock()
	delete(trees, t.path)
	treeslk.RUnlock()
}

// Ask the tree to perform auth checks on behalf of ai.
func (t *Fs) AuthFor(ai *auth.Info) (zx.Tree, error) {
	if ai != nil {
		t.dprintf("auth for %s %v\n", ai.Uid, ai.Gids)
	}
	nfs := &Fs{}
	*nfs = *t
	if nlfs, ok := t.lfs.(zx.AuthTree); ok {
		if x, err := nlfs.AuthFor(ai); err == nil {
			nfs.lfs = x.(zx.RWTree)
		}
	}
	nfs.ai = ai
	return nfs, nil
}

func (t *Fs) Stats() *zx.IOstats {
	return t.IOstats
}

func (f *mFile) String() string {
	return f.d["path"]
}

func (t *Fs) dprintf(fs string, args ...interface{}) {
	if t != nil && t.Dbg {
		fmt.Fprintf(os.Stderr, t.name+":"+fs, args...)
	}
}

func (t *Fs) dfprintf(f *mFile, fs string, args ...interface{}) {
	if f != nil && t != nil && t.Dbg {
		t.dprintf(f.d["path"]+":"+fs, args...)
	}
}

// no locks, used only at init time
func (t *Fs) reload() {
	d, err := zx.Stat(t.lfs, "/")
	if err != nil {
		dbg.Warn("%s: reload: %s", t.name, err)
		return
	}
	ud := d.UsrAttrs()
	ud["Sum"] = d["Sum"]
	for k, v := range ud {
		t.root.d[k] = v
	}
	t.reloadChild(t.root)
}

// no locks, used only at init time
func (t *Fs) reloadChild(f *mFile) {
	ds, err := zx.GetDir(t.lfs, f.d["path"])
	if err != nil {
		dbg.Warn("%s: reload: %s", t.name, err)
		return
	}
	for _, d := range ds {
		if d["path"] == "/Ctl" || d["path"] == "/Chg" {
			continue
		}
		cf := &mFile{
			name: d["name"],
			d: zx.Dir{
				"name":  d["name"],
				"path":  d["path"],
				"spath": d["path"],
				"tpath": t.path,
				"type":  d["type"],
				"Wuid":  d["Wuid"],
				"Sum":   d["Sum"],
			},
		}
		for k, v := range d.UsrAttrs() {
			cf.d[k] = v
		}
		f.child = append(f.child, cf)
		if cf.d["type"] == "d" {
			t.reloadChild(cf)
		}
	}
}

func (f *mFile) walk1(el string) (*mFile, error) {
	f.mlk.Lock()
	if f.d["type"] != "d" {
		defer f.mlk.Unlock()
		return nil, fmt.Errorf("%s: not a directory", f)
	}
	f.mlk.Unlock()
	f.clk.Lock()
	for _, c := range f.child {
		if c.name == el {
			f.clk.Unlock()
			return c, nil
		}
	}
	f.clk.Unlock()
	f.mlk.Lock()
	defer f.mlk.Unlock()
	return nil, fmt.Errorf("%s: '%s': file not found", f, el)
}

// if the error is a file not found, then file is returned with
// the parent file and left contains what's left to walk.
func (t *Fs) walk(rid string, pfp **mFile) (cf *mFile, left []string, e error) {
	els := zx.Elems(rid)
	f := t.root
	for len(els) > 0 {
		f.mlk.Lock()
		if !t.NoPermCheck && !f.d.CanWalk(t.ai) {
			defer f.mlk.Unlock()
			return f, els, fmt.Errorf("%s: %s", f, dbg.ErrPerm)
		}
		f.mlk.Unlock()
		if len(els) == 1 && pfp != nil {
			*pfp = f
		}
		nf, err := f.walk1(els[0])
		if err != nil {
			return f, els, err
		}
		f = nf
		els = els[1:]
	}
	return f, nil, nil
}

var ctldir = zx.Dir{
	"path":  "/Ctl",
	"spath": "/Ctl",
	"name":  "Ctl",
	"proto": "mfs",
	"size":  "0",
	"type":  "c",
	"Uid":   dbg.Usr,
	"Gid":   dbg.Usr,
	"Wuid":  dbg.Usr,
	"mode":  "0664",
	"Sum":   zx.Zsum(),
}

func (t *Fs) stat(rid string) (zx.Dir, error) {
	rid, err := zx.AbsPath(rid)
	if err != nil {
		return nil, err
	}
	if rid == "/Ctl" {
		cd := ctldir.Dup()
		cd["tpath"] = t.path
		return cd, nil
	}
	f, _, err := t.walk(rid, nil)
	if err != nil {
		return nil, err
	}
	f.mlk.Lock()
	defer f.mlk.Unlock()
	return f.d.Dup(), nil
}

func (t *Fs) Stat(rid string) chan zx.Dir {
	t.mvlk.RLock()
	defer t.mvlk.RUnlock()
	t.dprintf("stat %s\n", rid)
	cs := t.IOstats.NewCall(zx.Sstat)
	c := make(chan zx.Dir, 1)
	d, err := t.stat(rid)
	if err != nil {
		t.dprintf("stat %s: %s\n", rid, err)
		close(c, err)
		cs.End(true)
		return c
	}
	t.dprintf("stat %s: %s\n", rid, d)
	c <- d
	close(c)
	cs.End(false)
	return c
}

// locks/unlocks f's data
func (f *mFile) getDir(off, count int64, dc chan<- []byte, cs *zx.CallStat) error {
	f.clk.Lock()
	ds := make([]*mFile, len(f.child))
	copy(ds, f.child)
	f.clk.Unlock()
	nd := 0
	ctlsent := false
Dloop:
	for i := 0; i < len(ds); {
		if off > 0 {
			off--
			if !ctlsent && f.name == "/" {
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
		if !ctlsent && f.name == "/" {
			ctlsent = true
			nd++
			d := ctldir.Dup()
			d["tpath"] = f.d["tpath"]
			d.Send(dc)
			nd++ // but not i++
			continue
		}
		ds[i].mlk.Lock()
		d := ds[i].d.Dup()
		ds[i].mlk.Unlock()
		cs.Send(0)
		if _, err := d.Send(dc); err != nil {
			return err
		}
		nd++
		i++
	}
	return nil
}

func (t *Fs) getCtl(off, count int64, dc chan<- []byte, cs *zx.CallStat) error {
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
	if n > len(resp) || n < 0 {
		n = len(resp)
	}
	resp = resp[:n]
	cs.Send(int64(len(resp)))
	dc <- resp
	return nil
}

func (t *Fs) get(rid string, off, count int64, dc chan<- []byte, cs *zx.CallStat) error {
	rid, err := zx.AbsPath(rid)
	if err != nil {
		return err
	}
	if rid == "/Ctl" {
		return t.getCtl(off, count, dc, cs)
	}

	f, _, err := t.walk(rid, nil)
	if err == nil && !t.NoPermCheck {
		f.mlk.Lock()
		if !f.d.CanRead(t.ai) {
			err = dbg.ErrPerm
		}
		f.mlk.Unlock()
	}
	if err != nil {
		return err
	}
	f.mlk.Lock()
	if f.d["type"] == "d" {
		f.mlk.Unlock()
		return f.getDir(off, count, dc, cs)
	}
	f.mlk.Unlock()
	lc := t.lfs.Get(rid, off, count, "")
	for d := range lc {
		if ok := dc <- d; !ok {
			close(lc, cerror(dc))
		}
	}
	return cerror(lc)
}

var ErrNoMatch = errors.New("false")

func (t *Fs) matchDir(rid string, d zx.Dir, fpred string) error {
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

func (t *Fs) match(rid string, fpred string) error {
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
			return t.matchDir(rid, d, fpred)
		}
		return err
	}
	return t.matchDir(rid, d, fpred)
}

func (t *Fs) Get(rid string, off, count int64, pred string) <-chan []byte {
	t.dprintf("get %s %d %d %q\n", rid, off, count, pred)
	cs := t.IOstats.NewCall(zx.Sget)
	c := make(chan []byte)
	go func() {
		t.mvlk.RLock()
		defer t.mvlk.RUnlock()
		err := t.match(rid, pred)
		if err == nil {
			err = t.get(rid, off, count, c, cs)
		}
		cs.End(err != nil)
		if err != nil {
			t.dprintf("get %s: %s\n", rid, err)
		} else {
			t.dprintf("get %s: ok\n", rid)
		}
		close(c, err)
	}()
	return c
}

func (f *mFile) newDirSum() {
	h := sha1.New()
	for _, c := range f.child {
		fmt.Fprintf(h, "%s\n", c.name)
	}
	sum := h.Sum(nil)
	f.d["Sum"] = fmt.Sprintf("%040x", sum)
}

// f meta and child locked by caller
func (f *mFile) attach(cf *mFile, ai *auth.Info) error {
	u := dbg.Usr
	if ai != nil {
		u = ai.Uid
	}
	for i, c := range f.child {
		if c.name == cf.name {
			f.child[i] = cf
			f.d.SetTime("mtime", time.Now())
			f.d["Wuid"] = u
			return nil
		}
	}
	f.child = append(f.child, cf)
	sort.Sort(byName(f.child))
	n := len(f.child)
	if f.d["path"] == "/" {
		n++ // for Ctl
	}
	f.d["size"] = strconv.Itoa(n)
	f.d.SetTime("mtime", time.Now())
	f.d["Wuid"] = u
	f.newDirSum()
	return nil
}

type byName []*mFile

func (b byName) Len() int {
	return len(b)
}

func (b byName) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byName) Less(i, j int) bool {
	return b[i].d["name"] < b[j].d["name"]
}

func (t *Fs) put(rid string, d zx.Dir, off int64, dc <-chan []byte, pred string) (zx.Dir, error) {
	noinherit := false
	if m := d["Mode"]; m != "" {
		d["mode"] = m
		delete(d, "Mode")
		noinherit = true
	}
	var pf *mFile
	f, left, err := t.walk(rid, &pf)
	if err != nil && !dbg.IsNotExist(err) || len(left) > 1 || d["mode"] == "" && err != nil {
		return nil, err
	}
	pmode := uint64(0755)
	if pf != nil {
		pf.mlk.Lock()
		pmode = pf.d.Mode()
		pf.mlk.Unlock()
	}
	f.mlk.Lock()
	if err == nil && f.d["type"] == "d" || d["type"] == "d" {
		err = fmt.Errorf("%s: %s", f, dbg.ErrIsDir)
		f.mlk.Unlock()
		return nil, err
	}
	if !t.NoPermCheck && !f.d.CanWrite(t.ai) {
		err = fmt.Errorf("%s: %s", f, dbg.ErrPerm)
		f.mlk.Unlock()
		return nil, err
	}
	u := dbg.Usr
	if t.ai != nil {
		u = t.ai.Uid
	}
	if d == nil {
		d = zx.Dir{}
	}
	ai := t.ai
	if t.NoPermCheck {
		ai = nil
	}
	if d["mode"] == "" || err == nil { // truncate or rewrite
		if err := t.matchDir(rid, f.d, pred); err != nil {
			f.mlk.Unlock()
			return nil, err
		}
		if !t.WstatAll || t.ai != nil {
			if err := f.d.CanWstat(ai, d); err != nil {
				f.mlk.Unlock()
				return nil, err
			}
		}
	} else {
		if err := t.matchDir(rid, nil, pred); err != nil {
			f.mlk.Unlock()
			return nil, err
		}
	}
	if err == nil {
		// truncate existing file
		if d["mode"] != "" {
			f.d["mode"] = d["mode"]
			if !noinherit && (!t.WstatAll || t.ai != nil) {
				f.d.Inherit(pmode)
				d["mode"] = f.d["mode"]
			}
		}
		f.d["size"] = d["size"]
		if t.WstatAll && t.ai == nil && d["Wuid"] != "" {
			f.d["Wuid"] = d["Wuid"]
		} else {
			f.d["Wuid"] = u
		}
		if err := f.wstat(t, d); err != nil {
			f.mlk.Unlock()
			return nil, err
		}
	} else {
		// create a new file
		nf := &mFile{
			name: left[0],
			d: zx.Dir{
				"mode":  d["mode"],
				"name":  left[0],
				"path":  rid,
				"spath": rid,
				"tpath": f.d["tpath"],
				"Uid":   u,
				"Gid":   f.d["Gid"],
				"Wuid":  u,
				"type":  "-",
				"size":  "0",
			},
		}
		if !t.WstatAll || t.ai != nil {
			if !noinherit {
				nf.d.Inherit(pmode)
				d["mode"] = nf.d["mode"]
			}
			if err := nf.d.CanWstat(ai, d); err != nil {
				f.mlk.Unlock()
				return nil, err
			}
		}
		if t.WstatAll && t.ai == nil && d["Wuid"] != "" {
			nf.d["Wuid"] = d["Wuid"]
		}
		if d["Uid"] != "" {
			nf.d["Uid"] = d["Uid"]
		}
		if d["Gid"] != "" {
			nf.d["Gid"] = d["Gid"]
		}
		if d["size"] != "" {
			nf.d["size"] = d["size"]
		}
		f.clk.Lock()
		f.attach(nf, t.ai)
		f.clk.Unlock()
		f.mlk.Unlock()
		f = nf
		f.mlk.Lock()
	}
	f.mlk.Unlock()

	rdc := t.lfs.Put(rid, d, off, dc, "")
	rd := <-rdc
	rerr := cerror(rdc)

	f.mlk.Lock()
	defer f.mlk.Unlock()
	if err != nil {
		// the file was created, update our stat
		ud := d.UsrAttrs()
		delete(ud, "size")
		for k, v := range ud {
			if v == "" {
				delete(f.d, k)
			} else {
				f.d[k] = v
			}
		}
	}
	f.d.SetTime("mtime", time.Now())
	if t.WstatAll && t.ai == nil && d["Wuid"] != "" {
		f.d["Wuid"] = d["Wuid"]
	} else {
		f.d["Wuid"] = u
	}
	for k, v := range rd {
		f.d[k] = v
	}
	return f.d, rerr
}

func (t *Fs) putCtl(datc <-chan []byte) (int, error) {
	// no lock on root.d (race)
	if !t.NoPermCheck && (!t.root.d.CanWalk(t.ai) || !ctldir.CanWrite(t.ai)) {
		return 0, fmt.Errorf("/Ctl: %s", dbg.ErrPerm)
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

func (t *Fs) Put(rid string, d zx.Dir, off int64, dc <-chan []byte, pred string) chan zx.Dir {
	d = d.Dup()
	t.dprintf("put %s %v %d '%s'\n", rid, d, off, pred)
	cs := t.IOstats.NewCall(zx.Sput)
	c := make(chan zx.Dir, 1)
	go func() {
		t.mvlk.RLock()
		defer t.mvlk.RUnlock()
		cs.Sending()
		rid, err := zx.AbsPath(rid)
		var nd zx.Dir
		if err == nil && rid == "/Ctl" {
			_, xerr := t.putCtl(dc)
			if xerr == nil {
				nd = zx.Dir{"size": "0", "Sum": zx.Zsum()}
				nd.SetTime("mtime", time.Now())
			}
			err = xerr
		} else if err == nil {
			nd, err = t.put(rid, d, off, dc, pred)
		}
		if err == nil {
			rd := zx.Dir{"size": nd["size"], "mtime": nd["mtime"], "Sum": nd["Sum"]}
			t.dprintf("put %s: %s\n", rid, rd)
			c <- rd
		} else {
			t.dprintf("put %s: %s\n", rid, err)
			close(dc, err)
		}
		close(c, err)
		cs.End(err != nil)
	}()
	return c
}

// NB: Attributes that the user can't set are always ignored.
// If the user has no permissioin to set an attribute, that's an error.
// Setting an attribute to an empty string removes it.
// Uid, Gid, and Wuid can't be removed.
// Meta locking done by caller, might lock data on truncations
func (f *mFile) wstat(t *Fs, d zx.Dir) error {
	if len(d) == 0 {
		return nil
	}
	if err := <-t.lfs.Wstat(f.d["path"], d); err != nil {
		return err
	}
	for k, v := range d.UsrAttrs() {
		if v == "" {
			delete(f.d, k)
		}
	}
	nd, err := zx.Stat(t.lfs, f.d["path"])
	if err != nil {
		return nil
	}
	if nd["Sum"] != "" {
		f.d["Sum"] = nd["Sum"]
	}
	if nd["Wuid"] != "" {
		f.d["Wuid"] = nd["Wuid"]
	}
	for k, v := range nd.UsrAttrs() {
		f.d[k] = v
	}
	return nil
}

func (t *Fs) wstat(rid string, d zx.Dir) error {
	f, _, err := t.walk(rid, nil)
	if err != nil {
		return err
	}
	f.mlk.Lock()
	defer f.mlk.Unlock()
	ai := t.ai
	if t.NoPermCheck {
		ai = nil
	}
	ud := d.UsrAttrs()
	if !t.WstatAll || t.ai != nil {
		if err := f.d.CanWstat(ai, d); err != nil {
			return err
		}
	} else {
		if d["Wuid"] != "" {
			ud["Wuid"] = d["Wuid"]
		}
	}
	return f.wstat(t, ud)
}

func (t *Fs) Mkdir(rid string, d zx.Dir) chan error {
	t.mvlk.RLock()
	defer t.mvlk.RUnlock()
	t.dprintf("mkdir %s %v\n", rid, d)
	cs := t.IOstats.NewCall(zx.Smkdir)
	c := make(chan error, 1)
	rid, err := zx.AbsPath(rid)
	if rid == "/Ctl" {
		err = dbg.ErrExists
	}
	if err != nil {
		cs.End(true)
		t.dprintf("mkdir %s: %s\n", rid, err)
		c <- err
		close(c, err)
		return c
	}
	f, left, err := t.walk(rid, nil)
	if err == nil {
		err = fmt.Errorf("'%s': %s", rid, dbg.ErrExists)
	} else if len(left) == 1 {
		err = nil
	}
	f.mlk.Lock()
	if err == nil && !t.NoPermCheck && !f.d.CanWrite(t.ai) {
		err = fmt.Errorf("%s: %s", f, dbg.ErrPerm)
	}
	if err != nil {
		f.mlk.Unlock()
		cs.End(true)
		t.dprintf("mkdir %s: %s\n", rid, err)
		c <- err
		close(c, err)
		return c
	}
	ud := d.UsrAttrs()
	delete(ud, "size")
	noinherit := false
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
		ud.Inherit(f.d.Mode())
		delete(ud, "type")
	}
	f.mlk.Unlock()
	u := dbg.Usr
	if t.ai != nil {
		u = t.ai.Uid
	}
	nf := &mFile{
		name: left[0],
		d: zx.Dir{
			"mode":  ud["mode"],
			"name":  left[0],
			"path":  rid,
			"spath": rid,
			"tpath": t.path,
			"Uid":   u,
			"Gid":   f.d["Gid"],
			"Wuid":  u,
			"type":  "d",
			"size":  "0",
			"Sum":   zx.Zsum(),
		},
		child: []*mFile{},
	}
	nmode := nf.d["mode"]
	ai := t.ai
	if t.NoPermCheck {
		ai = nil
	}
	if err == nil && (!t.WstatAll || t.ai != nil) {
		err = nf.d.CanWstat(ai, ud)
	}
	if err == nil {
		f.mlk.Lock()
		f.clk.Lock()
		err = <-t.lfs.Mkdir(rid, d)
		if err != nil {
			t.dprintf("mkdir %s: %s\n", rid, err)
			// and try again with just the mode
			err = <-t.lfs.Mkdir(rid, zx.Dir{"mode": nmode})
		}
		if err == nil {
			f.attach(nf, t.ai)
		}
		f.clk.Unlock()
		f.mlk.Unlock()
	}
	if t.WstatAll && t.ai == nil && d["Wuid"] != "" {
		ud["Wuid"] = d["Wuid"]
	}
	if err == nil {
		f = nf
		f.mlk.Lock()
		err = f.wstat(t, ud)
		f.mlk.Unlock()
	}
	cs.End(err != nil)
	if err != nil {
		t.dprintf("mkdir %s: %s\n", rid, err)
	}
	c <- err
	close(c, err)
	return c
}

func (t *Fs) Wstat(rid string, d zx.Dir) chan error {
	t.mvlk.RLock()
	defer t.mvlk.RUnlock()
	t.dprintf("wstat %s %v\n", rid, d)
	c := make(chan error, 1)
	rid, err := zx.AbsPath(rid)
	if err != nil {
		c <- err
		close(c, err)
		return c
	}
	cs := t.IOstats.NewCall(zx.Swstat)
	if rid == "/Ctl" {
		close(c)
		cs.End(false)
		return c
	}
	err = t.wstat(rid, d)
	if err == nil {
		t.dprintf("wstat %s: ok\n", rid)
	} else {
		t.dprintf("wstat %s: %s\n", rid, err)
	}
	c <- err
	close(c, err)
	cs.End(err != nil)
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

// make sure paths are ok in children after a move
func (f *mFile) moved() {
	f.mlk.Lock()
	fpath := f.d["path"]
	fspath := f.d["spath"]
	f.mlk.Unlock()
	f.clk.Lock()
	defer f.clk.Unlock()
	for _, cs := range f.child {
		cs.mlk.Lock()
		cs.d["path"] = zx.Path(fpath, cs.name)
		cs.d["spath"] = zx.Path(fspath, cs.name)
		cs.mlk.Unlock()
		cs.moved()
	}
}

// locks f's parent's meta and child
func (t *Fs) detach(f *mFile) error {
	if f.name == "/" {
		return errors.New("can't remove /")
	}
	f.mlk.Lock()
	ppath := path.Dir(f.d["path"])
	f.mlk.Unlock()
	pf, _, err := t.walk(ppath, nil)
	if err != nil {
		return err
	}
	pf.mlk.Lock()
	defer pf.mlk.Unlock()
	pf.clk.Lock()
	defer pf.clk.Unlock()
	n := len(pf.child)
	for i, c := range pf.child {
		if c == f {
			copy(pf.child[i:], pf.child[i+1:])
			pf.child = pf.child[:n-1]
			break
		}
	}
	pf.d["size"] = strconv.Itoa(len(pf.child))
	pf.d.SetTime("mtime", time.Now())
	pf.newDirSum()
	u := dbg.Usr
	if t.ai != nil {
		u = t.ai.Uid
	}
	pf.d["Wuid"] = u
	return nil
}

func (t *Fs) Move(from, to string) chan error {
	t.dprintf("move %s %s \n", from, to)
	// at most one move at a time
	t.mvlk.Lock()
	defer t.mvlk.Unlock()
	cs := t.IOstats.NewCall(zx.Smove)
	c := make(chan error, 1)
	from, ferr := zx.AbsPath(from)
	to, terr := zx.AbsPath(to)
	var pfrom, pto *mFile
	var leftto []string
	var err error
	var fd, pd *mFile
	if ferr != nil {
		err = ferr
	} else if terr != nil {
		err = terr
	} else if from == to {
		c <- nil
		close(c)
		cs.End(false)
		return c
	}

	if err == nil {
		fd, _, err = t.walk(path.Dir(from), nil)
	}
	if err == nil && !t.NoPermCheck {
		fd.mlk.Lock()
		if !fd.d.CanWrite(t.ai) {
			err = fmt.Errorf("%s: %s", fd, dbg.ErrPerm)
		}
		fd.mlk.Unlock()
	}
	if err == nil {
		pd, _, err = t.walk(path.Dir(to), nil)
	}
	var pdpath, pdspath string
	if err == nil {
		pd.mlk.Lock()
		if !t.NoPermCheck && !pd.d.CanWrite(t.ai) {
			err = fmt.Errorf("%s: %s", pd, dbg.ErrPerm)
		}
		pdpath = pd.d["path"]
		pdspath = pd.d["spath"]
		pd.mlk.Unlock()
	}
	if err != nil {
		goto Fail
	}
	if from == "/Ctl" || to == "/Ctl" || from == "/" || to == "/" {
		err = dbg.ErrPerm
	} else if pfrom, _, ferr = t.walk(from, nil); ferr != nil {
		err = ferr
	} else if inconsistent(from, to) {
		err = errors.New("inconsistent move")
	} else if pto, leftto, terr = t.walk(to, nil); terr != nil && !dbg.IsNotExist(terr) {
		err = terr
	} else if len(leftto) > 1 {
		err = terr
	} else if len(leftto) == 0 && pto.d["type"] == "d" {
		err = fmt.Errorf("%s: %s", pto, dbg.ErrExists)
	} else if len(leftto) == 0 && pto.d["type"] != pfrom.d["type"] { // race: no lock in types
		err = fmt.Errorf("%s: incosistent move", pfrom)
	}
	if err == nil {
		err = <-t.lfs.Move(from, to)
	}
Fail:
	if err != nil {
		c <- err
		t.dprintf("move %s: %s\n", from, err)
		close(c, err)
		cs.End(true)
		return c
	}
	t.detach(pfrom)
	pfrom.mlk.Lock()
	pfrom.name = path.Base(to)
	pfrom.d["name"] = pfrom.name
	pfrom.d["path"] = zx.Path(pdpath, pfrom.name)
	pfrom.d["spath"] = zx.Path(pdspath, pfrom.name)
	pfrom.mlk.Unlock()
	pfrom.moved()
	pd.mlk.Lock()
	pd.clk.Lock()
	pd.attach(pfrom, t.ai)
	pd.clk.Unlock()
	pd.mlk.Unlock()
	t.dprintf("move %s: ok\n", from)
	close(c)
	cs.End(false)
	return c
}

func (t *Fs) remove(rid string, all bool, cs *zx.CallStat) chan error {
	t.mvlk.RLock()
	defer t.mvlk.RUnlock()
	c := make(chan error, 1)
	rid, err := zx.AbsPath(rid)
	if rid == "/Ctl" || rid == "/" {
		t.dprintf("remove %s: %s\n", rid, dbg.ErrPerm)
		err := fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
		c <- err
		close(c, err)
		cs.End(true)
		return c
	}
	f, _, err := t.walk(rid, nil)
	if f != nil {
		f.mlk.Lock()
	}
	if err == nil && !all && f.d["type"] == "d" && len(f.child) > 0 {
		err = errors.New("directory is not empty")
	}
	if err == nil && !t.NoPermCheck && !f.d.CanWrite(t.ai) {
		err = dbg.ErrPerm
	}
	if f != nil {
		if err == nil {
			f.d["rm"] = "y"
		}
		f.mlk.Unlock()
	}
	if err == nil {
		if all {
			err = <-t.lfs.RemoveAll(rid)
		} else {
			err = <-t.lfs.Remove(rid)
		}
	}
	if err != nil {
		t.dprintf("remove %s: %s\n", rid, err)
		c <- err
		close(c, err)
		cs.End(true)
		return c
	}
	t.detach(f)
	t.dprintf("remove %s: ok\n", rid)
	close(c)
	cs.End(false)
	return c
}

func (t *Fs) Remove(rid string) chan error {
	t.dprintf("remove %s\n", rid)
	cs := t.IOstats.NewCall(zx.Sremove)
	return t.remove(rid, false, cs)
}

func (t *Fs) RemoveAll(rid string) chan error {
	t.dprintf("removeall %s\n", rid)
	cs := t.IOstats.NewCall(zx.Sremoveall)
	return t.remove(rid, true, cs)
}

func (t *Fs) Fsys(name string) <-chan error {
	t.dprintf("fsys %s\n", name)
	c := make(chan error, 1)
	if name != "" && name != "main" {
		err := errors.New("fsys not supported for local trees")
		t.dprintf(name, "fsys", err)
		c <- err
		close(c, err)
	} else {
		t.dprintf(name, "fsys", "ok")
		close(c)
	}
	return c
}

// d is a dup and can be changed.
func (t *Fs) find(f *mFile, d zx.Dir, p *pred.Pred, spref, dpref string, lvl int, c chan<- zx.Dir) {
	match, pruned, err := p.EvalAt(d, lvl)
	t.dfprintf(f, "find at %v\n\t%v\n\t%v %v %v\n\n", d, p, match, pruned, err)
	if pruned {
		if !match {
			d["err"] = "pruned"
		}
		c <- d
		return
	}
	if d["type"] == "d" && err == nil {
		f.mlk.Lock()
		if !t.NoPermCheck && !f.d.CanWalk(t.ai) {
			err = dbg.ErrPerm
		}
		f.mlk.Unlock()

	}
	if err != nil {
		d["err"] = err.Error()
		c <- d
		return
	}
	if d["rm"] != "" {
		return
	}
	if match {
		if ok := c <- d; !ok {
			return
		}
	}
	if d["type"] != "d" {
		return
	}
	f.clk.Lock()
	child := make([]*mFile, len(f.child))
	copy(child, f.child)
	f.clk.Unlock()
	if f.name == "/" {
		nc := []*mFile{
			&mFile{name: "Ctl", d: ctldir.Dup()},
		}
		nc[0].d["tpath"] = t.path
		child = append(nc, child...)
	}
	for _, cf := range child {
		cf.mlk.Lock()
		cd := cf.d.Dup()
		cf.mlk.Unlock()
		// fmt.Printf("child %s\n", cd)
		if cd["rm"] != "" {
			continue
		}
		if spref != dpref {
			cpath := cd["path"]
			suff := zx.Suffix(cpath, spref)
			cd["path"] = zx.Path(dpref, suff)
		}
		t.find(cf, cd, p, spref, dpref, lvl+1, c)
	}
}

func (t *Fs) Find(rid, fpred, spref, dpref string, depth int) <-chan zx.Dir {
	t.dprintf("find %s '%s' '%s' '%s' %d\n", rid, fpred, spref, dpref, depth)
	cs := t.IOstats.NewCall(zx.Sfind)
	dc := make(chan zx.Dir)
	go func() {
		t.mvlk.RLock()
		defer t.mvlk.RUnlock()
		rid, err := zx.AbsPath(rid)
		if err != nil {
			cs.End(err != nil)
			t.dprintf("find %s: %s\n", rid, err)
			close(dc, err)
			return
		}
		f, _, err := t.walk(rid, nil)
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
		f.mlk.Lock()
		d := f.d.Dup()
		f.mlk.Unlock()
		if spref != dpref {
			suff := zx.Suffix(rid, spref)
			d["path"] = zx.Path(dpref, suff)
		}
		t.find(f, d, p, spref, dpref, depth, dc)
		cs.End(err != nil)
		t.dprintf("find %s: ok\n", rid)
		close(dc)
	}()
	return dc
}

func (t *Fs) FindGet(rid, fpred, spref, dpref string, depth int) <-chan zx.DirData {
	t.dprintf("findget %s '%s' '%s' '%s' %d\n", rid, fpred, spref, dpref, depth)
	gc := make(chan zx.DirData)
	cs := t.IOstats.NewCall(zx.Sfindget)
	go func() {
		dc := t.Find(rid, fpred, spref, dpref, depth) // BUG: will stat a Sfind
		for d := range dc {
			g := zx.DirData{Dir: d}
			var datac chan []byte
			if d["err"] == "" && d["type"] == "-" {
				datac = make(chan []byte)
				g.Datac = datac
			}
			if ok := gc <- g; !ok {
				close(dc, cerror(gc))
				break
			}
			if datac != nil {
				err := t.get(d["spath"], 0, zx.All, datac, nil)
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

func (t *Fs) Dump(w io.Writer) {
	if t == nil {
		fmt.Fprintf(w, "<nil tree>\n")
		return
	}
	fmt.Fprintf(w, "tree [%s] path %s\n", t.name, t.path)
	t.dump(t.root, w, 0)
	fmt.Fprintf(w, "\n")
}

func (t *Fs) dump(f *mFile, w io.Writer, lvl int) {
	tabs := strings.Repeat("    ", lvl)
	if f == nil {
		fmt.Fprintf(w, "%s<nil file>\n", tabs)
		return
	}
	d := f.d.Dup()
	if d["path"] == "/" {
		d["size"] = "0"
	}
	fmt.Fprintf(w, "%s%s\n", tabs, d.TestFmt())
	ld, err := zx.Stat(t.lfs, f.d["path"])
	if err != nil {
		fmt.Fprintf(w, "%s  lfs err: %s\n", tabs, err)
	}
	if d["type"] != "d" {
		fmt.Fprintf(w, "%s  %s bytes\n", tabs, ld["size"])
		return
	}
	for _, c := range f.child {
		t.dump(c, w, lvl+1)
	}

}
