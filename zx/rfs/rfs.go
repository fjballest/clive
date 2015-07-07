/*
	Rfs provides remote access to remote Finders and file Trees.

	It imports a remote zx.Tree or RWTree and provides the ns.Finder and
	the zx.RWTree interfaces.
*/
package rfs

import (
	"clive/dbg"
	"clive/nchan"
	"clive/net/auth"
	"clive/net/ds"
	"clive/zx"
	"clive/zx/lfs"
	"errors"
	"fmt"
	"strings"
	"sync"
	"os"
	"path/filepath"
	"time"
)

/*
	A remote file system.

	See zx.Find and zx.RWTree for a descrition of the methods implemented.
	Beware that calling methods in a given order does not mean that
	requests go out in that order.
*/
type Rfs  {
	c       *nchan.Mux
	Tag     string // usually the address of the server.
	addr    string

	*zx.Flags
}

var (
	rems   = map[string]*Rfs{}
	remslk sync.RWMutex

	VerbDebug bool

	// make sure we implement the right interfaces
	_fs *Rfs
	_t  zx.RWTree = _fs
)

func init() {
	zx.DefProto("zx", dial)
}

func dial(d zx.Dir) (zx.Tree, error) {
	addr := d["addr"]
	if addr == "" {
		return nil, errors.New("no address")
	}
	spath := d["spath"]
	if spath == "" {
		spath = d["path"]
	}
	if spath == "" {
		return nil, errors.New("no server path")
	}

	remslk.RLock()
	r := rems[addr]
	remslk.RUnlock()
	if r != nil {
		return r, nil
	}

	remslk.Lock()
	defer remslk.Unlock()
	toks := strings.Split(addr, "!")
	tree := "main"
	if len(toks) >= 4 {
		tree = toks[3]
	}
	c, err := ds.Dial(addr)
	if err != nil {
		return nil, err
	}
	if _, err := auth.AtClient(c, "", "zx"); err!=nil && err!=auth.ErrDisabled {
		close(c.In, err)
		close(c.Out, err)
		return nil, err
	}
	m := nchan.NewMux(c, true)
	r, err = New(m, tree)
	if err != nil {
		return nil, err
	}
	r.addr = addr
	hc := m.Hup()
	go func() {
		<-hc
		dbg.Warn("rfs hangup on %s", addr)
		r.gone()
	}()
	rems[addr] = r
	return r, nil
}

func (t *Rfs) gone() {
	remslk.Lock()
	defer remslk.Unlock()
	if rems[t.addr] == t {
		delete(rems, t.addr)
	}
}

func (t *Rfs) Name() string {
	return t.Tag
}

func (t *Rfs) dprintf(st string, args ...interface{}) {
	if t != nil && t.Dbg {
		fmt.Fprintf(os.Stderr, t.Tag + ": " + st, args...)
	}
}

/*
	NB: We might perhaps register closers to undo the imports made
	before the process exists.
	But, for unix processes, closing all the descriptors as part of the
	process exiit will close all connections.
	However in native clive this will be a BUG.
*/

/*
	Import the remote ZX server at the given address.
	This performs authentication if auth.Enabled and honors TLS configuration.
	Addresses of the form "*!*!lfs!tree!/a/b" are understood as a request to build
	a local fs at the given path.
	An address referring to a existing dir path is also used to build a local fs.
*/
func Import(addr string) (zx.RWTree, error) {
	if fi, err := os.Stat(addr); err == nil && fi.IsDir(){
		dir, err := filepath.Abs(addr)
		if err != nil {
			return nil, err
		}
		fs, err := lfs.New(addr, dir, lfs.RW)
		if err == nil {
			fs.SaveAttrs(true)
		}
		return fs, err
	}
	toks := strings.Split(addr, "!")
	if len(toks) < 2 {
		return nil, errors.New("bad address")
	}
	if len(toks) < 3 {
		toks = append(toks, "zx")
		addr = strings.Join(toks, "!")
	}
	if toks[2] == "lfs" {
		p := "/"
		if len(toks) >= 5 {
			p = toks[4]
		}
		fs, err := lfs.New(addr, p, lfs.RW)
		if err == nil {
			fs.SaveAttrs(true)
		}
		return fs, err
	}
	tree := "main"
	if len(toks) >= 4 {
		tree = toks[3]
	}
	c, err := ds.Dial(addr)
	if err != nil {
		return nil, err
	}
	if _, err := auth.AtClient(c, "", "zx"); err!=nil && err!=auth.ErrDisabled {
		err = fmt.Errorf("auth: %s", err)
		close(c.In, err)
		close(c.Out, err)
		return nil, err
	}
	return New(nchan.NewMux(c, true), tree)
}

