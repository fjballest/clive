package ns

import (
	"clive/zx"
	"clive/zx/zux"
	"clive/zx/rzx"
	"clive/net/auth"
	fpath "path"
	"sync"
	"io"
	"fmt"
	"strings"
)

var (
	lfs = map[string]zx.Fs{}
	lfslk sync.Mutex

	_fs zx.RWFs = &NS{}
	_fs2 zx.Finder = &NS{}
	_fs3 zx.FindGetter = &NS{}
)

// Add the given (absolute) paths as valid paths to resolve lfs addresses.
// If the path is not ok, it's a panic.
// DirFs relies on this to resolve addresses of the form lfs!*
// and the longest path added is used.
// If fs is not given, a default zux tree is made for it and it's a panic if
// we fail to make fs.
func AddLfsPath(path string, fs zx.Fs) {
	path, err := zx.UseAbsPath(path)
	if err != nil {
		panic(err)
	}
	if fs == nil {
		fs, err = zux.NewZX(path)
		if err != nil {
			panic(err)
		}
	}
	lfslk.Lock()
	defer lfslk.Unlock()
	addr := path
	if ofs, ok := lfs[addr]; ok {
		if cfs, ok := ofs.(io.Closer); ok {
			cfs.Close()
		}
	}
	lfs[addr] = fs
}

// Return the best lfs added for the given path, its root, and the relative path for path
// Returns nil if there's no such tree
func Lfs(path string) (zx.Fs, string, string) {
	lfslk.Lock()
	defer lfslk.Unlock()
	var bp, rel, root string
	path = fpath.Clean(path)
	for p, _ := range lfs {
		if len(p) > len(bp) && zx.HasPrefix(path, p) {
			bp = p
			rel = zx.Suffix(path, p)
			root = p
		}
	}
	return lfs[bp], root, rel
}

// Dial the server for this dir (if not already dialed) and return it,
// the dir addr is updated.
func DirFs(d zx.Dir) (zx.Fs, error) {
	switch p := d.Proto(); p {
	case "lfs":
		addr := d["addr"]
		toks := strings.Split(d["addr"], "!")	// lfs!root!/path
		if len(toks) != 3 {
			return nil, fmt.Errorf("ns: no zux tree for addr %q", addr)
		}
		fullpath := fpath.Join(toks[1], toks[2])
		fs, root, rel := Lfs(fullpath)
		if fs == nil {
			return nil, fmt.Errorf("ns: no zux tree for addr %q", addr)
		}
		d["addr"] = "lfs!" + root + "!" + rel
		return fs, nil
	case "zx":
		addr := d.SAddr()
		if len(addr) < 3 {
			panic("DirFs bug")
		}
		addr = addr[3:]	// remove zx!
		// rzx does cache dials, no need to do it again here.
		return rzx.Dial(addr, auth.TLSclient)
	default:
		return nil, fmt.Errorf("ns: no tree for addr %q", d["addr"])
	}
}

func cerr(err error) <-chan []byte {
	c := make(chan []byte)
	close(c, err)
	return c
}

func derr(err error) <-chan zx.Dir {
	c := make(chan zx.Dir)
	close(c, err)
	return c
}

func rerr(err error) <-chan error {
	c := make(chan error, 1)
	c <- err
	close(c, err)
	return c
}

func (ns *NS) Stat(path string) <-chan zx.Dir {
	_, ds, err := ns.Resolve(path)
	if err != nil {
		return derr(err)
	}
	d := ds[0]
	if d["addr"] == "" {
		c := make(chan zx.Dir, 1)
		c <- d
		close(c)
		return c
	}
	fs, err := DirFs(d)
	if err != nil {
		return derr(err)
	}
	return fs.Stat(d.SPath())
}

func (ns *NS) Get(path string, off, count int64) <-chan []byte {
	_, ds, err := ns.Resolve(path)
	if err != nil {
		return cerr(err)
	}
	d := ds[0]
	fs, err := DirFs(d)
	if err != nil {
		return cerr(err)
	}
	xfs, ok := fs.(zx.Getter)
	if !ok {
		return cerr(fmt.Errorf("%s: tree is not a getter"))
	}
	return xfs.Get(d.SPath(), off, count)
}

// On unions, the first entry is always used.
func (ns *NS) Put(path string, ud zx.Dir, off int64, dc <-chan []byte) <-chan zx.Dir {
	_, ds, err := ns.Resolve(path)
	if err != nil {
		close(dc, err)
		return derr(err)
	}
	d := ds[0]
	fs, err := DirFs(d)
	if err != nil {
		close(dc, err)
		return derr(err)
	}
	xfs, ok := fs.(zx.Putter)
	if !ok {
		close(dc, err)
		return derr(fmt.Errorf("%s: tree is not a putter"))
	}
	return xfs.Put(d.SPath(), ud, off, dc)
}

func (ns *NS) Wstat(path string, ud zx.Dir) <-chan zx.Dir {
	_, ds, err := ns.Resolve(path)
	if err != nil {
		return derr(err)
	}
	d := ds[0]
	fs, err := DirFs(d)
	if err != nil {
		return derr(err)
	}
	xfs, ok := fs.(zx.Wstater)
	if !ok {
		return derr(fmt.Errorf("%s: tree is not a wstater"))
	}
	return xfs.Wstat(d.SPath(), ud)
}

func (ns *NS) Remove(path string) <-chan error {
	_, ds, err := ns.Resolve(path)
	if err != nil {
		return rerr(err)
	}
	d := ds[0]
	fs, err := DirFs(d)
	if err != nil {
		return rerr(err)
	}
	xfs, ok := fs.(zx.Remover)
	if !ok {
		return rerr(fmt.Errorf("%s: tree is not a remover"))
	}
	return xfs.Remove(d.SPath())
}

func (ns *NS) RemoveAll(path string) <-chan error {
	_, ds, err := ns.Resolve(path)
	if err != nil {
		return rerr(err)
	}
	d := ds[0]
	fs, err := DirFs(d)
	if err != nil {
		return rerr(err)
	}
	xfs, ok := fs.(zx.Remover)
	if !ok {
		return rerr(fmt.Errorf("%s: tree is not a remover"))
	}
	return xfs.RemoveAll(d.SPath())
}

func (ns *NS) Move(from, to string) <-chan error {
	_, fromds, err := ns.Resolve(from)
	if err != nil {
		return rerr(err)
	}
	fromd := fromds[0]
	fromfs, err := DirFs(fromd)
	if err != nil {
		return rerr(err)
	}
	_, tods, err := ns.Resolve(to)
	if err != nil {
		return rerr(err)
	}
	tod := tods[0]
	if fromd.SAddr() != tod.SAddr() {
		return rerr(fmt.Errorf("%s: cross device move", from))
	}
	xfs, ok := fromfs.(zx.Mover)
	if !ok {
		return rerr(fmt.Errorf("%s: tree is not a mover", from))
	}
	return xfs.Move(fromd.SPath(), tod.SPath())
}

