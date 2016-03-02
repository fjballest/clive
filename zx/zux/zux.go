/*
	ZX for UX file systems
*/
package zux

import (
	"bytes"
	"clive/ch"
	"clive/dbg"
	"clive/net/auth"
	"clive/u"
	"clive/zx"
	"clive/zx/pred"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	fpath "path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

struct Fs {
	*dbg.Flag
	*zx.Flags
	*zx.Stats
	ai      *auth.Info
	root    string
	attrs   bool
	zxperms bool
}

var ctldir = zx.Dir{
	"name":  "Ctl",
	"path":  "/Ctl",
	"addr":  "lfs!/!/Ctl",
	"mode":  "0644",
	"size":  "0",
	"mtime": "0",
	"type":  "c",
	"uid":   u.Uid,
	"gid":   u.Uid,
	"wuid":  u.Uid,
}

var (
	uids   = map[uint32]string{}
	uidslk sync.Mutex

	dontremove bool      // set during testing to prevent removes
	_fs        zx.FullFs = &Fs{}

	paranoia = false // if true, would panic if removing outside /tmp/...
)

func (fs *Fs) String() string {
	return fs.Tag
}

// Return a new view for fs, authenticated for ai
func (fs *Fs) Auth(ai *auth.Info) (zx.Fs, error) {
	if !fs.attrs {
		return fs, nil
	}
	nfs := &Fs{}
	*nfs = *fs
	if ai != nil {
		dbg.Warn("%s: auth for %s %v\n", fs.Tag, ai.Uid, ai.Gids)
	}
	nfs.ai = ai
	return nfs, nil
}

func (fs *Fs) Sync() error {
	if fs.attrs {
		ac.sync()
	}
	return nil
}

func new(root string, attrs bool) (*Fs, error) {
	p, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(p)
	if err != nil {
		return nil, err
	}
	tag := fpath.Base(root)
	fs := &Fs{
		root:  p,
		attrs: attrs,
		Flag:  &dbg.Flag{Tag: tag},
		Flags: &zx.Flags{},
		Stats: &zx.Stats{},
	}
	fs.Flags.Add("debug", &fs.Debug)
	fs.Flags.AddRO("attrs", &fs.attrs)
	fs.Flags.Add("clear", func(...string) error {
		fs.Stats.Clear()
		return nil
	})
	return fs, nil
}

// Return a new Fs rooted at the given unix dir
// handling zx attrs
func NewZX(root string) (*Fs, error) {
	return new(root, true)
}

// Return a new Fs rooted at the given unix dir
// without handling zx attrs
func New(root string) (*Fs, error) {
	return new(root, false)
}

func uidName(uid uint32) string {
	uidslk.Lock()
	defer uidslk.Unlock()
	if u, ok := uids[uid]; ok {
		return u
	}
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return "none"
	}
	name := strings.ToLower(u.Username)
	uids[uid] = name
	return name
}

func newDir(fi os.FileInfo) zx.Dir {
	d := zx.Dir{}
	mode := fi.Mode()
	d.SetMode(uint64(mode))
	d.SetSize(fi.Size())
	switch {
	case fi.IsDir():
		d["type"] = "d"
		d["size"] = "0"
	case mode&os.ModeSymlink != 0:
		d["type"] = "l"
	case mode&(os.ModeNamedPipe|os.ModeSocket) != 0:
		d["type"] = "p"
	case mode&os.ModeDevice != 0:
		d["type"] = "c"
	default:
		d["type"] = "-"
	}
	d["name"] = fi.Name()
	d.SetTime("mtime", fi.ModTime())
	sys := fi.Sys()
	if st, ok := sys.(*syscall.Stat_t); ok && st != nil {
		d["uid"] = uidName(st.Uid)
		d["gid"] = uidName(st.Gid)
	} else {
		d["uid"] = u.Uid
		d["gid"] = u.Uid
	}
	d["wuid"] = d["uid"]
	return d
}

// Check ZX perms (besides the underlying unix ones)
func (fs *Fs) CheckZXPerms() {
	fs.zxperms = true
}

func (fs *Fs) stat(p string, chk bool) (zx.Dir, error) {
	p, err := zx.UseAbsPath(p)
	if err != nil {
		return nil, err
	}
	if fs.zxperms && chk {
		if err := fs.chkWalk(p, false); err != nil {
			return nil, err
		}
	}
	if p == "/Ctl" {
		d := ctldir.Dup()
		d["addr"] = fmt.Sprintf("lfs!%s!/Ctl", fs.root)
		return d, nil
	}
	path := fpath.Join(fs.root, p)
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	d := newDir(st)
	d["path"] = p
	d["addr"] = fmt.Sprintf("lfs!%s!%s", fs.root, p)
	if p == "/" {
		d["name"] = "/"
	}
	if fs.attrs || fs.zxperms {
		ac.get(path, d)
	}
	return d, nil
}