/*
	Return the named Tree (RWTree) reached through c
	The connection mux should no longer be used by the caller.
	Import is usually preferred.
*/
func New(c *nchan.Mux, name string) (*Rfs, error) {
	t := &Rfs{
		c:   c,
		Tag: c.Tag,
		Flags: &zx.Flags{},
	}
	t.addr = c.Tag
	if c.Tag == "" {
		c.Tag = "rfs"
	}
	if name!="" && name!="main" {
		err := <-t.Fsys(name)
		if err != nil {
			return nil, fmt.Errorf("fsys %s: %s", name, err)
		}
	}
	return t, nil
}

// TODO: Add a Redial() method, that takes t.addr and re-dials it.

/*
	Cease operation for t.
*/
func (t *Rfs) Close(e error) {
	t.gone()
	t.c.Close(e)
}

func isHangup(err error) bool {
	return err != nil && strings.Contains(err.Error(), "operation timed out")
}

var ErrHangUp = errors.New("network i/o error")

// Issue a Stat for / each ival, for use with server zx keep alives.
func (t *Rfs) Pings(ival time.Duration) {
	go func() {
		for {
			time.Sleep(ival)
			rc := t.Stat("/")
			select {
			case <-time.After(ival):
				t.Close(errors.New("ping time out"))
				return
			case d := <-rc:
				if d == nil {
					t.Close(errors.New("ping time out"))
					return
				}
			}
		}
	}()
}

func (t *Rfs) Stat(rid string) chan zx.Dir {
	t.c.Debug = t.Dbg && VerbDebug
	c := make(chan zx.Dir, 1)
	msg := &Msg{Op: Tstat, Rid: rid}
	t.dprintf("stat %s\n", rid)
	go func() {
		reqc, repc := t.c.Rpc()
		raw := msg.Pack()
		cs := t.IOstats.NewCallSize(zx.Sstat, len(raw))
		reqc <- raw
		close(reqc)
		for rep := range repc {
			cs.Send(int64(len(rep)))
			d, _, _ := zx.UnpackDir(rep)
			if d != nil {
				d["addr"] = t.addr
				d["proto"] = "zx"
			}
			c <- d
		}
		err := cerror(repc)
		cs.End(err != nil)
		if err != nil {
			if isHangup(err) {
				t.Close(ErrHangUp)
			}
			t.dprintf("stat %s: %s\n", rid, err)
		} else {
			t.dprintf("stat %s: ok\n", rid)
		}
		close(c, err)
	}()
	return c
}

func (t *Rfs) getrpc(rid string, msg *Msg) <-chan []byte {
	t.c.Debug = t.Dbg && VerbDebug
	c := make(chan []byte, 10)
	go func() {
		reqc, repc := t.c.Rpc()
		raw := msg.Pack()
		cs := t.IOstats.NewCallSize(zx.Sget, len(raw))
		reqc <- raw
		close(reqc)
		some := false
		for rep := range repc {
			cs.Send(int64(len(rep)))
			if ok := c <- rep; !ok {
				close(repc, cerror(c))
				break
			}
			some = true
			t.dprintf("get %s: ok\n", rid)
		}
		if !some {
			c <- nil
		}
		err := cerror(repc)
		cs.End(err != nil)
		if err != nil {
			if isHangup(err) {
				t.Close(ErrHangUp)
			}
			t.dprintf("get %s: %s\n", rid, err)
		} else {
			t.dprintf("get %s: ok\n", rid)
		}
		close(c, err)
	}()
	return c
}

