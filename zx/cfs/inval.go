package cfs

import (
	"clive/dbg"
	"clive/zx"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// invalidation protocol.
// If epoch is "" it means we don't have /Chg in rfs and we must poll for changes
// and rely on Rtimes to see if medata is invalid.
// If epoch has a time string in std fmt, we have /Chg in rfs and Rtimes are considered
// invalid if they are not exactly epoch.
//
// There are two processes used, mostly, one to receive invals from the poll/rfs that
// walks to our lfs files and calls gotMeta() to update them (and posts invals to clients)
//
// Another that receives changes we make, due to client activity, and posts invals
// to other peer clients.
//
// If a client is too slow to read invalidations, it's closed and we let it go.
// It will try to reopen /Chg if it's a cfs and reset it's epoch. Thus, a slow reader will
// just invalidate everything when /Chg is lost.

// change from a client
type cChg struct {
	d  zx.Dir
	ci *zx.ClientInfo
}

// client reading /Chg
type cli struct {
	ci    *zx.ClientInfo
	c     chan<- []byte
	donec chan error
}

type invalQ struct {
	Tag string
	Dbg *bool

	epoch string

	rfs      zx.Tree
	invalslk sync.Mutex
	invals   []zx.Dir  // invalidations from rfs
	invalsc  chan bool // notify that we got invals

	clslk sync.Mutex
	cls   []cli // list of clients

	cchgs   []cChg // changes (we) made by our clients
	cchgslk sync.Mutex
	cchgc   chan bool // to notify we made changes
}

// returns true if the caller must poll itself rfs for changes
func newInvalQ(tag string, dbgf *bool, rfs zx.Tree) (*invalQ, bool) {
	iq := &invalQ{
		Tag:     tag,
		Dbg:     dbgf,
		cchgc:   make(chan bool, 1),
		rfs:     rfs,
		invalsc: make(chan bool, 1),
	}
	go iq.postinvalproc()
	if rfs == nil {
		return iq, false
	}
	// Use /Chg from rfs or poll rfs it if there's no other way.
	dc := rfs.Get("/Chg", 0, zx.All, "")
	msg := <-dc
	if len(msg) == 0 {
		dbg.Warn("no invalidations: %s", cerror(dc))
		return iq, true
	}
	go iq.getinvalproc(dc)
	return iq, false
}

func (q *invalQ) dprintf(fstr string, args ...interface{}) {
	if q != nil && *q.Dbg {
		fmt.Fprintf(os.Stderr, q.Tag+": "+fstr, args...)
	}
}

/*
	Invalidations for our clients
*/

// tell the inval protocol that we changed a file due to a client operation
// This will post invalidations to all clients reading /Chg other than the one making the call.
func (f *cFile) changed() {
	if f.t.ci == nil {
		f.dprintf("changed\n")
	} else {
		f.dprintf("changed by %s\n", f.t.ci.Tag)
	}
	f.t.cchgslk.Lock()
	f.t.cchgs = append(f.t.cchgs, cChg{ci: f.t.ci, d: f.d.Dup()})
	f.t.cchgslk.Unlock()
	select {
	case f.t.cchgc <- true:
	default:
	}
}

func (q *invalQ) CloseInvals() {
	close(q.cchgc)
}

func (q *invalQ) postinvalproc() {
	q.dprintf("postinvalproc started\n")
	defer q.dprintf("postinvalproc done\n")
	for range q.cchgc {
		q.cchgslk.Lock()
		chgs := q.cchgs
		q.cchgs = nil
		q.cchgslk.Unlock()
		for _, c := range chgs {
			q.postInval(c.d, c.ci)
		}
	}
}

func (fs *Cfs) getChg(off, count int64, pred string, c chan<- []byte, cs *zx.CallStat) (int, error) {
	if fs.noinvalproto {
		return 0, errors.New("inval proto disabled")
	}
	if fs.ci == nil {
		return 0, errors.New("no client info: no invalidations")
	}
	if ok := c <- []byte("leasing1.0"); !ok {
		return 0, cerror(c)
	}
	cl := cli{ci: fs.ci, c: c, donec: make(chan error)}
	fs.clslk.Lock()
	fs.cls = append(fs.cls, cl)
	fs.clslk.Unlock()
	return 0, <-cl.donec
}

// fwd d as an inval to all clients but the given one.
func (q *invalQ) postInval(d zx.Dir, ci *zx.ClientInfo) {
	i := 0
	for {
		q.clslk.Lock()
		if i >= len(q.cls) {
			q.clslk.Unlock()
			break
		}
		cl := q.cls[i]
		q.clslk.Unlock()
		if ci != nil && cl.ci.Id == ci.Id { // don't post to the one causing it
			i++
			continue
		}
		nd := d.UsrAttrs()
		nocattrs(nd)
		if s := d["Sum"]; s != "" {
			nd["Sum"] = s
		}
		if s := d["rm"]; s != "" {
			nd["rm"] = s
		}
		nd["path"] = d["path"]
		nd["name"] = d["name"]
		nd["type"] = d["type"]
		msg := nd.Pack()
		var err error
		select {
		case cl.c <- msg:
			if cclosed(cl.c) {
				err = cerror(cl.c)
				if err == nil {
					err = io.EOF
				}
				q.dprintf("post %s: %s\n", cl.ci.Tag, err)
			} else {
				q.dprintf("post %s: inval %s\n", cl.ci.Tag, nd)
				i++
			}
		case <-time.After(IvalTout):
			err = errors.New("slow reader client: closing its /Chg")
		}
		if err != nil {
			dbg.Warn("%s: %s", cl.ci.Tag, err)
			close(cl.c, err)
			q.clslk.Lock()
			copy(q.cls[i:], q.cls[i+1:])
			n := len(q.cls)
			q.cls = q.cls[:n-1]
			q.clslk.Unlock()
			cl.donec <- errors.New("client is gone")
			close(cl.donec)
		} else {
			i++
		}
	}
}

/*
	Invalidations from our server
*/

func (fs *Cfs) invalproc() {
	fs.dprintf("invalproc started\n")
	defer fs.dprintf("invalproc terminated\n")
	for {
		invals := fs.getInvals()
		for _, d := range invals {
			fs.dprintf("srv invalidating %s\n", d)
			fs.invalidate(d)
		}
	}
}

func (q *invalQ) getInvals() []zx.Dir {
	<-q.invalsc
	q.invalslk.Lock()
	defer q.invalslk.Unlock()
	dirs := q.invals
	q.invals = nil
	return dirs
}

func (q *invalQ) newEpoch() {
	ed := zx.Dir{}
	ed.SetTime("x", time.Now()) // use the std zx.Dir fmt.
	q.epoch = ed["x"]
	dbg.Warn("epoch %s\n", q.epoch)
}

func (q *invalQ) gotInval(ds ...zx.Dir) {
	if len(ds) > 0 {
		q.invalslk.Lock()
		q.invals = append(q.invals, ds...)
		q.invalslk.Unlock()
		select {
		case q.invalsc <- true:
		default:
		}
	}
}

func (q *invalQ) getinvalproc(dc <-chan []byte) {
	q.dprintf("getinvalproc started\n")
	defer q.dprintf("getinvalproc done\n")
	q.newEpoch()
	for {
		inv, ok := <-dc
		if !ok {
			q.newEpoch()
			err := cerror(dc)
			if err == nil {
				err = io.EOF
			}
			dbg.Warn("/Chg: closed: %s; epoch %s\n", err, q.epoch)
			time.Sleep(time.Second)
			dbg.Fatal("redial not implemented: run dialzx by hand. exiting.")
			dc = q.rfs.Get("/Chg", 0, zx.All, "")
			continue
		}
		var d zx.Dir
		var err error
		var ds []zx.Dir
		for len(inv) > 0 {
			d, inv, err = zx.UnpackDir(inv)
			if err != nil {
				q.dprintf("inval: %s\n", err)
				continue
			}
			ds = append(ds, d)
		}
		q.gotInval(ds...)
	}
}

func (q *invalQ) nInvalClients() int {
	q.clslk.Lock()
	defer q.clslk.Unlock()
	return len(q.cls)
}
