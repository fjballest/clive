package app

import (
	"sync"
	"io"
	"sync/atomic"
	"os"
	"clive/dbg"
	"strconv"
	"errors"
	"fmt"
)

type cRef {
	c chan interface{}
	fd io.Closer	// will go in the future
	n int32		// <0 means it's never closed.
	nb int
}

// The app IOset is similar to a file descriptor group in Plan 9, and
// should work along those lines.
// The conventions are those expected for a Plan 9 file descriptor group.
// The difference is that here I/O channels are chan interface{} and
// not file channels.
type IOSet {
	lk sync.Mutex
	n int32
	io []*cRef
}

// types implementing this interface (besides []byte)
// are written to external output streams.
type Byteser interface {
	Bytes() []byte
}

var (
	Null, out, err chan interface{}	// DevNull, Stdout, Stderr
	outdone = make(chan bool)	// closed when out is done
	errdone = make(chan bool)	// closed when err is done

	errNotUsed = errors.New("closed by user")
)

func (io *IOSet) sprintf(w io.Writer)  {
	io.lk.Lock()
	defer io.lk.Unlock()
	fmt.Fprintf(w, "io %p ref %d\n", io, io.n)
	for i, r := range io.io {
		if r != nil {
			closed := ""
			if cclosed(r.c) {
				closed= " closed"
			}
			fmt.Fprintf(w, "\t[%d] %p\tref %d %p%s\n", i, r, r.n, r.c, closed)
		}
	}
}

func wio(tag string, c chan interface{}, w io.Writer, done chan bool) {
	dprintf("%s started\n", tag)
	defer dprintf("%s terminated\n", tag)
	for x := range c {
		switch x := x.(type) {
		case []byte:
			w.Write(x)
	/*
		case Byteser:
			w.Write(x.Bytes())
		case string:
			fmt.Fprintf(w, "%s", x)
		case error:
			if x != nil {
				fmt.Fprintf(w, "error: %s", x)
			}
		case zx.Dir:
			p := x["upath"]
			if p == "" {
				p = x["path"]
			}
			fmt.Fprintln(w, p)
		case fmt.Stringer:
			fmt.Fprintf(w, "%s", x)
	*/
		}
	}
	close(done)
}

func init() {
	Null  = make(chan interface{})
	close(Null)
	out = make(chan interface{})
	err = make(chan interface{})
	go wio("osout", out, os.Stdout, outdone)
	go wio("oserr", err, os.Stderr, errdone)
}

func (cr *cRef) used() {
	if cr != nil && atomic.LoadInt32(&cr.n) >= 0 {
		atomic.AddInt32(&cr.n, 1)
	}
}

func (cr *cRef) close(sts error) {
	if cr == nil {
		return
	}
	if old := atomic.LoadInt32(&cr.n); old >= 0 && atomic.AddInt32(&cr.n, -1) <= 0 {
		if old == 0 {
			dbg.Warn("app: too many closes on chan refs")
		}
		dprintf("closing %d\n", cr.nb)
		if cr.fd != nil {
			cr.fd.Close()
		} else if cr.c != nil {
			close(cr.c, sts)
		}
	}
}

func mkIO() *IOSet {
	io := &IOSet{
		n: 1,
		io: []*cRef{
			&cRef{c: Null, n: -1, nb: 0},
			&cRef{c: out, n: -1, nb: 1},
			&cRef{c: err, n: -1, nb: 2},
		},
	}
	return io
}

// Start using a new IO set.
// If the given one is nil, the IO set is re-initialized from that in the underlying os
// (stdin is from /dev/null in that case).
func NewIO(io *IOSet)  {
	if io == nil {
		io = mkIO()
	}
	c := ctx()
	c.lk.Lock()
	old := c.io
	if old == io {
		c.lk.Unlock()
		return
	}
	c.io = io
	c.lk.Unlock()
	if old != nil {
		old.closeAll(errNotUsed)
	}
}

// Return a copy of the current IO set.
// Caller should close all on it before releasing the reference.
func IO() *IOSet {
	c := ctx()
	return c.IO()
}

// Return a copy of the IO set used in this context.
// Caller should close all on it before releasing the reference.
func (c *Ctx) IO() *IOSet {
	c.lk.Lock()
	defer c.lk.Unlock()
	return c.io.dup()
}


// Start using a copy of the current IO set
func DupIO() {
	NewIO(IO())
}

func (io *IOSet) dup() *IOSet {
	io.lk.Lock()
	defer io.lk.Unlock()
	nio := &IOSet{n: 1}
	for _, c := range io.io {
		c.used()
		nio.io = append(nio.io, c)
	}
	return nio
}

func (io *IOSet) close(n int, sts error) error {
	io.lk.Lock()
	defer io.lk.Unlock()
	if n < 0 || n >= len(io.io) || io.io[n] == nil {
		return errors.New("no such io chan")
	}
	io.io[n].close(sts)
	io.io[n] = nil
	return nil
}