func (t *Rfs) Get(rid string, off, count int64, pred string) <-chan []byte {
	t.dprintf("get %s %d %d %q\n", rid, off, count, pred)
	msg := &Msg{Op: Tget, Rid: rid, Off: off, Count: count, Pred: pred}
	return t.getrpc(rid, msg)
}

func (t *Rfs) putrpc(rid string, msg *Msg, dc <-chan []byte) chan zx.Dir {
	t.c.Debug = t.Dbg && VerbDebug
	c := make(chan zx.Dir, 1)
	go func() {
		reqc, repc := t.c.Rpc()
		raw := msg.Pack()
		cs := t.IOstats.NewCallSize(zx.Sput, len(raw))
		reqc <- raw
		nm := 0
		tot := 0
		for m := range dc {
			nm++
			cs.Recv(int64(len(m)))
			if ok := reqc <- m; !ok {
				err := cerror(reqc)
				close(dc, err)
				close(c, err)
				cs.End(err != nil)
				return
			}
			if len(m) == 0 {
				break
			}
			tot += len(m)
		}
		close(reqc, cerror(dc))
		rep := <-repc
		var err error
		var d zx.Dir
		if len(rep) == 0 {
			err = cerror(repc)
		} else {
			d, _, err = zx.UnpackDir(rep)
			if err==nil && d==nil {
				err = errors.New("null dir in put reply")
			}
		}
		cs.End(err != nil)
		if err == nil {
			t.dprintf("put %s: %s\n", rid, d)
			c <- d
		} else {
			if isHangup(err) {
				t.Close(ErrHangUp)
			}
			t.dprintf("put %s: %s\n", rid, err)
		}
		close(c, err)
	}()
	return c
}

func (t *Rfs) Put(rid string, d zx.Dir, off int64, dc <-chan []byte, pred string) chan zx.Dir {
	t.dprintf("put %s %v %d '%s'\n", rid, d, off, pred)
	msg := &Msg{Op: Tput, Rid: rid, D: d, Off: off, Pred: pred}
	return t.putrpc(rid, msg, dc)
}

func (t *Rfs) errrpc(rid, op string, msg *Msg, k int) chan error {
	t.c.Debug = t.Dbg && VerbDebug
	c := make(chan error, 1)
	go func() {
		reqc, repc := t.c.Rpc()
		raw := msg.Pack()
		var cs *zx.CallStat
		if k >= 0 {
			cs = t.IOstats.NewCallSize(k, len(raw))
		}
		reqc <- raw
		close(reqc)
		<-repc
		err := cerror(repc)
		cs.End(err != nil)
		if err != nil {
			if isHangup(err) {
				t.Close(ErrHangUp)
			}
			t.dprintf("%s %s: %s\n", op, rid, err)
		} else {
			t.dprintf("%s %s: ok\n", op, rid)
		}
		c <- err
		close(c, err)
		cs.End(err != nil)
	}()
	return c
}

func (t *Rfs) Fsys(name string) <-chan error {
	t.dprintf("fsys %s\n", name)
	msg := &Msg{Op: Tfsys, Rid: name}
	return t.errrpc(name, "fsys", msg, -1)
}

func (t *Rfs) Mkdir(rid string, d zx.Dir) chan error {
	msg := &Msg{Op: Tmkdir, Rid: rid, D: d}
	t.dprintf("mkdir %s %v\n", rid, d)
	return t.errrpc(rid, "mkdir", msg, zx.Smkdir)
}

func (t *Rfs) Move(from, to string) chan error {
	t.dprintf("move %s %s \n", from, to)
	msg := &Msg{Op: Tmove, Rid: from, To: to}
	return t.errrpc(from, "move", msg, zx.Smove)
}

