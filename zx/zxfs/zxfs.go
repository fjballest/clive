/*
	FUSE server for zx.

	In general, fuse requests are sent directly as zx requests.
*/
package zxfs

import (
	"clive/dbg"
	fs "clive/fuse"
	"clive/mblk"
	"clive/x/bazil.org/fuse"
	"clive/zx"
	"errors"
	"fmt"
	"io"
	"os"
	fpath "path"
	"strings"
	"sync"
	"time"
)

/*
	Implementation of fuse FS for a zx.Tree

	Keeps an in-memory cache of directory entries to track
	generated inode numbers for each path/entry.
*/
struct FS {
	fs    zx.Getter
	igen  uint64
	paths map[string]*Dir
	sync.RWMutex
	isctl bool
}

/*
	Implementation of fuse Node interface
	To match unix semantics we issue many more zx calls
	than needed so that each call is made when unix expects it.
*/
struct Dir {
	fs.NodeRef
	z  *FS
	fs zx.Getter
	d  zx.Dir

	sync.Mutex
	fds []*Fd
}

/*
	Implementation of fuse handle interface.
*/
struct Fd {
	// everything kept at and locked by Dir
	*Dir
}

var (
	Verb    bool // print errors to stderr
	Debug   bool
	dprintf = dbg.FlagPrintf(&Debug)
	vprintf = dbg.FlagPrintf(&Verb)

	_xt *Dir
	_nn fs.Node        = _xt
	_nx fs.NodeXAttrer = _xt
)

func (z *FS) String() string {
	return fmt.Sprintf("fs[%s]", z.fs)
}

func (zd Dir) String() string {
	return "dir[" + zd.d["path"] + "]"
}

func (fd *Fd) String() string {
	return "fd[" + fd.d["path"] + "]"
}

func (fd *Fd) IsCtl() bool {
	if fd.z != nil && fd.z.isctl {
		return true
	}
	return fd.d["path"] == "/Ctl"
}

// Create a fuse server for fs.
func New(fs zx.Getter) (*FS, error) {
	t := &FS{
		fs:    fs,
		paths: make(map[string]*Dir, 1024),
	}
	// If it implements zx.IsCtler and IsCtl() returns true,
	// then we don't let UNIX cache things and it's likely execs will fail
	// due to data not being page aligned, but UNIX won't have problems
	// with size not reporting the actual size for files.
	t.isctl = false

	return t, nil
}

func (z *FS) mkfile(d zx.Dir) *Dir {
	z.RLock()
	path := d["path"]
	if zd, ok := z.paths[path]; ok {
		z.RUnlock()
		return zd
	}
	z.RUnlock()
	z.Lock()
	defer z.Unlock()
	z.igen++
	zd := &Dir{fs: z.fs, z: z, d: d}
	ino := z.igen
	zd.d["ino"] = fmt.Sprintf("%d", ino)
	z.paths[path] = zd
	return zd
}

func (z *FS) lookfile(path string) *Dir {
	z.RLock()
	defer z.RUnlock()
	return z.paths[path]
}

func (z *FS) rmfile(path string) {
	z.Lock()
	defer z.Unlock()
	delete(z.paths, path)
}

// Dump a debug representation of the state of z into w, with lvl indent.
func (zd *FS) DumpTo(w io.Writer, lvl int) {
	tabs := strings.Repeat("    ", lvl)
	for p, d := range zd.paths {
		fmt.Fprintf(w, "%s%s\t%s\n", tabs, p, d)
	}
}

// return the root node
func (z *FS) Root() (fs.Node, fuse.Error) {
	dc := z.fs.Stat("/")
	d := <-dc
	dprintf("%s: root dir %v\n", z, d)
	if d == nil {
		err := cerror(dc)
		vprintf("%s: root: %s\n", z, err)
		return nil, fuse.ENOENT
	}
	zd := z.mkfile(d)
	dprintf("%s: root: %v\n", z, zd.d)
	return zd, nil
}

// return the dir entry for a node
func (zd *Dir) Attr() (*fuse.Attr, fuse.Error) {
	dc := zd.fs.Stat(zd.d["path"])
	d := <-dc
	if d == nil {
		err := cerror(dc)
		if !zx.IsNotExist(err) {
			vprintf("%s: stat: %s\n", zd, err)
		} else {
			dprintf("%s: stat: %s\n", zd, err)
		}
		return nil, fuse.ENOENT
	}
	d["ino"] = zd.d["ino"]
	zd.d = d
	md := zd.d.Mode()
	m := os.FileMode(md)
	if zd.d["type"] == "d" {
		m |= os.ModeDir
	}
	ino := zd.d.Uint("ino")
	return &fuse.Attr{
		Inode: ino,
		Mode:  m,
		Size:  uint64(zd.d.Size()),
		Mtime: zd.d.Time("mtime"),
	}, nil
}

