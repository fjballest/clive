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
	sync.Mutex	// for in, out, err
	c  chan interface{}
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
			close(cr.c)
		}
	}
}

func (cr *ioChan) start() {
	c := make(chan interface{})
	cr.c = c
	switch cr.name {
	case "in":
		rfn := ch.ReadMsgs
		if cr.ux {
			rfn = ch.ReadBytes
		}
		go func() {
			_, _, err := rfn(os.Stdin, c)
			close(c, err)
		}()
	case "out", "err":
		donec := make(chan bool)
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
		close(c)
	}
}

func (io *ioSet) add(name string, c chan interface{}) *ioChan {
	io.Lock()
	defer io.Unlock()
	oc, ok := io.set[name]
	if ok {
		oc.close()
	}
	nc := &ioChan{name: name, ref: 1, c: c}
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
	if c.c == nil {	// 1st time for in, out, err, null
		c.Lock()
		defer c.Unlock()
		if c.c == nil {
			c.start()
		}
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

func (io *ioSet) unixIO() {
	io.Lock()
	defer io.Unlock()
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
	nc := io.add("in", nil)
	nc.ref = -1
	nc = io.add("out", nil)
	nc.ref = -1
	nc = io.add("err", nil)
	nc.ref = -1
	nc = io.add("null", nil)
	nc.ref = -1
	return io
}
