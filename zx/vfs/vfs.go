/*
	A virtual ZX fs library to help virtual FS writers.
*/
package vfs

import (
	"bytes"
	"clive/app"
	"clive/dbg"
	"clive/nchan"
	"clive/net/auth"
	"clive/zx"
	"clive/zx/pred"
	"errors"
	"fmt"
	"path"
	"sort"
	"time"
)

// A virtual ZX tree implemented by the user of this package.
// Depending on the interfaces implemented by the user structures
// for the files, each file accepts more or less operations.
type Fs struct {
	name string
	path string // fake tpath
	ai   *auth.Info
	*zx.Flags

	root File
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
	}

	// make sure we implement the right interfaces
	_fs  *Fs
	_t   zx.RWTree   = _fs
	_r   zx.Recver   = _fs
	_snd zx.Sender   = _fs
	_g   zx.Getter   = _fs
	_w   zx.Walker   = _fs
	_s   zx.Stater   = _fs
	_a   zx.AuthTree = _fs
)

func (t *Fs) String() string {
	return t.name
}

// Tell fuse if the entire tree is virtual. See New.
func (t *Fs) IsCtl() bool {
	if ct, ok := t.root.(zx.IsCtler); ok {
		return ct.IsCtl()
	}
	return false
}

func (t *Fs) DirFile(df zx.Dir) zx.File {
	return zx.File{t, df}
}