// read extended attribute.
func (zd *Dir) Xattr(name string) ([]byte, fuse.Error) {
	return nil, fuse.ENOSYS
	dprintf("%s: xattr %s\n", zd, name)
	if zd.d[name] == "" {
		vprintf("%s: xattr: %s not found\n", zd, name)
		return nil, fuse.ENODATA
	}
	return []byte(zd.d[name]), nil
}

// write extended attribute
func (zd *Dir) Wxattr(name string, v []byte) fuse.Error {
	return fuse.ENOSYS
}

// list extended attributes.
func (zd *Dir) Xattrs() []string {
	return []string{}
	var ats []string
	for k := range zd.d {
		if len(k) > 0 && k[0] >= 'A' && k[0] <= 'Z' {
			ats = append(ats, k)
		}
	}
	return ats
}

// lookup a name in a dir
func (zd *Dir) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	cpath := fpath.Join(zd.d["path"], name)
	dc := zd.fs.Stat(cpath)
	d := <-dc
	if d == nil {
		err := cerror(dc)
		if zx.IsNotExist(err) {
			dprintf("%s: lookup %s: %s\n", zd, name, err)
		} else {
			vprintf("%s: lookup %s: %s\n", zd, name, err)
		}
		return nil, fuse.ENOENT
	}
	cd := zd.z.mkfile(d)
	dprintf("%s: lookup %s: %s\n", zd, name, cd)
	return cd, nil
}

// open a dir/file
func (zd *Dir) Open(flg fuse.OpenFlags, _ fs.Intr) (fs.Handle, fuse.Error) {
	// Could re-stat the file to see if it's still ok.
	dprintf("%s: open %v\n", zd, flg)
	fd := &Fd{Dir: zd}
	zd.Lock()
	zd.fds = append(zd.fds, fd)
	zd.Unlock()
	if int(flg)&os.O_TRUNC != 0 {
		wfs, ok := zd.fs.(zx.Wstater)
		if !ok {
			vprintf("%s: O_TRUNC: not a rw tree\n", zd.d["path"])
			return fd, nil
		}
		errc := wfs.Wstat(zd.d["path"], zx.Dir{"size": "0"})
		<-errc
		if err := cerror(errc); err != nil {
			vprintf("%s: O_TRUNC: %s\n", zd.d["path"], err)
			return fd, nil
		}
		zd.Lock()
		zd.d["size"] = "0"
		zd.Unlock()
	}
	return fd, nil
}

/*
	optional fuse.Node ops
*/

// Set attributes (EPERM otherwise).
func (zd *Dir) SetAttr(r *fuse.SetattrRequest, _ fs.Intr) fuse.Error {
	nd := zx.Dir{}
	if r.Valid.Mode() {
		nd.SetMode(uint64(r.Mode))
	}
	if r.Valid.Size() && zd.d["type"] != "d" {
		nd["size"] = fmt.Sprintf("%d", r.Size)
	}
	if r.Valid.MtimeNow() {
		nd.SetTime("mtime", time.Now())
	} else if r.Valid.Mtime() {
		nd.SetTime("mtime", r.Mtime)
	}
	// For other attrs, ee bazil.org/fuse/fuse.go:/func (r *SetattrRequest) String()
	dprintf("%s: wstat %v\n", zd.d["path"], nd)
	wfs, ok := zd.fs.(zx.Wstater)
	if !ok {
		vprintf("%s: wstat: not a rw tree\n", zd.d["path"])
		return fuse.EPERM
	}
	errc := wfs.Wstat(zd.d["path"], nd)
	<-errc
	if err := cerror(errc); err != nil {
		vprintf("%s: wstat: %s\n", zd.d["path"], err)
		return fuse.EPERM
	}
	for k, v := range nd {
		zd.d[k] = v
	}
	return nil
}

func (zd *Dir) elemcall(elem string) (wfs zx.FullFs, npath string, err error) {
	if strings.Contains(elem, "/") || elem == "." || elem == ".." {
		return nil, "", fmt.Errorf("bad element name '%s", elem)
	}
	np := fpath.Join(zd.d["path"], elem)
	wfs, ok := zd.fs.(zx.FullFs)
	if !ok {
		return nil, "", fmt.Errorf("not a full tree")
	}
	return wfs, np, nil
}

// Remove (EPERM otherwise)
func (zd *Dir) Remove(elem string, _ fs.Intr) fuse.Error {
	dprintf("%s: remove %s\n", zd.d["path"], elem)
	wfs, np, err := zd.elemcall(elem)
	if err != nil {
		vprintf("%s: remove: %s\n", zd.d["path"], err)
		return fuse.EPERM
	}

	errc := wfs.Remove(np)
	if err := <-errc; err != nil {
		vprintf("%s: remove: %s\n", zd.d["path"], err)
		return fuse.EPERM
	}
	zd.z.rmfile(np)
	return nil
}