func (fs *Fs) Stat(p string) <-chan zx.Dir {
	fs.Count(zx.Sstat)
	c := make(chan zx.Dir, 1)
	d, err := fs.stat(p, false)
	if err == nil {
		c <- d
	}
	close(c, err)
	return c
}

func (fs *Fs) getCtl(off, count int64, dc chan<- []byte) error {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "lfs %s:\n", fs.Tag)
	fmt.Fprintf(&buf, "%s", fs.Flags)
	fmt.Fprintf(&buf, "%s", fs.Stats)

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
	if ok := dc <- resp[:n]; !ok {
		return cerror(dc)
	}
	return nil
}

// can't use ch, because it uses chan<- face{} and not chan<- []byte
func readBytes(r io.Reader, c chan<- []byte) error {
	var err error
	buf := make([]byte, ch.MsgSz)
	for {
		n, rerr := r.Read(buf[0:])
		if rerr != nil {
			if rerr != io.EOF && err == nil {
				err = rerr
			}
			return err
		}
		m := make([]byte, n)
		copy(m, buf[:n])
		if ok := c <- m; !ok {
			if err == nil {
				err = cerror(c)
			}
			return err
		}
	}
}

func (fs *Fs) get(p string, off, count int64, dc chan<- []byte) error {
	p, err := zx.UseAbsPath(p)
	if err != nil {
		return err
	}
	if p == "/Ctl" {
		return fs.getCtl(off, count, dc)
	}
	if fs.zxperms {
		if err := fs.chkGet(p); err != nil {
			return err
		}
	}
	path := fpath.Join(fs.root, p)
	fd, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fd.Close()
	st, err := fd.Stat()
	if err != nil {
		return err
	}
	if !st.IsDir() {
		if off != 0 {
			if _, err = fd.Seek(off, 0); err != nil {
				return err
			}
		}
		if count == zx.All {
			return readBytes(fd, dc)
		} else {
			rr := io.LimitReader(fd, count)
			return readBytes(rr, dc)
		}
	}

	ds, err := ioutil.ReadDir(path)
	ctlsent := false
Dloop:
	for i := 0; i < len(ds); {
		if off > 0 {
			off--
			if !ctlsent && p == "/" {
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
		if !ctlsent && p == "/" {
			ctlsent = true
			cd := ctldir.Dup()
			cd["addr"] = fmt.Sprintf("lfs!%s!/Ctl", fs.root)
			if ok := dc <- cd.Bytes(); !ok {
				return cerror(dc)
			}
			// but not i++
			continue
		}
		fi := ds[i]
		if fi.Name() == AttrFile || fi.Name() == ".#zx" { // .#zx was the old AttrFile
			if i == len(ds)-1 {
				break
			}
			copy(ds[i:], ds[i+1:])
			ds = ds[:len(ds)-1]
			continue
		}
		d := newDir(fi)
		if d["name"] == AttrFile {
			dbg.Warn("zux get: dir has name .zx")
		}
		cp := fpath.Join(p, fi.Name())
		cpath := fpath.Join(path, fi.Name())
		d["path"] = cp
		d["addr"] = fmt.Sprintf("lfs!%s!%s", fs.root, cp)
		if fs.attrs || fs.zxperms {
			ac.get(cpath, d)
		}
		if ok := dc <- d.Bytes(); !ok {
			return cerror(dc)
		}
		i++
	}
	return nil
}

func (fs *Fs) Get(path string, off, count int64) <-chan []byte {
	c := make(chan []byte)
	go func() {
		fs.Count(zx.Sget)
		err := fs.get(path, off, count, c)
		close(c, err)
	}()
	return c
}

func (fs *Fs) putCtl(c <-chan []byte) error {
	var buf bytes.Buffer
	for d := range c {
		buf.Write(d)
	}
	if err := cerror(c); err != nil {
		return err
	}
	ctl := buf.String()
	return fs.Ctl(ctl)

}

func (fs *Fs) wstat(p string, d zx.Dir, chk bool) error {
	p, err := zx.UseAbsPath(p)
	if err != nil {
		return err
	}
	if fs.zxperms && chk {
		if err := fs.chkWstat(p, d); err != nil {
			return err
		}
	}
	path := fpath.Join(fs.root, p)
	if _, ok := d["size"]; ok && d["type"] != "d" {
		sz := d.Size()
		err = os.Truncate(path, sz)
	}
	if _, ok := d["mode"]; ok {
		mode := d.Mode()
		err = os.Chmod(path, os.FileMode(mode))
	}
	if _, ok := d["mtime"]; ok {
		mt := d.Time("mtime")
		if nerr := os.Chtimes(path, mt, mt); nerr != nil && err == nil {
			err = nerr
		}
	}
	if fs.attrs {
		ac.set(path, d)
	}
	return err
}

func (fs *Fs) Wstat(p string, d zx.Dir) <-chan zx.Dir {
	rc := make(chan zx.Dir)
	go func() {
		fs.Count(zx.Swstat)
		d = d.SysDup()
		if d["wuid"] != "" || d["size"] != "" {
			d["wuid"] = u.Uid
			if fs.attrs && fs.ai != nil {
				d["wuid"] = fs.ai.Uid
			}
		}
		err := fs.wstat(p, d, true)
		if err == nil {
			var d zx.Dir
			d, err = fs.stat(p, false)
			if err == nil {
				rc <- d
			}
		}
		close(rc, err)
	}()
	return rc
}

func (fs *Fs) remove(p string, all bool) error {
	fs.Count(zx.Sremove)
	if fs.attrs {
		ac.sync()
	}
	p, err := zx.UseAbsPath(p)
	if err != nil {
		return err
	}
	if p == "/Ctl" || p == "/" {
		return fmt.Errorf("remove %s: %s", p, zx.ErrPerm)
	}
	if fs.zxperms {
		if err := fs.chkPut(fpath.Dir(p), false); err != nil {
			return err
		}
	}
	path := fpath.Join(fs.root, p)
	if dontremove {
		dbg.Warn("%s: dontremove: rm %s", fs.Tag, path)
		return nil
	}
	if paranoia && !strings.HasPrefix(path, "/tmp/") {
		panic("zux: trying to remove outside /tmp")
	}
	if all {
		if path == "/" || p == "/" || !strings.HasPrefix(path, fs.root) {
			return fmt.Errorf("removeall %s: too dangerous", path)
		}
		return os.RemoveAll(path)
	}
	err = os.Remove(path)
	if err != nil && zx.IsNotEmpty(err) {
		os.Remove(fpath.Join(path, AttrFile))
		os.Remove(fpath.Join(path, ".#zx")) // old attr file
		err = os.Remove(path)
	}
	return err
}

func (fs *Fs) Remove(p string) <-chan error {
	c := make(chan error, 1)
	err := fs.remove(p, false)
	c <- err
	close(c, err)
	return c
}

func (fs *Fs) RemoveAll(p string) <-chan error {
	c := make(chan error, 1)
	err := fs.remove(p, true)
	c <- err
	close(c, err)
	return c
}

func inconsistentMove(from, to string) bool {
	if from == to {
		return false
	}
	// moves from inside itself?
	// i.e. is from a prefix of to
	return zx.HasPrefix(to, from)
}

func (fs *Fs) move(from, to string) error {
	if fs.attrs {
		ac.sync()
	}
	pfrom, err := zx.UseAbsPath(from)
	if err != nil {
		return err
	}
	pto, err := zx.UseAbsPath(to)
	if err != nil {
		return err
	}
	if pfrom == pto {
		return nil
	}
	if pfrom == "/Ctl" || pfrom == "/" {
		return fmt.Errorf("move %s: %s", pfrom, zx.ErrPerm)
	}
	if pto == "/Ctl" || pto == "/" {
		return fmt.Errorf("move %s: %s", pto, zx.ErrPerm)
	}
	if fs.zxperms {
		if err := fs.chkPut(fpath.Dir(pfrom), false); err != nil {
			return err
		}
		if err := fs.chkPut(fpath.Dir(pto), false); err != nil {
			return err
		}
	}
	if inconsistentMove(from, to) {
		return fmt.Errorf("move %s: inconsistent move", from)
	}
	pathfrom := fpath.Join(fs.root, pfrom)
	pathto := fpath.Join(fs.root, pto)

	var d zx.Dir
	if fs.attrs {
		// we must move zx attributes to the new dir
		d, _ = fs.stat(from, false)
	}
	err = os.Rename(pathfrom, pathto)
	if err == nil && d != nil {
		ac.set(pathto, d)
	}
	return err
}

func (fs *Fs) Move(from, to string) <-chan error {
	c := make(chan error, 1)
	fs.Count(zx.Smove)
	err := fs.move(from, to)
	c <- err
	close(c, err)
	return c
}

func inconsistentLink(oldp, newp string) bool {
	// links back to a parent?
	// i.e. is oldp a prefix of newp
	return zx.HasPrefix(newp, oldp)
}

func (fs *Fs) link(oldp, newp string) error {
	if fs.attrs {
		ac.sync()
	}
	oldp, err := zx.UseAbsPath(oldp)
	if err != nil {
		return err
	}
	newp, err = zx.UseAbsPath(newp)
	if err != nil {
		return err
	}
	if oldp == newp {
		return fmt.Errorf("link %s: would link to self", oldp)
	}
	if oldp == "/Ctl" || oldp == "/" {
		return fmt.Errorf("link %s: %s", oldp, zx.ErrPerm)
	}
	if newp == "/Ctl" || newp == "/" {
		return fmt.Errorf("link %s: %s", newp, zx.ErrPerm)
	}
	if fs.zxperms {
		if err := fs.chkPut(fpath.Dir(newp), false); err != nil {
			return err
		}
	}
	if inconsistentLink(oldp, newp) {
		return fmt.Errorf("link %s: inconsistent link", oldp)
	}
	pathold := fpath.Join(fs.root, oldp)
	pathnew := fpath.Join(fs.root, newp)
	return os.Link(pathold, pathnew)
}

func (fs *Fs) Link(oldp, newp string) <-chan error {
	c := make(chan error, 1)
	fs.Count(zx.Slink)
	err := fs.link(oldp, newp)
	c <- err
	close(c, err)
	return c
}

// can't use ch, because it uses chan<- face{} and not chan<- []byte
func writeBytes(w io.Writer, c <-chan []byte) error {
	for b := range c {
		if _, err := w.Write(b); err != nil {
			return err
		}
	}
	return cerror(c)
}

func (fs *Fs) put(p string, d zx.Dir, off int64, c <-chan []byte) error {
	p, err := zx.UseAbsPath(p)
	if err != nil {
		return err
	}
	if p == "/Ctl" {
		return fs.putCtl(c)
	}
	mkall := false
	path := fpath.Join(fs.root, p)
	flg := os.O_RDWR // in case we resize it
	if d["type"] == "F" {
		d["type"] = "-"
		mkall = true
	} else if d["type"] == "D" {
		d["type"] = "d"
		mkall = true
	}
	if d["type"] == "-" && d["size"] == "" {
		d["size"] = "0"
	}
	mode := d.Mode()
	if d["mode"] == "" {
		if d["type"] == "d" {
			mode = 0755
		} else {
			mode = 0644
		}
	}
	if fs.attrs {
		d["wuid"] = u.Uid
		if fs.ai != nil {
			d["wuid"] = fs.ai.Uid
		}
	}
	if d["type"] != "" {
		// create or recreate
		if fs.zxperms {
			if err := fs.chkPut(fpath.Dir(p), mkall); err != nil {
				return err
			}
		} else if mkall {
			dpath := fpath.Dir(path)
			if _, err := os.Stat(dpath); zx.IsNotExist(err) {
				os.MkdirAll(dpath, 0755)
			}
		}
		if d["type"] == "d" {
			if err := os.Mkdir(path, os.FileMode(mode)); err != nil {
				if fi, nerr := os.Stat(path); nerr != nil || !fi.IsDir() {
					return err
				}
			}
			close(c, zx.ErrIsDir)
			delete(d, "size")
			if len(d) > 2 {
				fs.wstat(p, d, false)
			}
			return nil
		}
		if d["type"] != "-" {
			return zx.ErrBadType
		}
		// create or truncate file
		flg |= os.O_CREATE
	} else if fs.zxperms {
		if err := fs.chkPut(p, false); err != nil {
			return err
		}
	}
	if c == nil {
		c = make(chan []byte)
		close(c)
	}
	var sz = int64(-1)
	whence := 0
	if off < 0 {
		off = 0
		whence = 2
	}
	if d["size"] != "" {
		sz = d.Size()
		if sz == 0 {
			flg |= os.O_TRUNC
		}
	}
	fd, err := os.OpenFile(path, flg, os.FileMode(mode))
	if err != nil {
		return fmt.Errorf("put: %s", err)
	}
	defer fd.Close()
	if sz != -1 {
		if err := fd.Truncate(sz); err != nil {
			return err
		}
	}
	delete(d, "mode")
	delete(d, "size")
	fs.wstat(p, d, false)
	if off != 0 || whence != 0 {
		if _, err := fd.Seek(off, whence); err != nil {
			return err
		}
	}
	if c != nil {
		if _, ok := d["mtime"]; ok {
			mt := d.Time("mtime")
			defer os.Chtimes(path, mt, mt)
		}
		if err := writeBytes(fd, c); err != nil {
			return err
		}
	}
	return nil
}

func (fs *Fs) Put(p string, d zx.Dir, off int64, c <-chan []byte) <-chan zx.Dir {
	rc := make(chan zx.Dir)
	go func() {
		fs.Count(zx.Sput)
		d = d.SysDup()
		err := fs.put(p, d, off, c)
		if err != nil {
			close(c, err)
		}
		if err == nil {
			var d zx.Dir
			d, err = fs.stat(p, false)
			if err == nil {
				rc <- d
			}
		}
		close(rc, err)
	}()
	return rc
}

// d is a dup and can be changed.
func (fs *Fs) findr(d zx.Dir, fp *pred.Pred, p, spref, dpref string, lvl int, c chan<- zx.Dir) error {
	match, pruned, err := fp.EvalAt(d, lvl)
	// fs.Dprintf("findr at %v\n\t%v\n\t%v %v %v\n\n",
	//	d.LongFmt(), p, match, pruned, err)
	if pruned {
		if !match {
			d["proto"] = "lfs"
			d["err"] = "pruned"
		}
		c <- d
		return nil
	}
	if err != nil {
		return err
	}
	if d["rm"] != "" {
		return nil
	}
	var ds []zx.Dir
	if d["type"] == "d" {
		// GetDir will call Get and that will checkout perms
		ds, err = zx.GetDir(fs, p)
		if err != nil {
			d["err"] = err.Error()
		}
	}
	if match || err != nil {
		if ok := c <- d; !ok {
			return cerror(c)
		}
	}
	for i := 0; i < len(ds); i++ {
		cd := ds[i]
		if cd["rm"] != "" {
			continue
		}
		cp := cd["path"]
		if spref != dpref {
			cpath := cd["path"]
			suff := zx.Suffix(cpath, spref)
			if suff == "" {
				return fmt.Errorf("Y%s: %s: %s", spref, cpath, zx.ErrNotSuffix)
			}
			cd["path"] = fpath.Join(dpref, suff)
		}
		if err := fs.findr(cd, fp, cp, spref, dpref, lvl+1, c); err != nil {
			return err
		}
	}
	return nil
}

func (fs *Fs) find(p, fpred, spref, dpref string, depth int, c chan<- zx.Dir) error {
	d, err := fs.stat(p, true)
	if err != nil {
		return err
	}
	p = d["path"]
	if spref != "" || dpref != "" {
		spref, err = zx.UseAbsPath(spref)
		if err != nil {
			return err
		}
		dpref, err = zx.UseAbsPath(dpref)
		if err != nil {
			return err
		}
	}
	fp, err := pred.New(fpred)
	if err != nil {
		return err
	}
	if spref != dpref {
		suff := zx.Suffix(p, spref)
		if suff == "" {
			return fmt.Errorf("suffix %s %s: %s", spref, p, zx.ErrNotSuffix)
		}
		d["path"] = fpath.Join(dpref, suff)
	}
	return fs.findr(d, fp, p, spref, dpref, depth, c)
}

func (fs *Fs) Find(path, fpred, spref, dpref string, depth0 int) <-chan zx.Dir {
	c := make(chan zx.Dir)
	go func() {
		fs.Count(zx.Sfind)
		err := fs.find(path, fpred, spref, dpref, depth0, c)
		close(c, err)
	}()
	return c
}

func (fs *Fs) dpath(d zx.Dir) string {
	old := d["addr"]
	p := strings.LastIndexByte(old, '!')
	if p < 0 {
		p = 0
	} else {
		p++
	}
	return old[p:]
}

func (fs *Fs) FindGet(path, fpred, spref, dpref string, depth0 int) <-chan face{} {
	c := make(chan face{})
	go func() {
		dc := fs.Find(path, fpred, spref, dpref, depth0)
		for d := range dc {
			if ok := c <- d.Dup(); !ok {
				close(dc, cerror(c))
				return
			}
			if d["err"] != "" || d["type"] == "d" {
				continue
			}
			p := fs.dpath(d)
			if p == "" {
				panic("zux: bad dir addr path")
			}
			bc := fs.Get(p, 0, -1)
			for d := range bc {
				c <- d
			}
			if err := cerror(bc); err != nil {
				c <- err
			}
		}
		close(c, cerror(dc))
	}()
	return c
}