func (t *Fs) Close(e error) {
	t.dprintf("close sts %v\n", e)
	if cf, ok := t.root.(Closer); ok {
		cf.Close(e)
	}
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

func (t *Fs) Name() string {
	return t.name
}

func (t *Fs) dprintf(fs string, args ...interface{}) {
	if t != nil && t.Dbg {
		dbg.Printf(t.name+":"+fs, args...)
	}
}

// Create a ZX tree starting at root.
// If root implements zx.IsCtlr then the tree IsCtl returns true,
// which means that FUSE considers it all-virtual.
func New(name string, root File) (*Fs, error) {
	t := &Fs{
		name:  name,
		root:  root,
		Flags: &zx.Flags{},
	}
	p := fmt.Sprintf("qlfs%p", t)
	t.path = p
	t.Flags.Add("debug", &t.Dbg)
	return t, nil
}

func (t *Fs) Dprintf(fmts string, args ...interface{}) {
	if t.Dbg {
		app.Eprintf(fmts, args...)
	}
}

func (t *Fs) Stats() *zx.IOstats {
	return t.IOstats
}

func (t *Fs) walk(rid string) (File, error) {
	rid, err := zx.AbsPath(rid)
	if err != nil {
		return nil, err
	}
	if rid == "/" {
		return t.root, nil
	}
	if rid == "/Ctl" {
		return nil, nil
	}
	els := zx.Elems(rid)
	f := t.root
	p := "/"
	var d zx.Dir
	for _, e := range els {
		t.Dprintf("walk %s %s...\n", f, e)
		d, err = f.Stat()
		if err != nil {
			t.Dprintf("\tstat: %s\n", f, err)
			return nil, err
		}
		if d["type"] != "d" {
			t.Dprintf("\tnot dir\n")
			return nil, fmt.Errorf("%s: %s", p, dbg.ErrNotDir)
		}
		if !t.NoPermCheck && !d.CanWalk(t.ai) {
			t.Dprintf("\tno perm\n")
			return nil, fmt.Errorf("%s: %s", p, dbg.ErrPerm)
		}
		wf, ok := f.(Walker)
		if !ok {
			t.Dprintf("\tnot walker\n")
			return nil, fmt.Errorf("%s: %s: %s", p, e, dbg.ErrNotExist)
		}
		f, err = wf.Walk(e)
		if err != nil {
			t.Dprintf("\twalk: %s\n", err)
			return nil, err
		}
		p = zx.Path(p, e)
		t.Dprintf("walked %s\n", f)
	}
	return f, nil
}

func (t *Fs) statf(f File, rid string) (zx.Dir, error) {
	d, err := f.Stat()
	if d != nil {
		d["path"] = rid
		d["spath"] = rid
		d["proto"] = "proc"
		if d["Uid"] == "" {
			d["Uid"] = dbg.Usr
			d["Gid"] = dbg.Usr
			d["Wuid"] = dbg.Usr
		}
	}
	return d, err
}

func (t *Fs) stat(rid string) (zx.Dir, error) {
	f, err := t.walk(rid)
	if err != nil {
		return nil, err
	}
	if f == nil && err == nil {
		return ctldir.Dup(), nil
	}
	return t.statf(f, rid)
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

func (t *Fs) match(d zx.Dir, err error, rid, fpred string) error {
	if fpred == "" {
		return nil
	}
	if err != nil {
		if dbg.IsNotExist(err) {
			d = zx.Dir{
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

func (t *Fs) getCtl(off, count int64, c chan<- []byte) error {
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
	c <- resp
	return nil
}

func (t *Fs) getdir(f File, d zx.Dir, off, count int64, c chan<- []byte) error {
	gf, ok := f.(Walker)
	if !ok {
		return nil
	}
	ns, err := gf.Getdir()
	if err != nil {
		return err
	}
	sort.Sort(sort.StringSlice(ns))
	if d["name"] == "/" {
		ns = append([]string{"Ctl"}, ns...)
	}
Dloop:
	for _, n := range ns {
		if n == "" {
			err = fmt.Errorf("%s: empty name in getdir", d["path"], n)
			app.Warn("fs bug: %s", err)
			return err
		}
		if off > 0 {
			off--
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
		if d["name"] == "/" && n == "Ctl" {
			cd := ctldir.Dup()
			cd["tpath"] = t.path
			cd.Send(c)
			continue
		}
		cf, err := gf.Walk(n)
		if err != nil {
			return err
		}
		cp := zx.Path(d["path"], n)
		cd, err := t.statf(cf, cp)
		if err != nil {
			return err
		}
		t.Dprintf("getdir %s: %s\n", gf, cf)
		if _, err := cd.Send(c); err != nil {
			return err
		}
	}
	return nil
}

func (t *Fs) Get(rid string, off, count int64, pred string) <-chan []byte {
	t.dprintf("get %s %d %d %q\n", rid, off, count, pred)
	cs := t.IOstats.NewCall(zx.Sget)
	c := make(chan []byte)
	go func() {
		var d zx.Dir
		f, err := t.walk(rid)
		if f == nil && err == nil { // Ctl
			d = ctldir.Dup()
		} else if err == nil {
			d, err = t.statf(f, rid)
		}
		if err == nil && !t.NoPermCheck {
			if !d.CanRead(t.ai) {
				err = fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
			}
		}
		if err == nil && pred != "" {
			err = t.match(d, err, rid, pred)
		}
		if err == nil {
			if d["path"] == "/Ctl" {
				err = t.getCtl(off, count, c)
			} else if d["type"] != "d" {
				if gf, ok := f.(Getter); ok {
					err = gf.Get(off, count, c)
				}
			} else {
				err = t.getdir(f, d, off, count, c)
			}
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

func (t *Fs) put(rid string, d zx.Dir, off int64, dc <-chan []byte, pred string) error {
	pf, err := t.walk(path.Dir(rid))
	if err != nil {
		return err
	}
	pd, err := pf.Stat()
	if pd["type"] != "d" {
		return fmt.Errorf("%s: %s", pd["path"], dbg.ErrNotDir)
	}
	wpf, ok := pf.(Walker)
	if !ok {
		return fmt.Errorf("%s: %s", pd["path"], dbg.ErrPerm)
	}
	f, err := wpf.Walk(path.Base(rid))
	if err != nil && !dbg.IsNotExist(err) {
		return err
	}
	if err != nil {
		if err := t.matchDir(rid, nil, pred); err != nil {
			return err
		}
		if !t.NoPermCheck && !pd.CanWrite(t.ai) {
			return fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
		}
		if putf, ok := pf.(Putter); ok {
			return putf.Put(path.Base(rid), d, off, dc)
		}
		return fmt.Errorf("%s: %s", pd["path"], dbg.ErrPerm)
	} else {
		d, err := f.Stat()
		if err != nil {
			return err
		}
		if d["type"] == "d" {
			return fmt.Errorf("%s: %s", rid, dbg.ErrIsDir)
		}
		if err := t.matchDir(rid, d, pred); err != nil {
			return err
		}
		if !t.NoPermCheck && !d.CanWrite(t.ai) {
			return fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
		}
		if putf, ok := f.(Putter); ok {
			return putf.Put("", d, off, dc)
		}
		return fmt.Errorf("%s: %s", d["path"], dbg.ErrPerm)
	}
}

func (t *Fs) putCtl(datc <-chan []byte) error {
	ctl, err := nchan.String(datc)
	if err != nil {
		return fmt.Errorf("/Ctl: %s", err)
	}
	if cp, ok := t.root.(Ctler); ok {
		ok, err = cp.PutCtl(ctl)
		if ok {
			return err
		}
	}
	return t.Ctl(ctl)
}

func (t *Fs) Put(rid string, d zx.Dir, off int64, dc <-chan []byte, pred string) chan zx.Dir {
	d = d.UsrAttrs()
	t.dprintf("put %s %v %d '%s'\n", rid, d, off, pred)
	cs := t.IOstats.NewCall(zx.Sput)
	c := make(chan zx.Dir, 1)
	go func() {
		cs.Sending()
		rid, err := zx.AbsPath(rid)
		var nd zx.Dir
		if err == nil && rid == "/" {
			err = fmt.Errorf("/: %s", dbg.ErrPerm)
		}
		if err == nil && rid == "/Ctl" {
			xerr := t.putCtl(dc)
			if xerr == nil {
				nd = zx.Dir{"size": "0", "Sum": zx.Zsum()}
				nd.SetTime("mtime", time.Now())
			}
			err = xerr
		} else if err == nil {
			err = t.put(rid, d, off, dc, pred)
			if err == nil {
				nd, err = t.stat(rid)
			}
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

func (t *Fs) mkdir(rid string, d zx.Dir) error {
	d = d.UsrAttrs()
	pf, err := t.walk(path.Dir(rid))
	if err != nil {
		return err
	}
	pd, err := pf.Stat()
	if pd["type"] != "d" {
		return fmt.Errorf("%s: %s", pd["path"], dbg.ErrNotDir)
	}
	wpf, ok := pf.(Mkdirer)
	if !ok {
		return fmt.Errorf("%s: %s", pd["path"], dbg.ErrPerm)
	}
	_, err = wpf.Walk(path.Base(rid))
	if err == nil {
		return fmt.Errorf("%s: %s", pd["path"], dbg.ErrExists)
	}
	if !t.NoPermCheck && !pd.CanWrite(t.ai) {
		return fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
	}
	return wpf.Mkdir(path.Base(rid), d)
	return fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
}

func (t *Fs) Mkdir(rid string, d zx.Dir) chan error {
	t.dprintf("mkdir %s %v\n", rid, d)
	cs := t.IOstats.NewCall(zx.Smkdir)
	rid, err := zx.AbsPath(rid)
	if rid == "/Ctl" || rid == "/" {
		err = dbg.ErrExists
	} else {
		err = t.mkdir(rid, d)
	}
	c := make(chan error, 1)
	cs.End(err != nil)
	if err != nil {
		t.dprintf("mkdir %s: %s\n", rid, err)
	}
	c <- err
	close(c, err)
	return c
}

func (t *Fs) wstat(rid string, d zx.Dir) error {
	f, err := t.walk(rid)
	if err != nil {
		return err
	}
	ud := d.UsrAttrs()
	d, err = f.Stat()
	if err != nil {
		return err
	}
	ai := t.ai
	if t.NoPermCheck {
		ai = nil
	}
	if !t.WstatAll || t.ai != nil {
		if err := d.CanWstat(ai, ud); err != nil {
			return err
		}
	}
	if wsf, ok := f.(Wstater); !ok {
		if _, ok := f.(Putter); !ok {
			return fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
		}
		// ignore wstats if there's Put so that echo > file works.
		return nil
	} else {
		return wsf.Wstat(ud)
	}
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

func (t *Fs) move(from, to string) error {
	from, ferr := zx.AbsPath(from)
	if ferr != nil {
		return ferr
	}
	if from == "/" || from == "/Ctl" {
		return fmt.Errorf("%s: %s", from, dbg.ErrPerm)
	}
	to, terr := zx.AbsPath(to)
	if terr != nil {
		return terr
	}
	if to == "/" || to == "/Ctl" {
		return fmt.Errorf("%s: %s", from, dbg.ErrPerm)
	}
	if inconsistent(from, to) {
		return fmt.Errorf("%s: inconsistent move", from)
	}
	pfromf, err := t.walk(path.Dir(from))
	if err != nil {
		return err
	}
	pfrommv, ok := pfromf.(Mover)
	if !ok {
		return fmt.Errorf("%s: %s", from, dbg.ErrPerm)
	}
	pfd, err := pfromf.Stat()
	if err != nil {
		return err
	}
	if !t.NoPermCheck && !pfd.CanWrite(t.ai) {
		return fmt.Errorf("%s: %s", pfd["path"], dbg.ErrPerm)
	}
	var fromf File
	fromf, err = pfrommv.Walk(path.Base(from))
	if err != nil {
		return err
	}

	pto, err := t.walk(path.Dir(to))
	if err != nil {
		return err
	}
	ptd, err := pto.Stat()
	if err != nil {
		return err
	}
	if !t.NoPermCheck && !ptd.CanWrite(t.ai) {
		return fmt.Errorf("%s: %s", ptd["path"], dbg.ErrPerm)
	}
	ptow, ok := pto.(Walker)
	if !ok {
		return fmt.Errorf("%s: %s", ptd["path"], dbg.ErrPerm)
	}
	tof, err := ptow.Walk(path.Base(to))
	if err == nil {
		tod, err := tof.Stat()
		if tod["type"] == "d" {
			return fmt.Errorf("%s: %s", to, dbg.ErrIsDir)
		}
		return err
	}
	return pfrommv.Move(fromf, path.Base(from), pto, path.Base(to))
}

func (t *Fs) Move(from, to string) chan error {
	t.dprintf("move %s %s \n", from, to)
	cs := t.IOstats.NewCall(zx.Smove)
	c := make(chan error, 1)
	err := t.move(from, to)
	cs.End(err != nil)
	c <- err
	close(c, err)
	return c
}

func (t *Fs) remove(rid string, all bool) error {
	rid, err := zx.AbsPath(rid)
	if err != nil {
		return err
	}
	if rid == "/" || rid == "/Ctl" {
		return fmt.Errorf("%s: %s", rid, dbg.ErrPerm)
	}
	pf, err := t.walk(path.Dir(rid))
	if err != nil {
		return err
	}
	pd, err := pf.Stat()
	if err != nil {
		return err
	}
	pfrm, ok := pf.(Remover)
	if !ok {
		return fmt.Errorf("%s: %s", pd["path"], dbg.ErrPerm)
	}
	if !t.NoPermCheck && !pd.CanWrite(t.ai) {
		return fmt.Errorf("%s: %s", pd["path"], dbg.ErrPerm)
	}
	nm := path.Base(rid)
	f, err := pfrm.Walk(nm)
	if err != nil {
		return err
	}
	return pfrm.Remove(f, nm, all)
}

func (t *Fs) Remove(rid string) chan error {
	t.dprintf("remove %s\n", rid)
	cs := t.IOstats.NewCall(zx.Sremove)
	c := make(chan error, 1)
	err := t.remove(rid, false)
	if err != nil {
		t.dprintf("remove %s: %s\n", rid, err)
	}
	cs.End(err != nil)
	c <- err
	close(c, err)
	t.dprintf("remove %s: ok\n", rid)
	return c
}

func (t *Fs) RemoveAll(rid string) chan error {
	t.dprintf("removeall %s\n", rid)
	cs := t.IOstats.NewCall(zx.Sremove)
	c := make(chan error, 1)
	err := t.remove(rid, true)
	if err != nil {
		t.dprintf("removeall %s: %s\n", rid, err)
	}
	cs.End(err != nil)
	c <- err
	close(c, err)
	t.dprintf("removeall %s: ok\n", rid)
	return c
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
func (t *Fs) find(f File, d zx.Dir, p *pred.Pred, spref, dpref string, lvl int,
	c chan<- zx.Dir, ai *auth.Info) {
	match, pruned, err := p.EvalAt(d, lvl)
	t.dprintf("find  at %v\n\t%v\n\t%v %v %v\n\n", d, p, match, pruned, err)
	if pruned {
		if !match {
			d["err"] = "pruned"
		}
		c <- d
		return
	}
	if d["type"] == "d" && err == nil {
		if !t.NoPermCheck && !d.CanWalk(ai) {
			err = dbg.ErrPerm
		}
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
	if d["type"] != "d" || f == nil {
		return
	}
	wf, ok := f.(Walker)
	if !ok {
		return
	}
	ns, err := wf.Getdir()
	if err != nil {
		return
	}
	sort.Sort(sort.StringSlice(ns))
	if d["name"] == "/" {
		cd := ctldir.Dup()
		if spref != dpref {
			cpath := cd["path"]
			suff := zx.Suffix(cpath, spref)
			cd["path"] = zx.Path(dpref, suff)
		}
		t.find(nil, cd, p, spref, dpref, lvl+1, c, ai)
	}
	for _, cnm := range ns {
		cf, err := wf.Walk(cnm)
		if err != nil {
			continue
		}
		cp := zx.Path(d["path"], cnm)
		cd, err := t.statf(cf, cp)
		if err != nil || cd["rm"] != "" {
			continue
		}
		cd = cd.Dup()
		if spref != dpref {
			cpath := cd["path"]
			suff := zx.Suffix(cpath, spref)
			cd["path"] = zx.Path(dpref, suff)
		}
		t.find(cf, cd, p, spref, dpref, lvl+1, c, ai)
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
		f, err := t.walk(rid)
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
		d, err := t.statf(f, rid)
		if err != nil {
			cs.End(err != nil)
			t.dprintf("find %s: %s\n", rid, err)
			close(dc, err)
			return
		}
		d = d.Dup()
		if spref != dpref {
			suff := zx.Suffix(rid, spref)
			d["path"] = zx.Path(dpref, suff)
		}
		t.find(f, d, p, spref, dpref, depth, dc, t.ai)
		cs.End(err != nil)
		t.dprintf("find %s: ok\n", rid)
		close(dc)
	}()
	return dc
}

// used only by findget
func (t *Fs) get(rid string, datac chan<- []byte) error {
	f, err := t.walk(rid)
	if err != nil {
		return err
	}
	gf, ok := f.(Getter)
	if !ok {
		return nil
	}
	return gf.Get(0, -1, datac)
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
			if d["err"] == "" && !t.NoPermCheck &&
				d["type"] != "d" && !d.CanRead(t.ai) {
				d["err"] = dbg.ErrPerm.Error()

			}
			if d["err"] == "" && d["type"] != "d" {
				datac = make(chan []byte)
				g.Datac = datac
			}
			if ok := gc <- g; !ok {
				close(dc, cerror(gc))
				break
			}
			if datac != nil {
				err := t.get(d["spath"], datac)
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
