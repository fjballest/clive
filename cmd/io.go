package cmd

import (
	"clive/ch"
	"clive/dbg"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

struct ioChan {
	sync.Mutex
	isIn  bool
	inc   <-chan face{}
	outc  chan<- face{}
	donec chan bool
	fd    io.Closer // will go in the future
	ref   int32     // <0 means it's never closed.
	name  string
	ux    bool
	uxfd  int
}

struct ioSet {
	sync.Mutex
	ref int32
	set map[string]*ioChan
}

func (cr *ioChan) refer() {
	if cr != nil && atomic.LoadInt32(&cr.ref) >= 0 {
		atomic.AddInt32(&cr.ref, 1)
	}
}

func (cr *ioChan) close() {
	if cr == nil {
		return
	}

	// If the ref counter is >= 0, we must close (and wait for outstanding I/O)
	// only if a decref makes it 0.
	// However, OS in/out/err have their ref set to -1 so they are never closed
	// which means that we must wait from I/O if it's the main context the one
	// exiting, because in that case the entire UNIX process will die.
	mainexits := ctx() == mainctx

	if old := atomic.LoadInt32(&cr.ref); mainexits || old >= 0 && atomic.AddInt32(&cr.ref, -1) <= 0 {
		if old == 0 {
			dbg.Warn("app: too many closes on chan refs")
		}
		close(cr.inc)
		close(cr.outc)
		if cr.donec != nil {
			<-cr.donec
		}
		if cr.fd != nil {
			cr.fd.Close()
		}
	}
}

// TODO: We shouldn't be using "err" anymore.
// Errors should be sent along the output stream.
// The printer at the end of the pipe could print them in-line
// to os.Stderr, after making a flush of os.Stdout so the error shows up
// where it should.
// The risk is that we miss error diags if the consumer eats them and
// does not print them.
// But It's worth considering.

func (cr *ioChan) start() {
	c := make(chan face{})
	if cr.uxfd < 0 {
		close(c)
		if cr.isIn {
			cr.inc = c
		} else {
			cr.outc = c
		}
		return
	}
	var fd *os.File
	switch cr.uxfd {
	case 0:
		fd = os.Stdin
	case 1:
		fd = os.Stdout
	case 2:
		fd = os.Stderr
	default:
		fd = os.NewFile(uintptr(cr.uxfd), cr.name)
		cr.fd = fd
	}
	if cr.isIn {
		cr.inc = c
		rfn := ch.ReadMsgs
		if cr.ux {
			rfn = ch.ReadBytes
		}
		go func() {
			_, _, err := rfn(fd, c)
			close(c, err)
		}()
	} else {
		cr.outc = c
		donec := make(chan bool)
		cr.donec = donec
		if cr.ux {
			go func() {
				_, _, err := ch.WriteBytes(fd, c)
				close(c, err)
				close(donec)
			}()
		} else {
			go func() {
				_, _, err := ch.WriteMsgs(fd, 1, c)
				close(c, err)
				close(donec)
			}()
		}
		runtime.AtExit(func() {
			close(c)
			<-donec
		})
	}
}

func (io *ioSet) addIn(name string, c <-chan face{}) *ioChan {
	io.Lock()
	defer io.Unlock()
	oc, ok := io.set[name]
	if ok {
		oc.close()
	}
	nc := &ioChan{name: name, ref: 1, inc: c, isIn: true, uxfd: -1}
	nc.outc = make(chan face{})
	close(nc.outc, "not for output")
	io.set[name] = nc
	return nc
}

func (io *ioSet) addOut(name string, c chan<- face{}) *ioChan {
	io.Lock()
	defer io.Unlock()
	oc, ok := io.set[name]
	if ok {
		oc.close()
	}
	nc := &ioChan{name: name, ref: 1, outc: c, isIn: false, uxfd: -1}
	nc.inc = make(chan face{})
	close(nc.inc, "not for input")
	io.set[name] = nc
	return nc
}

func (io *ioSet) addUXIn(name string, fd int) *ioChan {
	io.Lock()
	defer io.Unlock()
	oc, ok := io.set[name]
	if ok {
		oc.close()
	}
	nc := &ioChan{name: name, ref: 1, isIn: true, uxfd: fd}
	nc.outc = make(chan face{})
	close(nc.outc, "not for output")
	io.set[name] = nc
	return nc

}

func (io *ioSet) addUXOut(name string, fd int) *ioChan {
	io.Lock()
	defer io.Unlock()
	oc, ok := io.set[name]
	if ok {
		oc.close()
	}
	nc := &ioChan{name: name, ref: 1, isIn: false, uxfd: fd}
	nc.inc = make(chan face{})
	close(nc.inc, "not for input")
	io.set[name] = nc
	return nc
}

func (io *ioSet) del(name string) {
	io.Lock()
	defer io.Unlock()
	if c, ok := io.set[name]; ok {
		delete(io.set, name)
		c.close()
	}
}

func (io *ioSet) get(name string) *ioChan {
	io.Lock()
	defer io.Unlock()
	c, ok := io.set[name]
	if !ok {
		return nil
	}
	c.Lock()
	defer c.Unlock()
	if (c.isIn && c.inc == nil) || (!c.isIn && c.outc == nil) {
		c.start()
	}
	return c
}

func (io *ioSet) refer() {
	io.Lock()
	io.ref++
	io.Unlock()
}

func (io *ioSet) dup() *ioSet {
	io.Lock()
	defer io.Unlock()
	nio := &ioSet{
		ref: 1,
		set: map[string]*ioChan{},
	}
	for k, cr := range io.set {
		cr.refer()
		nio.set[k] = cr
	}
	return nio
}

func (io *ioSet) chans() (in []string, out []string) {
	io.Lock()
	defer io.Unlock()
	for k, cr := range io.set {
		if cr.isIn {
			in = append(in, k)
		} else {
			out = append(out, k)
		}
	}
	sort.Sort(sort.StringSlice(in))
	sort.Sort(sort.StringSlice(out))
	return in, out
}

func (io *ioSet) close() {
	io.Lock()
	defer io.Unlock()
	io.ref--
	if io.ref < 0 {
		panic("ioset ref < 0")
	}
	if io.ref == 0 {
		for _, cr := range io.set {
			cr.close()
		}
	}
}

func (io *ioSet) unixIO(name ...string) {
	io.Lock()
	defer io.Unlock()
	if len(name) > 0 {
		for _, n := range name {
			if cf, ok := io.set[n]; ok {
				cf.ux = true
			}
		}
		return
	}
	for _, cr := range io.set {
		cr.Lock()
		cr.ux = true
		cr.Unlock()
	}

}

func (io *ioSet) addUXio() {
	env := os.Environ()
	for _, v := range env {
		if !strings.HasPrefix(v, "cliveio#") {
			continue
		}
		toks := strings.Split(v, "=")
		if len(toks) != 2 {
			continue
		}
		cname := strings.TrimPrefix(toks[0], "cliveio#")
		fdval := toks[1]
		if len(fdval) < 2 {
			continue
		}
		dir := fdval[0]
		if dir != '<' && dir != '>' {
			continue
		}
		fdname := fdval[1:]
		n, err := strconv.Atoi(fdname)
		if err != nil {
			continue
		}
		if dir == '<' {
			io.addUXIn(cname, n)
		} else {
			io.addUXOut(cname, n)
		}
	}
}

// Initialize a new io from the os
func mkIO() *ioSet {
	io := &ioSet{
		ref: 1,
		set: map[string]*ioChan{},
	}
	nc := io.addUXIn("in", 0)
	nc.ref = -1
	nc = io.addUXOut("out", 1)
	nc.ref = -1
	nc = io.addUXOut("err", 2)
	nc.ref = -1
	c := make(chan face{})
	close(c)
	nc = io.addIn("null", c)
	nc.outc = c
	io.addUXio()
	return io
}
