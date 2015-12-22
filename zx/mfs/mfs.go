/*
	on-memory zx fs

	It's main usage is as an on-memory cache for remote
	zx trees.

	See the new cfs.
*/
package mfs

import (
	"bytes"
	"clive/bufs"
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
	A ram file system
*/
type Fs struct {
	name string
	path string // fake tpath
	root *mFile
	ai   *auth.Info

	*zx.Flags
}

type mFile struct {
	name string

	d zx.Dir

	data  *bufs.Blocks
	child []*mFile

	// lock order: meta and then data|child
	mlk sync.Mutex // for the meta
	clk sync.Mutex // for the child
	// data has its own lock.

	t *Fs // for dprintf, mostly

}

var (
	ctldir = zx.Dir{
		"path":  "/Ctl",
		"spath": "/Ctl",
		"name":  "Ctl",
		"proto": "proc",
		"size":  "0",
		"type":  "c",
		"Uid":   dbg.Usr,
		"Gid":   dbg.Usr,
		"Wuid":  dbg.Usr,
		"mode":  "0644",
		//	"Sum": zx.Zsum(),
	}

	// When DoSum is set, mfs computes the Sum attribute for
	// files and directories. This can slow down FUSE quite a bit.
	DoSum = false

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

func (t *Fs) DirFile(df zx.Dir) zx.File {
	return zx.File{t, df}
}

func (t *Fs) Name() string {
	return t.name
}

func New(name string) (*Fs, error) {
	r := &mFile{
		name: "/",
		d: zx.Dir{
			"name":  "/",
			"path":  "/",
			"spath": "/",
			"proto": "proc",
			"type":  "d",
			"mode":  "0755",
			"Uid":   dbg.Usr,
			"Gid":   dbg.Usr,
			"Wuid":  dbg.Usr,
			//	"Sum": zx.Zsum(),
			"size": "0",
		},
		child: []*mFile{},
	}
	r.d.SetTime("mtime", time.Now())
	l := &Fs{
		name:  name,
		root:  r,
		Flags: &zx.Flags{},
	}
	p := fmt.Sprintf("mfs%p", l)
	l.path = p
	r.d["tpath"] = p
	l.Flags.Add("debug", &l.Dbg)
	l.Flags.AddRO("noperm", &l.NoPermCheck)
	l.Flags.Add("clear", func(...string) error {
		l.IOstats.Clear()
		return nil
	})
	r.t = l
	return l, nil
}

func (t *Fs) Close(e error) {
	t.dprintf("close sts %v\n", e)
	zx.UnregisterProcTree(t.path)
}

// Ask the tree to perform auth checks on behalf of ai.
func (t *Fs) AuthFor(ai *auth.Info) (zx.Tree, error) {
	if ai != nil {
		t.dprintf("auth for %s\n", ai.Uid)
	}
	nfs := &Fs{}
	*nfs = *t
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

func (f *mFile) dprintf(fs string, args ...interface{}) {
	if f != nil && f.t.Dbg {
		f.t.dprintf(f.d["path"]+":"+fs, args...)
	}
}

func (f *mFile) walk1(el string) (*mFile, error) {
	f.clk.Lock()
	defer f.clk.Unlock()
	if f.d["type"] != "d" {
		return nil, fmt.Errorf("%s: not a directory", f)
	}
	for _, c := range f.child {
		nm := c.name
		if nm == el {
			return c, nil
		}
	}
	return nil, fmt.Errorf("%s: '%s': file not found", f, el)
}

// to be used when updating files: under WstatAll we don't check
// perms if ai is nil, so we can update mfs caches in cfs.
func (t *Fs) noPerms() bool {
	return t.NoPermCheck || (t.WstatAll && t.ai == nil)
}

// if the error is a file not found, then file is returned with
// the parent file and left contains what's left to walk.
func (t *Fs) walk(rid string, pfp **mFile) (cf *mFile, left []string, e error) {
	els := zx.Elems(rid)
	f := t.root
	for len(els) > 0 {
		if !t.noPerms() && !f.d.CanWalk(t.ai) {
			return f, els, fmt.Errorf("%s: %s", f, dbg.ErrPerm)
		}
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
			if !ctlsent && f.d["name"] == "/" {
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
		if !ctlsent && f.d["name"] == "/" {
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
	cs.Sending()
	n, nm, err := f.data.SendTo(off, count, dc)
	cs.Sends(int64(nm), n)
	return err
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
func (f *mFile) attach(cf *mFile) error {
	u := dbg.Usr
	if f.t.ai != nil {
		u = f.t.ai.Uid
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
	if DoSum {
		f.newDirSum()
	}
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
	return b[i].name < b[j].name
}

func (t *Fs) put(rid string, d zx.Dir, off int64, dc <-chan []byte, pred string) (zx.Dir, int64, int64, error) {
	noinherit := false
	if m := d["Mode"]; m != "" {
		d["mode"] = m
		delete(d, "Mode")
		noinherit = true
	}
	var pf *mFile
	f, left, err := t.walk(rid, &pf)
	if err != nil && !dbg.IsNotExist(err) || len(left) > 1 || d["mode"] == "" && err != nil {
		return nil, 0, 0, err
	}
	pmode := uint64(0755)
	if pf != nil {
		pf.mlk.Lock()
		pmode = pf.d.Mode()
		pf.mlk.Unlock()
	}
	app := false
	f.mlk.Lock()
	if err == nil && f.d["type"] == "d" || d["type"] == "d" {
		err = fmt.Errorf("%s: %s", f, dbg.ErrIsDir)
		f.mlk.Unlock()
		return nil, 0, 0, err
	}
	if !t.noPerms() && !f.d.CanWrite(t.ai) {
		err = fmt.Errorf("%s: %s", f, dbg.ErrPerm)
		f.mlk.Unlock()
		return nil, 0, 0, err
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
			return nil, 0, 0, err
		}
		if !t.WstatAll || t.ai != nil {
			if err := f.d.CanWstat(ai, d); err != nil {
				f.mlk.Unlock()
				return nil, 0, 0, err
			}
		}
	} else {
		if err := t.matchDir(rid, nil, pred); err != nil {
			f.mlk.Unlock()
			return nil, 0, 0, err
		}
	}
	if d["mode"] == "" {
		if off < 0 {
			off = 0
			app = true
		}
		if t.WstatAll && t.ai == nil && d["Wuid"] != "" {
			f.d["Wuid"] = d["Wuid"]
		} else {
			f.d["Wuid"] = u
		}
		if d["size"] != "" {
			f.data.Truncate(d.Int64("size"))
			f.d["size"] = strconv.FormatInt(int64(f.data.Len()), 10)
		}
	} else if err == nil {
		// truncate existing file
		if !noinherit && (!t.WstatAll || t.ai != nil) {
			d.Inherit(pmode)
		}
		f.data.Truncate(d.Int64("size"))
		f.d["size"] = strconv.FormatInt(int64(f.data.Len()), 10)
		if t.WstatAll && t.ai == nil && d["Wuid"] != "" {
			f.d["Wuid"] = d["Wuid"]
		} else {
			f.d["Wuid"] = u
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
				"proto": "proc",
			},
			data: &bufs.Blocks{Mutex: &sync.Mutex{}},
			t:    t,
		}
		if !t.WstatAll || t.ai != nil {
			if !noinherit {
				nf.d.Inherit(pmode)
				d["mode"] = nf.d["mode"]
			}
			if err := f.d.CanWstat(ai, d); err != nil {
				f.mlk.Unlock()
				return nil, 0, 0, err
			}
		} else if d["Wuid"] != "" {
			f.d["Wuid"] = d["Wuid"]
		}
		nf.data.Truncate(d.Int64("size"))
		nf.d["size"] = strconv.FormatInt(int64(nf.data.Len()), 10)
		f.clk.Lock()
		f.attach(nf)
		f.clk.Unlock()
		f.mlk.Unlock()
		f = nf
		f.mlk.Lock()
	}
	if app {
		off = int64(f.data.Len())
	}
	delete(d, "size")
	f.wstat(d.UsrAttrs())
	f.mlk.Unlock()

	n, err := f.data.RecvAtFrom(off, dc)

	f.mlk.Lock()
	defer f.mlk.Unlock()
	if DoSum {
		sum := f.data.Sum()
		f.d["Sum"] = sum
	}
	size := f.data.Len()
	f.d["size"] = strconv.Itoa(size)
	if d["mtime"] == "" {
		f.d.SetTime("mtime", time.Now())
	}
	return f.d.Dup(), 1, int64(n), err
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
		cs.Sending()
		rid, err := zx.AbsPath(rid)
		var nm, n int64
		var nd zx.Dir
		if err == nil && rid == "/Ctl" {
			nc, xerr := t.putCtl(dc)
			if xerr == nil {
				nd = zx.Dir{"size": "0"}
				if DoSum {
					nd["Sum"] = zx.Zsum()
				}
				nd.SetTime("mtime", time.Now())
				nm = 1
				n = int64(nc)
			}
			err = xerr
		} else if err == nil {
			nd, nm, n, err = t.put(rid, d, off, dc, pred)
			cs.Sends(nm, n)
		}
		if err == nil {
			rd := zx.Dir{"size": nd["size"], "mtime": nd["mtime"]}
			if nd["Sum"] != "" {
				rd["Sum"] = nd["Sum"]
			}
			t.dprintf("put %s: %s (wrote %d)\n", rid, rd, n)
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
func (f *mFile) wstat(d zx.Dir) error {
	if len(d) == 0 {
		return nil
	}
	d = d.Dup()
	sum := ""
	if f.d["type"] != "d" {
		if _, ok := d["size"]; ok {
			sz := d.Int64("size")
			if sz < 0 {
				sz = 0
			}
			f.data.Truncate(sz)
			d["size"] = strconv.FormatInt(sz, 10)
			if DoSum {
				sum = f.data.Sum()
			}
		}
	} else {
		delete(d, "size")
	}
	if _, ok := d["mode"]; ok {
		mode := d.Int("mode") & 0777
		d["mode"] = "0" + strconv.FormatInt(int64(mode), 8)
	}
	if _, ok := d["mtime"]; ok {
		d.SetTime("mtime", d.Time("mtime"))
	}
	if sum != "" {
		f.d["Sum"] = sum
	}
	ud := d.UsrAttrs()
	if d["Wuid"] != "" {
		ud["Wuid"] = d["Wuid"]
	}
	for k, v := range ud {
		if v == "" {
			delete(f.d, k)
		} else {
			f.d[k] = v
		}
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
	return f.wstat(ud)
}

func (t *Fs) Mkdir(rid string, d zx.Dir) chan error {
	t.dprintf("mkdir %s %v\n", rid, d)
	cs := t.IOstats.NewCall(zx.Smkdir)
	c := make(chan error, 1)
	rid, err := zx.AbsPath(rid)
	if rid == "/" || rid == "/Ctl" {
		err = dbg.ErrExists
	}
	var f *mFile
	var left []string
	if err == nil {
		f, left, err = t.walk(rid, nil)
	}
	if err == nil {
		err = fmt.Errorf("'%s': %s", rid, dbg.ErrExists)
	} else if len(left) == 1 {
		err = nil
	}
	if err != nil {
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
		f.mlk.Lock()
		ud["type"] = "d"
		ud.Inherit(f.d.Mode())
		delete(ud, "type")
		f.mlk.Unlock()
	}
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
			"tpath": f.t.path,
			"Uid":   u,
			"Gid":   f.d["Gid"],
			"Wuid":  u,
			"type":  "d",
			"size":  "0",
			"proto": "proc",
		},
		child: []*mFile{},
		t:     t,
	}
	if DoSum {
		nf.d["Sum"] = zx.Zsum()
	}
	f.mlk.Lock()
	if !t.noPerms() && !f.d.CanWrite(t.ai) {
		err = fmt.Errorf("%s: %s", f, dbg.ErrPerm)
	}
	ai := t.ai
	if t.NoPermCheck {
		ai = nil
	}
	if err == nil && (!t.WstatAll || t.ai != nil) {
		err = nf.d.CanWstat(ai, ud)
	}
	if err != nil {
		f.mlk.Unlock()
		cs.End(true)
		t.dprintf("mkdir %s: %s\n", rid, err)
		c <- err
		close(c, err)
		return c
	}
	f.mlk.Unlock()
	f.mlk.Lock()
	f.clk.Lock()
	f.attach(nf)
	f.clk.Unlock()
	f.mlk.Unlock()
	f = nf
	f.mlk.Lock()
	if t.WstatAll && t.ai == nil && d["Wuid"] != "" {
		ud["Wuid"] = d["Wuid"]
	}
	err = f.wstat(ud)
	f.mlk.Unlock()
	cs.End(err != nil)
	if err != nil {
		t.dprintf("mkdir %s: %s\n", rid, err)
	}
	c <- err
	close(c, err)
	return c
}

func (t *Fs) Wstat(rid string, d zx.Dir) chan error {
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
	if DoSum {
		pf.newDirSum()
	}
	u := dbg.Usr
	if t.ai != nil {
		u = t.ai.Uid
	}
	pf.d["Wuid"] = u
	return nil
}

func (t *Fs) Move(from, to string) chan error {
	t.dprintf("move %s %s \n", from, to)
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
	if err == nil && !t.noPerms() {
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
		if !t.noPerms() && !pd.d.CanWrite(t.ai) {
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
	pd.attach(pfrom)
	pd.clk.Unlock()
	pd.mlk.Unlock()
	t.dprintf("move %s: ok\n", from)
	close(c)
	cs.End(false)
	return c
}

func (t *Fs) remove(rid string, all bool, cs *zx.CallStat) chan error {
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
	if err == nil && !t.noPerms() && !f.d.CanWrite(t.ai) {
		err = dbg.ErrPerm
	}
	if f != nil {
		if err == nil {
			f.d["rm"] = "y"
		}
		f.mlk.Unlock()
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
func (f *mFile) find(d zx.Dir, p *pred.Pred, spref, dpref string, lvl int, c chan<- zx.Dir, ai *auth.Info) {
	match, pruned, err := p.EvalAt(d, lvl)
	f.dprintf("find  at %d %s\n\t%v\n\t%v %v %v\n\n",
		lvl, d.Long(), p, match, pruned, err)
	if pruned {
		if !match {
			d["err"] = "pruned"
		}
		c <- d
		return
	}
	if d["type"] == "d" && err == nil {
		f.mlk.Lock()
		if !f.t.NoPermCheck && !f.d.CanWalk(ai) {
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
			&mFile{name: "Ctl", d: ctldir.Dup(), t: f.t},
		}
		nc[0].d["tpath"] = f.t.path
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
		cf.find(cd, p, spref, dpref, lvl+1, c, ai)
	}
}

func (t *Fs) Find(rid, fpred, spref, dpref string, depth int) <-chan zx.Dir {
	t.dprintf("find %s '%s' '%s' '%s' %d\n", rid, fpred, spref, dpref, depth)
	cs := t.IOstats.NewCall(zx.Sfind)
	dc := make(chan zx.Dir)
	go func() {
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
		f.find(d, p, spref, dpref, depth, dc, t.ai)
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
	t.root.Dump(w, 0)
	fmt.Fprintf(w, "\n")
}

func (f *mFile) Dump(w io.Writer, lvl int) {
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
	if d["type"] != "d" {
		fmt.Fprintf(w, "%s  %d bytes\n", tabs, f.data.Len())
		return
	}
	for _, c := range f.child {
		c.Dump(w, lvl+1)
	}

}
