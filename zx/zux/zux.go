/*
	ZX for UX file systems
*/
package zux

import (
	"sync"
	"os/user"
	"strings"
	"strconv"
	"syscall"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	fpath "path"
	"path/filepath"
	"clive/ch"
	"clive/dbg"
	"clive/u"
	"clive/zx"
	"clive/zx/pred"
)

type Fs struct {
	*dbg.Flag
	*zx.Flags
	root	string
	rdonly	bool
}

var ctldir = zx.Dir {
	"name":  "Ctl",
	"path":  "/Ctl",
	"addr": "lfs!local!/Ctl",
	"mode":  "0644",
	"size":  "0",
	"mtime": "0",
	"type":  "c",
	"uid":   u.Uid,
	"gid":   u.Uid,
	"wuid":  u.Uid,
}

func new(root string, rdonly bool) (*Fs, error) {
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
		root:   p,
		rdonly: rdonly,
		Flag:   &dbg.Flag{Tag: tag},
		Flags:  &zx.Flags{},
	}
	fs.Flags.Add("debug", &fs.Debug)
	fs.Flags.AddRO("rdonly", &fs.rdonly)
	return fs, nil
}

// Return a new Fs rooted at the given unix dir
func New(root string) (*Fs, error) {
	return new(root, false)
}

func NewRO(root string) (*Fs, error) {
	return new(root, true)
}

var (
	uids = map[uint32] string{}
	uidslk sync.Mutex

	dontremove bool	// set during testing to prevent removes
)

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

func (fs *Fs) stat(p string) (zx.Dir, error) {
	p, err := zx.UseAbsPath(p)
	if err != nil {
		return nil, err
	}
	if p == "/Ctl" {
		return ctldir.Dup(), nil
	}
	path := fpath.Join(fs.root, p)
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	d := newDir(st)
	d["path"] = p
	d["addr"] = "lfs!local!" + path
	if p == "/" {
		d["name"] = "/"
	}
	return d, nil
}

func (fs *Fs) Stat(p string) chan zx.Dir {
	c := make(chan zx.Dir, 1)
	d, err := fs.stat(p)
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
		}
		rr := io.LimitReader(fd, count)
		return readBytes(rr, dc)
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
			if ok := dc <- ctldir.Bytes(); !ok {
				return cerror(dc)
			}
			// but not i++
			continue
		}
		fi := ds[i]
		if fi.Name() == AttrFile {
			if i == len(ds)-1 {
				break
			}
			ds = append(ds[:i], ds[i+1:]...)
			fi = ds[i]
		}
		d := newDir(fi)
		cp :=fpath.Join(p, fi.Name())
		cpath :=fpath.Join(path, fi.Name())
		d["path"] = cp
		d["addr"] = "lfs!local!" + cpath
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