func (t *Rfs) Remove(rid string) chan error {
	t.dprintf("remove %s\n", rid)
	msg := &Msg{Op: Tremove, Rid: rid}
	return t.errrpc(rid, "remove", msg, zx.Sremove)
}

func (t *Rfs) RemoveAll(rid string) chan error {
	t.dprintf("removeall %s\n", rid)
	msg := &Msg{Op: Tremoveall, Rid: rid}
	return t.errrpc(rid, "removeall", msg, zx.Sremoveall)
}

func (t *Rfs) Wstat(rid string, d zx.Dir) chan error {
	t.dprintf("wstat %s %v\n", rid, d)
	msg := &Msg{Op: Twstat, Rid: rid, D: d}
	return t.errrpc(rid, "wstat", msg, zx.Swstat)
}

func (t *Rfs) Find(rid, pred, spref, dpref string, depth int) <-chan zx.Dir {
	t.dprintf("find %s '%s' '%s' '%s' %d\n", rid, pred, spref, dpref, depth)
	dc := make(chan zx.Dir)
	msg := &Msg{Op: Tfind, Rid: rid, Pred: pred, Spref: spref, Dpref: dpref, Depth: depth}
	go func() {
		reqc, repc := t.c.Rpc()
		raw := msg.Pack()
		cs := t.IOstats.NewCallSize(zx.Sfind, len(raw))
		reqc <- raw
		close(reqc)
		some := false
		for rep := range repc {
			cs.Send(int64(len(rep)))
			d, _, _ := zx.UnpackDir(rep)
			if d != nil {
				d["addr"] = t.addr
				d["proto"] = "zx"
			}
			if ok := dc <- d; !ok {
				close(repc, cerror(dc))
				break
			}
			some = true
			t.dprintf(rid, "find", "<- %v\n", d)
		}
		if !some {
			dc <- nil
		}
		err := cerror(repc)
		cs.End(err != nil)
		if err != nil {
			if isHangup(err) {
				t.Close(ErrHangUp)
			}
			t.dprintf("find %s: %s\n", rid, err)
		} else {
			t.dprintf("find %s: ok\n", rid)
		}
		close(dc, err)
		cs.End(err != nil)
	}()
	return dc

}

func (t *Rfs) FindGet(rid, pred, spref, dpref string, depth int) <-chan zx.DirData {
	t.dprintf("findget %s '%s' '%s' '%s' %d\n", rid, pred, spref, dpref, depth)
	dc := make(chan zx.DirData)
	msg := &Msg{Op: Tfindget, Rid: rid, Pred: pred, Spref: spref, Dpref: dpref, Depth: depth}
	go func() {
		reqc, repc := t.c.Rpc()
		raw := msg.Pack()
		cs := t.IOstats.NewCallSize(zx.Sfind, len(raw))
		reqc <- raw
		close(reqc)
		for rep := range repc {
			cs.Send(int64(len(rep)))
			d, _, _ := zx.UnpackDir(rep)
			if d != nil {
				d["addr"] = t.addr
				d["proto"] = "zx"
			}
			res := zx.DirData{Dir: d}
			var datc chan []byte
			if d["err"]=="" && d["type"]=="-" {
				datc = make(chan []byte)
				res.Datac = datc
			}
			if ok := dc <- res; !ok {
				close(repc, cerror(dc))
				break
			}
			t.dprintf(rid, "find", "<- %v\n", d)
			if datc != nil {
				for rep := range repc {
					if len(rep) == 0 {
						break
					}
					if ok := datc <- rep; !ok {
						continue
					}
				}
				close(datc, cerror(repc))
			}
		}
		err := cerror(repc)
		cs.End(err != nil)
		if err != nil {
			if isHangup(err) {
				t.Close(ErrHangUp)
			}
			t.dprintf("find %s: %s\n", rid, err)
		} else {
			t.dprintf("find %s: ok\n", rid)
		}
		close(dc, err)
	}()
	return dc

}
