/*
	Trazing FS.

	Posts operations made to an underlying FS to a chan,
	for tracing.
*/
package trfs

import (
	"clive/dbg"
	"clive/net/auth"
	"clive/zx"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
)

type Flags struct {
	nb   int32
	C    chan string
	Verb bool
}

// A zx.RWTree wrapping another zx.Tree that sends
// log messages to C to report FS activity.
type Fs struct {
	ufs zx.Tree
	fs  zx.RWTree
	Tag string // set to prefix each msg with the tag.
	*Flags
}

// A Call, parsed from a traced message
type Call struct {
	Fs   string   // fs name
	Dir  string   // -> or <-
	Tag  int      // seq number
	Op   string   // op name
	Args []string // rest of msg
}

// parse a message as printed
func Parse(s string) *Call {
	c := &Call{Op: "unknown format"}

	dash := strings.IndexRune(s, '-')
	if dash < 0 || dash >= len(s)-3 {
		return c
	}
	c.Fs = s[:dash]
	if s[dash+1] == '>' {
		c.Dir = "->"
	} else {
		c.Dir = "<-"
	}
	s = s[dash+2:]
	stag := strings.IndexRune(s, '[')
	if stag < 0 {
		return c
	}
	c.Op = s[:stag]
	s = s[stag:]
	if len(s) < 3 {
		return c
	}
	s = s[1:]
	etag := strings.IndexRune(s, ']')
	if etag < 0 {
		return c
	}
	c.Tag, _ = strconv.Atoi(s[:etag])
	s = s[etag:]
	if len(s) < 2 {
		return c
	}
	s = s[1:]
	c.Args = strings.Fields(s)
	return c
}

func (c *Call) String() string {
	return fmt.Sprintf("%s%s%s[%d] %s", c.Fs, c.Dir, c.Op, c.Tag, strings.Join(c.Args, " "))
}

func New(t zx.Tree) *Fs {
	fs := &Fs{
		ufs:   t,
		fs:    zx.RWTreeFor(t),
		Flags: &Flags{},
	}
	return fs
}

func (t *Fs) Debug() *bool {
	if dfs, ok := t.fs.(zx.Debugger); ok {
		return dfs.Debug()
	}
	var fake bool
	return &fake
}

// Restart req numbering, for testing.
func (t *Fs) Restart() {
	t.nb = 0
}

func (t *Fs) vprintf(fs string, args ...interface{}) {
	if t.C != nil && t.Verb {
		t.C <- t.Tag+fmt.Sprintf(fs, args...)
	}
}

func (t *Fs) printf(fs string, args ...interface{}) {
	if t.C != nil {
		t.C <- t.Tag+fmt.Sprintf(fs, args...)
	}
}

func (t *Fs) Name() string {
	return t.fs.Name()
}

func (t *Fs) Close(e error) {
	n := atomic.AddInt32(&t.nb, 1)
	t.printf("->close[%d] sts %v", n, e)
	t.fs.Close(e)
}

func (t *Fs) String() string {
	return fmt.Sprintf("trfs:%s", t.fs.Name())
}

func (t *Fs) Stat(rid string) chan zx.Dir {
	n := atomic.AddInt32(&t.nb, 1)
	t.printf("->stat[%d] %s", n, rid)
	dc := make(chan zx.Dir, 1)
	go func() {
		xc := t.fs.Stat(rid)
		d := <-xc
		err := cerror(xc)
		t.printf("<-stat[%d] %s sts %v", n, d.LongTestFmt(), err)
		dc <- d
		close(dc, err)
	}()
	return dc
}

func (t *Fs) Get(rid string, off, count int64, pred string) <-chan []byte {
	n := atomic.AddInt32(&t.nb, 1)
	t.printf("->get[%d] %s %d %d '%s'", n, rid, off, count, pred)
	dc := make(chan []byte)
	go func() {
		xc := t.fs.Get(rid, off, count, pred)
		nb, nm := 0, 0
		for d := range xc {
			s := dbg.HexStr(d, 16)
			t.vprintf("<-get[%d] %d bytes %s", n, len(d), s)
			if ok := dc <- d; !ok {
				close(xc, "client gone")
			}
			nb += len(d)
			nm++
		}
		err := cerror(xc)
		t.printf("<-get[%d] %d bytes %d msgs sts %v", n, nb, nm, err)
		close(dc, err)
	}()
	return dc
}

func (t *Fs) Put(rid string, d zx.Dir, off int64, dc <-chan []byte, pred string) chan zx.Dir {
	n := atomic.AddInt32(&t.nb, 1)
	t.printf("->put[%d] %s %s %d '%s'", n, rid, d, off, pred)
	rc := make(chan zx.Dir, 1)
	datc := make(chan []byte)
	if dc == nil {
		datc = nil
	} else {
		go func() {
			for d := range dc {
				s := dbg.HexStr(d, 16)
				t.vprintf("->put[%d] %d bytes %s", n, len(d), s)
				if ok := datc <- d; !ok {
					close(dc, cerror(datc))
					break
				}
			}
			err := cerror(dc)
			t.vprintf("->put[%d] sts %v", n, err)
			close(datc, err)
		}()
	}
	go func() {
		xc := t.fs.Put(rid, d, off, datc, pred)
		d := <-xc
		err := cerror(xc)
		if err != nil {
			close(datc, err)
		}
		t.printf("<-put[%d] %s sts %v", n, d, err)
		rc <- d
		close(rc, err)
	}()
	return rc
}