func (zd *Dir) Mkdir(elem string, mode os.FileMode, intr fs.Intr) (fs.Node, fuse.Error) {
	dprintf("%s: mkdir %s %v\n", zd.d["path"], elem, mode)
	wfs, np, err := zd.elemcall(elem)
	if err != nil {
		vprintf("%s: mkdir: %s\n", zd.d["path"], err)
		return nil, fuse.EPERM
	}
	nd := zx.Dir{"type": "d"}
	nd.SetMode(uint64(mode))
	errc := wfs.Put(np, nd, 0, nil)
	<-errc
	if err := cerror(errc); err != nil {
		vprintf("%s: mkdir: %s\n", zd.d["path"], err)
		return nil, fuse.EPERM
	}
	return zd.Lookup(elem, intr)
}

func (zd *Dir) PutNode() {
	dprintf("%s: put back\n", zd.d["path"])
	zd.z.rmfile(zd.d["path"])
}

func (fd *Fd) PutHandle() {
	fd.Dir.putfd(fd)
}

func (szd *Dir) Rename(oelem, nelem string, tond fs.Node, _ fs.Intr) fuse.Error {
	dzd, ok := tond.(*Dir)
	if !ok {
		vprintf("%s: mv: not a Dir??\n", szd.d["path"])
		return fuse.EPERM
	}
	dprintf("%s: mv %s -> %s %s \n", szd.d["path"], oelem, dzd.d["path"], nelem)
	wfs, op, err := szd.elemcall(oelem)
	if err != nil {
		vprintf("%s: mv: %s\n", szd.d["path"], err)
		return fuse.EPERM
	}
	np := fpath.Join(dzd.d["path"], nelem)

	dprintf("%s: mv to %s...\n", op, np)
	errc := wfs.Move(op, np)
	if err := <-errc; err != nil {
		vprintf("%s: mv: %s\n", szd.d["path"], err)
		return fuse.EPERM
	}
	// RACE here: other does a lookup before we update our attr cache
	dc := wfs.Stat(np)
	d := <-dc
	if d == nil {
		vprintf("%s: mv %s\n", szd.d["path"], cerror(dc))
		return nil
	}
	dprintf("%s: mv %s ok \n", szd.d["path"], oelem)
	szd.z.Lock()
	defer szd.z.Unlock()
	delete(szd.z.paths, np)
	for p, x := range szd.z.paths {
		if zx.HasPrefix(p, op) {
			suff := zx.Suffix(p, op)
			newp := fpath.Join(np, suff)
			dprintf("cached path %s -> %s\n", p, newp)
			delete(szd.z.paths, p)
			if x.d != nil {
				x.d["path"] = newp
			}
			szd.z.paths[newp] = x
		}
	}
	return nil
}

func (zd *Dir) dirent() fuse.Dirent {
	d := zd.d
	t := fuse.DT_File
	if d["type"] == "d" {
		t = fuse.DT_Dir
	}
	dent := fuse.Dirent{
		Inode: d.Uint("ino"),
		Name:  d["name"],
		Type:  t,
	}
	if dent.Name == "/" {
		dent.Name = "dev"
	}
	return dent
}

func (zd *Dir) Create(elem string, flg fuse.OpenFlags, mode os.FileMode, intr fs.Intr) (fs.Node, fs.Handle, fuse.Error) {
	dprintf("%s: create %s %v %v\n", zd.d["path"], elem, flg, mode)
	wfs, np, err := zd.elemcall(elem)
	if err != nil {
		vprintf("%s: create %s: %s\n", zd.d["path"], elem, err)
		return nil, nil, fuse.EPERM
	}

	// Create may issue the request and then wait until further writes to issue a new put
	// to update the file, or it might collect all the data and wait until close, or it
	// might issue a single put and keep on streaming data during further writes.
	//
	// By now we keep as close to unix as we can.

	nd := zx.Dir{"type": "-", "size": "0"}
	nd.SetMode(uint64(mode))
	dc := make(chan []byte)
	close(dc)
	xc := wfs.Put(np, nd, 0, dc)
	<-xc
	if err := cerror(xc); err != nil {
		vprintf("%s: create %s: %s\n", zd.d["path"], elem, err)
		return nil, nil, fuse.EPERM
	}
	// Shouldn't put return the new stat?
	x, err := zd.Lookup(elem, intr)
	if err != nil {
		vprintf("%s: create %s: %s\n", zd.d["path"], elem, err)
		return nil, nil, err
	}
	xd := x.(*Dir)
	h := &Fd{Dir: xd}
	zd.Lock()
	xd.fds = append(xd.fds, h)
	zd.Unlock()
	return x, h, err
}

