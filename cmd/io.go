package cmd

import (
	"sync"
	"io"
	"sync/atomic"
	"clive/dbg"
	"clive/ch"
	"runtime"
	"os"
)

type ioChan struct {
	sync.Mutex
	isIn bool
	inc  <-chan interface{}
	outc chan<- interface{}
	donec chan bool
	fd io.Closer // will go in the future
	ref  int32     // <0 means it's never closed.
	name string
	ux bool
}

type ioSet struct {
	sync.Mutex
	ref  int32
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
	if old := atomic.LoadInt32(&cr.ref); old >= 0 && atomic.AddInt32(&cr.ref, -1) <= 0 {
		if old == 0 {
			dbg.Warn("app: too many closes on chan refs")
		}
		if cr.fd != nil {
			cr.fd.Close()
		} else {
			close(cr.inc)
			close(cr.outc)
		}
		if cr.donec != nil {
			<-cr.donec
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
	switch cr.name {
	case "in":
		c := make(chan interface{})
		cr.inc = c
		rfn := ch.ReadMsgs
		if cr.ux {
			rfn = ch.ReadBytes
		}
		go func() {
			_, _, err := rfn(os.Stdin, c)
			close(c, err)
		}()
	case "out", "err":
		c := make(chan interface{})
		cr.outc = c
		donec := make(chan bool)
		cr.donec = donec
		fd := os.Stdout
		if cr.name == "err" {
			fd = os.Stderr
		}
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
	default:
		// XXX: TODO: bridge to OS fds from ql chans
		c := make(chan interface{})
		close(c)
		if cr.isIn {
			cr.inc = c
		} else {
			cr.outc = c
		}
	}
}

func (io *ioSet) addIn(name string, c <-chan interface{}) *ioChan {
	io.Lock()
	defer io.Unlock()
	oc, ok := io.set[name]
	if ok {
		oc.close()
	}
	nc := &ioChan{name: name, ref: 1, inc: c, isIn: true}
	nc.outc = make(chan interface{})
	close(nc.outc, "not for output")
	io.set[name] = nc
	return nc
}

func (io *ioSet) addOut(name string, c chan<- interface{}) *ioChan {
	io.Lock()
	defer io.Unlock()
	oc, ok := io.set[name]
	if ok {
		oc.close()
	}
	nc := &ioChan{name: name, ref: 1, outc: c, isIn: true}
	nc.inc = make(chan interface{})
	close(nc.inc, "not for input")
	io.set[name] = nc
	return nc
}

func (io *ioSet) del(name string)  {
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

// Initialize a new io from the os
func mkIO() *ioSet {
	io := &ioSet{
		ref: 1,
		set: map[string]*ioChan{},
	}
	nc := io.addIn("in", nil)
	nc = io.addOut("out", nil)
	nc = io.addOut("err", nil)
	nc = io.addIn("null", nil)
	nc.outc = make(chan interface{})
	close(nc.outc)
	// XXX: TODO: must define chans for ql unix fds
	// look for env varrs io#name=nb
	// and then use the open fd #nb to get/send msgs.
	// But that requires our io to know which chans are for input
	// and which ones are for output.
	return io
}