func (t *Fs) Mkdir(rid string, d zx.Dir) chan error {
	n := atomic.AddInt32(&t.nb, 1)
	t.printf("->mkdir[%d] %s %s", n, rid, d)
	dc := make(chan error, 1)
	go func() {
		xc := t.fs.Mkdir(rid, d)
		err := <-xc
		t.printf("<-mkdir[%d] sts %v", n, err)
		dc <- err
		close(dc, cerror(xc))
	}()
	return dc
}

func (t *Fs) Wstat(rid string, d zx.Dir) chan error {
	n := atomic.AddInt32(&t.nb, 1)
	t.printf("->wstat[%d] %s %s", n, rid, d)
	dc := make(chan error, 1)
	go func() {
		xc := t.fs.Wstat(rid, d)
		err := <-xc
		t.printf("<-wstat[%d] sts %v", n, err)
		dc <- err
		close(dc, cerror(xc))
	}()
	return dc
}

func (t *Fs) Move(from, to string) chan error {
	n := atomic.AddInt32(&t.nb, 1)
	t.printf("->move[%d] %s %s", n, from, to)
	dc := make(chan error, 1)
	go func() {
		xc := t.fs.Move(from, to)
		err := <-xc
		t.printf("<-move[%d] sts %v", n, err)
		dc <- err
		close(dc, cerror(xc))
	}()
	return dc
}

func (t *Fs) Remove(rid string) chan error {
	n := atomic.AddInt32(&t.nb, 1)
	t.printf("->remove[%d] %s", n, rid)
	dc := make(chan error, 1)
	go func() {
		xc := t.fs.Remove(rid)
		err := <-xc
		t.printf("<-remove[%d] sts %v", n, err)
		dc <- err
		close(dc, cerror(xc))
	}()
	return dc
}

func (t *Fs) RemoveAll(rid string) chan error {
	n := atomic.AddInt32(&t.nb, 1)
	t.printf("->removeall[%d] %s", n, rid)
	dc := make(chan error, 1)
	go func() {
		xc := t.fs.RemoveAll(rid)
		err := <-xc
		t.printf("<-removeall[%d] sts %v", n, err)
		dc <- err
		close(dc, cerror(xc))
	}()
	return dc
}

func (t *Fs) Fsys(name string) <-chan error {
	n := atomic.AddInt32(&t.nb, 1)
	t.printf("->fsys[%d] %s", n, name)
	dc := make(chan error, 1)
	go func() {
		var xc <-chan error
		if fnd, ok := t.fs.(zx.Finder); ok {
			xc = fnd.Fsys(name)
		} else {
			xx := make(chan error, 1)
			xx <- dbg.ErrBug
			close(xx, dbg.ErrBug)
			xc = xx
		}
		err := <-xc
		t.printf("<-fsys[%d] sts %v", n, err)
		dc <- err
		close(dc, cerror(xc))
	}()
	return dc
}

func (t *Fs) Find(rid, fpred, spref, dpref string, depth int) <-chan zx.Dir {
	n := atomic.AddInt32(&t.nb, 1)
	t.printf("->find[%d] %s '%s' '%s' '%s' %d", n, rid, fpred, spref, dpref, depth)
	dc := make(chan zx.Dir, 1)
	go func() {
		xc := t.fs.Find(rid, fpred, spref, dpref, depth)
		nm := 0
		for d := range xc {
			nm++
			t.vprintf("<-find[%d] %s", n, d)
			dc <- d
		}
		err := cerror(xc)
		t.printf("<-find[%d] %d msgs sts %v", n, nm, err)
		close(dc, err)
	}()
	return dc
}

func (t *Fs) FindGet(rid, fpred, spref, dpref string, depth int) <-chan zx.DirData {
	n := atomic.AddInt32(&t.nb, 1)
	t.printf("->findget[%d] %s '%s' '%s' '%s' %d", n, rid, fpred, spref, dpref, depth)
	dc := make(chan zx.DirData)
	go func() {
		xc := t.fs.FindGet(rid, fpred, spref, dpref, depth)
		nm := 0
		for d := range xc {
			nm++
			t.vprintf("<-findget[%d] %s", n, d)
			if ok := dc <- d; !ok {
				close(xc, "client gone")
			}
		}
		err := cerror(xc)
		t.printf("<-findget[%d] %d msgs sts %v", n, nm, err)
		close(dc, err)
	}()
	return dc
}

// Ask the tree to perform auth checks on behalf of ai.
func (t *Fs) AuthFor(ai *auth.Info) (zx.Tree, error) {
	nfs := &Fs{}
	*nfs = *t
	if afs, ok := t.ufs.(zx.AuthTree); ok {
		x, err := afs.AuthFor(ai)
		if err == nil {
			nfs.ufs = x
			nfs.fs = zx.RWTreeFor(nfs.ufs)
			return nfs, nil
		}
		return nil, err
	}
	return nil, errors.New("no auth")
}

func (t *Fs) Dump(w io.Writer) {
	if dfs, ok := t.fs.(zx.Dumper); ok {
		dfs.Dump(w)
	} else {
		fmt.Fprintf(w, "no dumps for %s\n", t)
	}
}

func (t *Fs) Stats() *zx.IOstats {
	if dfs, ok := t.fs.(zx.StatTree); ok {
		return dfs.Stats()
	}
	return nil
}