func (io *IOSet) closeAll(sts error)  {
	io.lk.Lock()
	defer io.lk.Unlock()
	if io.n < 0 {
		return
	}
	io.n--
	if io.n > 0 {
		return
	}
	dprintf("clossing all io\n")
	for i, c := range io.io {
		if c != nil {
			c.close(sts)
		}
		io.io[i] = nil
	}
}

// Use -1 to set at the first available
func (io *IOSet) setc(c chan interface{}, n int) (int, error) {
	return io.setioc(&cRef{c: c, n: 1}, n)
}

func (io *IOSet) setioc(nc *cRef, n int) (int, error) {
	io.lk.Lock()
	defer io.lk.Unlock()
	if n < 0 {
		for n := range io.io {
			if io.io[n] == nil {
				io.io[n] = nc
				return n, nil
			}
		}
		io.io = append(io.io, nc)
		nc.nb = len(io.io)-1
		return nc.nb, nil
	}
	if n > len(io.io) {
		return -1, errors.New("bad io number: too large")
	}
	if n == len(io.io) {
		io.io = append(io.io, nc)
		nc.nb = n
		return n, nil
	}
	if io.io[n] != nil {
		io.io[n].close(errNotUsed)
	}
	io.io[n] = nc
	nc.nb = n
	return n, nil
}

func (io *IOSet) getc(n int) (chan interface{}, error) {
	io.lk.Lock()
	defer io.lk.Unlock()
	if n < 0 || n >= len(io.io) || io.io[n] == nil {
		return Null, errors.New("no such io chan")
	}
	return io.io[n].c, nil
}

func (io *IOSet) dupc(from, to int) error {
	io.lk.Lock()
	if from < 0 || from >= len(io.io) || io.io[from] == nil {
		io.lk.Unlock()
		return errors.New("no such io chan")
	}
	ioc := io.io[from]
	io.lk.Unlock()
	ioc.used()
	_, err := io.setioc(ioc, to)
	if err != nil {
		ioc.close(err)
	}
	return err
}

// Get the n-th IO chan
func IOchan(n int) (chan interface{}, error) {
	c := ctx()
	c.lk.Lock()
	io := c.io
	c.lk.Unlock()
	return io.getc(n)
}

// Get the IO chan for a #name arg
func IOarg(name string) (chan interface{}, error) {
	if len(name) == 0 {
		return nil, errors.New("empty #name")
	}
	if name[0] != '#' {
		return nil, errors.New("not pipe name")
	}
	nb, err := strconv.Atoi(name[1:])
	if err != nil {
		return nil, fmt.Errorf("pipe name: '%s': not a number", name)
	}
	ioc, err := IOchan(nb)
	if err != nil {
		return nil, fmt.Errorf("pipe name: '%s': %s", name, err)
	}
	return ioc, nil
}

// IOchan(0)
func In() chan interface{} {
	c, _ := IOchan(0)
	if c == nil {
		return Null
	}
	return c
}

// IOchan(1)
func Out() chan interface{} {
	c, _ := IOchan(1)
	if c == nil {
		return Null
	}
	return c
}

// IOchan(2)
func Err() chan interface{} {
	c, _ := IOchan(2)
	if c == nil {
		return Null
	}
	return c
}

const Any = -1
// Set the n-th io chan (or the 1st available if n < 0)
// Returns the pos of the chan in the io set.
func SetIO(c chan interface{}, n int) (int, error) {
	xc := ctx()
	xc.lk.Lock()
	io := xc.io
	xc.lk.Unlock()
	return io.setc(c, n)
}

// Dup the from io chan into to.
// Unlike SetIO(IOchan(from), to), this closes the chan both
// descriptors go and not when one of them goes.
// Add this chan to the ioset and return its id.
func CopyIO(from, to int) error {
	xc := ctx()
	xc.lk.Lock()
	io := xc.io
	xc.lk.Unlock()
	return io.dupc(from, to)
}

func AddIO(c chan interface{}) int {
	n, _ := SetIO(c, -1)
	return n
}

func mksts(sts ...interface{}) error {
	if len(sts) > 0 {
		if e, ok := sts[0].(error); ok {
			return e
		}
		if e, ok := sts[0].(string); ok {
			return errors.New(e)
		}
		return errors.New("errors")
	}
	return nil
}

// Close the n-th io chan from the set
func Close(n int, sts ...interface{}) error {
	c := ctx()
	c.lk.Lock()
	io := c.io
	c.lk.Unlock()
	return io.close(n, mksts(sts...))
}

// Drop the reference on the IO set and close all the chans in IO
// if it's the last reference.
func CloseAll(sts ...interface{}) {
	c := ctx()
	c.lk.Lock()
	io := c.io
	c.lk.Unlock()
	io.closeAll(mksts(sts...))
}