func (fd *Fd) ReadDir(intr fs.Intr) ([]fuse.Dirent, fuse.Error) {
	ds, err := zx.GetDir(fd.fs, fd.d["path"])
	if err != nil {
		vprintf("%s: readdir: %s\n", fd, err)
		return nil, fuse.EPERM
	}
	dents := make([]fuse.Dirent, len(ds))
	for i := 0; i < len(ds); i++ {
		cd := fd.z.mkfile(ds[i])
		dents[i] = cd.dirent()
	}
	return dents, nil
}

func (zd *Dir) putfd(fd *Fd) error {
	dprintf("%s: put fd back\n", zd.d["path"])
	zd.Lock()
	defer zd.Unlock()

	for i := 0; i < len(zd.fds); i++ {
		if zd.fds[i] == fd {
			if i < len(zd.fds)-1 {
				zd.fds[i] = zd.fds[len(zd.fds)-1]
			}
			zd.fds = zd.fds[:len(zd.fds)-1]
			return nil
		}
	}
	return errors.New("putfd: no such fd")
}

func (fd *Fd) Close(_ fs.Intr) fuse.Error {
	dprintf("%s: close\n", fd)
	zd := fd.Dir

	err := zd.putfd(fd)
	// err is a placeholder for later
	if err != nil && false {
		vprintf("%s: close: write error: %s\n", fd, err)
		return fuse.EPERM
	}
	return nil
}

func (fd *Fd) Read(off int64, sz int, i fs.Intr) ([]byte, fuse.Error) {
	dprintf("%s: read %d #%d\n", fd, off, sz)
	d := fd.d
	if d["type"] == "d" {
		// shouldn't happen.
		vprintf("%s: read: not a file\n", fd)
		return nil, fuse.EPERM
	}
	if sz <= 0 {
		return []byte{}, nil
	}
	zd := fd.Dir
	zd.Lock()
	rdata := []byte{}
	datac := fd.fs.Get(d["path"], off, int64(sz))
	zd.Unlock()
	for {
		data := <-datac
		if len(data) == 0 {
			err := cerror(datac)
			if err != nil {
				vprintf("%s: read %d: %s\n", fd, off, err)
			} else {
				dprintf("%s: read %d: #%d\n", fd, off, len(rdata))
			}
			if len(data) > 0 {
				err = nil
			}
			close(datac)
			return rdata, err
		}
		rdata = append(rdata, data...)
	}
}

var fuseflags = &zx.Flags{}

func init() {
	fuseflags.Add("fdebug", &Debug)
	fuseflags.Add("fverb", &fs.Debug)
}

func fuseCtl(data []byte) bool {
	s := string(data)
	if !strings.Contains(s, "fdebug") && !strings.Contains(s, "fverb") {
		return false
	}
	s = strings.TrimSpace(s)
	err := fuseflags.Ctl(s) // and ignore errors
	dbg.Warn("***** fuse ctl %s sts %v", s, err)
	Verb = fs.Debug
	if !Debug {
		Verb = false
		fs.Debug = false
	}
	return true
}

// TODO:
// when we start a write, keep the chan open after the write, receiving async errors
// from previous writes, and check the write offset.
// Any other operation including moves and lookups involving this file or the parent
// must close the write chan and await until the put completes.
// If there's no futher operation in, say, 1s, the put must be closed and completed.
// This is good to coallesce multiple writes into a single put.
// Speed seems to be ok
func (fd *Fd) Write(data []byte, off int64, _ fs.Intr) (int, fuse.Error) {
	sz := len(data)
	dprintf("%s: write %d #%d\n", fd, off, sz)
	wfs, ok := fd.fs.(zx.Putter)
	if !ok {
		vprintf("%s: write: not a rw tree\n", fd)
		return 0, fuse.EPERM
	}
	fd.Lock()
	defer fd.Unlock()
	d := fd.d
	if d["path"] == "/Ctl" {
		if fuseCtl(data) {
			return len(data), nil
		}
	}
	if d["type"] == "d" {
		// shouldn't happen.
		vprintf("%s: write: not a file\n", fd)
		return 0, fuse.EPERM
	}
	wc := make(chan []byte, 16)
	wec := wfs.Put(d["path"], nil, off, wc)
	tot := 0
	for len(data) > 0 {
		n := len(data)
		if n > mblk.Size {
			n = mblk.Size
		}
		wc <- data[:n]
		tot += n
		data = data[n:]
	}
	close(wc)
	<-wec
	err := cerror(wec)
	if err != nil {
		vprintf("%s: write %d: %s\n", fd, off, err)
		return 0, fuse.EPERM
	}
	dprintf("%s: pwrite at %d: ok\n", fd, off)
	return tot, nil
}