func (fs *Fs) wstat(p string, d zx.Dir) error {
	if fs.rdonly {
		return fmt.Errorf("%s: %s", fs.Tag, zx.ErrRO)
	}
	p, err := zx.UseAbsPath(p)
	if err != nil {
		return err
	}
	path := fpath.Join(fs.root, p)
	if _, ok := d["size"]; ok {
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
	return err
}

func (fs *Fs) Wstat(p string, d zx.Dir) chan zx.Dir {
	rc := make(chan zx.Dir)
	go func() {
		err := fs.wstat(p, d)
		if err == nil {
			var d zx.Dir
			d, err = fs.stat(p)
			if err == nil {
				rc <- d
			}
		}
		close(rc, err)
	}()
	return rc
}

func (fs *Fs) remove(p string, all bool) error {
	if fs.rdonly {
		return fmt.Errorf("%s: %s", fs.Tag, zx.ErrRO)
	}
	p, err := zx.UseAbsPath(p)
	if err != nil {
		return err
	}
	if p == "/Ctl" || p == "/" {
		return fmt.Errorf("remove %s: %s", p, zx.ErrPerm)
	}
	path := fpath.Join(fs.root, p)
	if dontremove {
		dbg.Warn("%s: dontremove: rm %s", fs.Tag, path)
		return nil
	}
	if all {
		if path == "/" || p == "/" || !strings.HasPrefix(path, fs.root) {
			return fmt.Errorf("removeall %s: too dangerous", path)
		}
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

func (fs *Fs) Remove(p string) chan error {
	c := make(chan error, 1)
	err := fs.remove(p, false)
	c <- err
	close(c, err)
	return c
}

func (fs *Fs) RemoveAll(p string) chan error {
	c := make(chan error, 1)
	err := fs.remove(p, true)
	c <- err
	close(c, err)
	return c
}

func (fs *Fs) move(from, to string) error {
	if fs.rdonly {
		return fmt.Errorf("%s: %s", fs.Tag, zx.ErrRO)
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
	pathfrom := fpath.Join(fs.root, pfrom)
	pathto := fpath.Join(fs.root, pto)
	return os.Rename(pathfrom, pathto)
}

func (fs *Fs) Move(from, to string) chan error {
	c := make(chan error, 1)
	err := fs.move(from, to)
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
	if fs.rdonly {
		return fmt.Errorf("%s: %s", fs.Tag, zx.ErrRO)
	}
	p, err := zx.UseAbsPath(p)
	if err != nil {
		return err
	}
	if p == "/Ctl" {
		return fs.putCtl(c)
	}
	path := fpath.Join(fs.root, p)
	flg := os.O_RDWR // in case we resize it
	mode := d.Mode()
	if d["mode"] == "" {
		if d["type"] == "d" {
			mode = 0755
		} else {
			mode = 0644
		}
	}
	if d["type"] != "" {
		// create or truncate dir
		if d["type"] == "d" {
			if err := os.MkdirAll(path, os.FileMode(mode)); err != nil {
				if !zx.IsExists(err) {
					return err
				}
			}
			close(c, zx.ErrIsDir)
			if len(d) > 2 {
				fs.wstat(p, d)
			}
			return nil
		}
		if d["type"] != "-" {
			return zx.ErrBadType
		}
		// create or truncate file
		flg |= os.O_CREATE
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
		return err
	}
	defer fd.Close()
	if sz != -1 {
		if err := fd.Truncate(sz); err != nil {
			return err
		}
	}
	if off != 0 || whence != 0 {
		if _, err := fd.Seek(off, whence); err != nil {
			return err
		}
	}
	if c != nil {
		if err := writeBytes(fd, c); err != nil {
			return err
		}
	}
	// BUG: should also update other attributes (besides size, mode, mtime)
	if _, ok := d["mode"]; ok {
		mode := d.Mode()
		os.Chmod(path, os.FileMode(mode))
	}
	if _, ok := d["mtime"]; ok {
		mt := d.Time("mtime")
		os.Chtimes(path, mt, mt)
	}
	return nil
}

func (fs *Fs) Put(p string, d zx.Dir, off int64, c <-chan []byte) chan zx.Dir {
	rc := make(chan zx.Dir)
	go func() {
		err := fs.put(p, d, off, c)
		if err != nil {
			close(c, err)
		}
		if err == nil {
			var d zx.Dir
			d, err = fs.stat(p)
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
	fs.Dprintf("findr at %v\n\t%v\n\t%v %v %v\n\n", d.LongFmt(), p, match, pruned, err)
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
	d, err := fs.stat(p)
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
		err := fs.find(path, fpred, spref, dpref, depth0, c)
		close(c, err)
	}()
	return c
}

func (fs *Fs) FindGet(path, fpred, spref, dpref string, depth0 int) <-chan interface{} {
	c := make(chan interface{})
	go func() {
		dc := fs.Find(path, fpred, spref, dpref, depth0)
		for d := range dc {
			if ok := c <- d; !ok {
				close(dc, cerror(c))
				return
			}
			if d["err"] != "" || d["type"] != "-" {
				continue
			}
			toks := strings.Split(d["addr"], "!")
			if len(toks) < 3 {
				panic("zux: bad dir addr")
			}
			p := zx.Suffix(toks[2], fs.root)
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
