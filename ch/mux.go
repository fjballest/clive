package ch

import (
	"clive/dbg"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

const (
	// The tag space uses even tags in the caller  and odd in the callee.
	// The first tag has the first bit set
	// The last tag has the end bit set
	// RPC tags have the rpc bit set
	firsttag uint32 = (1 << (31 - iota))
	rpctag
	flowtag
	endtag
	tagmask = firsttag | rpctag | flowtag | endtag
)

struct conn {
	tag     uint32
	in, out chan face{}
	flow    chan bool
}

interface flusher {
	Flush() error
}

// A Mux is a multiplexed set of channels on a duplex connection.
// It relies on Conns to perform I/O, but permits multiple Conns
// to be active on the same underlying device.
// There is flow control and it is ok for any of the mux clients to
// cease reading for a while, and to stream a bunch of data,
// other connections will be able to stream their data at the same time.
struct Mux {
	In   <-chan Conn   // new connections are sent here
	Hup  <-chan bool   // closed upon device hang up
	rw   io.ReadWriter // underlying device
	fl   flusher
	in   chan Conn        // In, for the implementation
	tag  uint32           // tag generator
	tags map[uint32]*conn // muxed chans
	err  error
	lk   sync.Mutex // for everything buf for writemsg
	wlk  sync.Mutex // for writemsg
	dbg.Flag
}

var (
	// Number of messages in chan buffers; can't be < 2
	nbuf = 1024

	ErrBadPeer = errors.New("both peers are caller/callee")
)

// Create a Mux on the given underlying device.
// One end of the device must be the caller and the other the callee.
// It does not matter which end is each one.
// When I/O ceases due to errors or the mux being closed, the underlying
// device is closed if it implements io.Closer.
// No half-closes are ever used.
func NewMux(rw io.ReadWriter, iscaller bool) *Mux {
	in := make(chan Conn, 10)
	m := &Mux{
		Flag: dbg.Flag{Tag: "mux"},
		In:   in,
		in:   in,
		Hup:  make(chan bool),
		rw:   rw,
		tag:  0,
		tags: map[uint32]*conn{},
	}
	m.fl, _ = rw.(flusher)
	if iscaller {
		m.tag = 1
	}
	go m.demux()
	return m
}

func (m *Mux) newConn(tag uint32, in, out chan face{}) *conn {
	tv := tag &^ tagmask
	mc := &conn{tag: tv, in: in, out: out, flow: make(chan bool, 3)}
	mc.flow <- true
	mc.flow <- true
	m.tags[tv] = mc
	m.Dprintf("new conn %x\n", tv)
	return mc
}

func (m *Mux) closeConn(mc *conn, err error) {
	m.Dprintf("close conn %x\n", mc.tag)
	close(mc.in, err)
	close(mc.out, err)
	close(mc.flow, err)
	delete(m.tags, mc.tag)
}

// Ask for a channel to send an output stream to the other end.
// There is no reply for the request stream.
func (m *Mux) Out() Conn {
	m.lk.Lock()
	defer m.lk.Unlock()
	if (m.tag+2)&tagmask != 0 {
		m.tag &= 1
	}
	m.tag += 2
	tv := m.tag
	out := make(chan face{}, nbuf)
	stag := fmt.Sprintf("%s!%x", m.Tag, tv)
	uc := Conn{Tag: stag, Out: out}
	mc := m.newConn(tv, nil, out)
	go m.out(mc, false)
	return uc
}

// Ask for a channel to send an output stream that expects
// an input stream as its reply.
func (m *Mux) Rpc() Conn {
	m.lk.Lock()
	defer m.lk.Unlock()
	if (m.tag+2)&tagmask != 0 {
		m.tag &= 1
	}
	m.tag += 2
	tv := m.tag
	in := make(chan face{}, nbuf)
	out := make(chan face{}, nbuf)
	stag := fmt.Sprintf("%s!%x", m.Tag, tv)
	uc := Conn{Tag: stag, In: in, Out: out}
	mc := m.newConn(tv, in, out)
	go m.out(mc, false)
	return uc
}

func (m *Mux) out(mc *conn, isreply bool) {
	tag := mc.tag
	c := mc.out
	isrpc := mc.in != nil && mc.out != nil
	tag &^= tagmask
	if isrpc {
		tag |= rpctag | firsttag
	} else {
		tag |= firsttag
	}
	m.Dprintf("out %x\n", tag)
	defer m.Dprintf("out %x done\n", tag)
	// Each ticket in mc.flow permits sending half the messages
	// in the chan buffer.
	<-mc.flow
	nmsgs := nbuf / 2
	for {
		d, ok := <-c
		if !ok {
			break
		}

		// flow control
		if nmsgs == 0 {
			m.Dprintf("stop flow %x\n", tag)
			<-mc.flow
			m.Dprintf("cont flow %x\n", tag)
			nmsgs += nbuf / 2
		}
		m.Dprintf("-> %x ... %d msgs\n", tag, nmsgs)
		if nmsgs > nbuf {
			panic("mux out nbuf too large")
		}
		m.wlk.Lock()
		_, err := WriteMsg(m.rw, tag, d)
		if err == nil && m.fl != nil {
			err = m.fl.Flush()
			if err != nil {
				err = fmt.Errorf("%s: %s", ErrIO, err)
			}
		}
		m.wlk.Unlock()
		nmsgs--
		m.Dprintf("-> %x sts %v\n", tag, err)
		if err == ErrDiscarded {
			continue
		}
		tag &^= firsttag
		if err != nil {
			close(c, err)
			m.lk.Lock()
			m.err = err
			m.lk.Unlock()
			break
		}
	}
	err := cerror(c)
	m.wlk.Lock()
	if err != nil {
		_, e := WriteMsg(m.rw, tag|endtag, err)
		if e == nil && m.fl != nil {
			e = m.fl.Flush()
		}
		m.Dprintf("-> %x %v sts %v\n", tag|endtag, err, e)
	} else {
		_, err = WriteMsg(m.rw, tag|endtag, empty)
		if err == nil && m.fl != nil {
			err = m.fl.Flush()
		}
		m.Dprintf("-> %x sts %v\n", tag|endtag, err)
	}
	m.wlk.Unlock()
	if isreply || !isrpc || err != nil {
		m.Dprintf("out %x closing\n", tag)
		m.lk.Lock()
		m.closeConn(mc, err)
		m.lk.Unlock()
	}
}

// flow control: when client consumes half the space
// we grant the peer the right to send another half
func (m *Mux) flowproc(tv uint32, min, uin chan face{}) {
	nposts := 0
	for {
		d, ok := <-min
		if !ok {
			close(uin, cerror(min))
			return
		}
		ok = uin <- d
		if !ok {
			close(min, cerror(uin))
			return
		}
		nposts++
		if nposts == nbuf/2 {
			m.Dprintf("+flow -> %x\n", tv|flowtag)
			m.wlk.Lock()
			WriteMsg(m.rw, tv|flowtag, empty)
			if m.fl != nil {
				m.fl.Flush()
			}
			m.wlk.Unlock()
			nposts = 0
		}
	}
}

func (m *Mux) demux() {
	for {
		_, tag, d, err := ReadMsg(m.rw)
		m.Dprintf("<- %x\n", tag)
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			m.err = err
			break
		}
		tv := tag &^ tagmask
		m.lk.Lock()
		if mc, ok := m.tags[tv]; !ok {
			if tag&firsttag == 0 {
				// the chan was closed, discard
				m.Dprintf("<-%x: discard\n", tag)
				m.lk.Unlock()
				continue
			}
			stag := fmt.Sprintf("%s!%x", m.Tag, tv)
			in := make(chan face{}, nbuf)
			m.Dprintf("in<-%x\n", tag)
			in <- d
			mc = m.newConn(tv, in, nil)
			if tag&rpctag != 0 {
				mc.out = make(chan face{}, nbuf)
			} else {
				close(mc.flow)
			}
			uin := make(chan face{}, 0)
			uc := Conn{Tag: stag, In: uin, Out: mc.out}
			go m.flowproc(tv, in, uin)
			m.lk.Unlock()
			if ok := m.in <- uc; !ok {
				m.lk.Lock()
				m.err = cerror(m.In)
				if m.err == nil {
					m.err = errors.New("mux.In is closed")
				}
				m.Dprintf("in<-%x: %v\n", tag, m.err)
				m.closeConn(mc, m.err)
				m.lk.Unlock()
				break
			}
			if tag&rpctag != 0 {
				go m.out(mc, true)
			}
		} else {
			m.lk.Unlock()
			// flow control: If this is a grant, make a ticket for out
			if tag&flowtag != 0 {
				m.Dprintf("flow<-%x\n", tag)
				go func() {
					mc.flow <- true
				}()
				continue
			}
			m.Dprintf("mux %s: in<-%x\n", m.Tag, tag)
			if tag&endtag != 0 {
				err, _ := d.(error)
				m.Dprintf("-> %x: end\n", tag)
				m.lk.Lock()
				if tag&rpctag == 0 {
					m.closeConn(mc, err)
				} else {
					close(mc.flow, err)
					close(mc.in, err)
				}
				m.lk.Unlock()
				continue
			}
			ok := true
			if mc.in != nil {
				// may be nil for flow cntl replies on Out requests
				ok = mc.in <- d
			}
			m.lk.Lock()
			m.Dprintf("in<-%x sent\n", tag)
			if !ok {
				m.Dprintf("in<-%x not ok\n", tag)
				m.closeConn(mc, cerror(mc.in))
			}
			m.lk.Unlock()
		}
	}
	m.Dprintf("in done\n")
	m.Close()
}

// Cease I/O in this mux and release all resources.
func (m *Mux) Close() {
	m.lk.Lock()
	defer m.lk.Unlock()
	m.Dprintf("closed\n")
	if m.err == nil {
		m.err = errors.New("mux closed by user")
	}
	close(m.In, m.err)
	if c, ok := m.rw.(io.Closer); ok {
		c.Close()
	}
	for _, mc := range m.tags {
		m.closeConn(mc, m.err)
	}
	close(m.Hup, m.err)
}

struct muxpipe {
	r io.ReadCloser
	w io.WriteCloser
}

func (b *muxpipe) Write(dat []byte) (int, error) {
	return b.w.Write(dat)
}

func (b *muxpipe) Read(dat []byte) (int, error) {
	return b.r.Read(dat)
}

func (b *muxpipe) CloseWrite() error {
	return b.w.Close()
}

func (b *muxpipe) CloseRead() error {
	return b.r.Close()
}

func (b *muxpipe) Close() error {
	if err := b.CloseWrite(); err != nil {
		b.CloseRead()
		return err
	}
	return b.CloseRead()
}

// Create a pair of (os) piped muxes
func NewMuxPair() (*Mux, *Mux, error) {
	// io.Pipe is synchronous and may lead to a deadlock
	// if readers don't read, despite flow control
	fd1 := &muxpipe{}
	fd2 := &muxpipe{}
	var err error
	fd1.r, fd2.w, err = os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	fd2.r, fd1.w, err = os.Pipe()
	if err != nil {
		fd1.r.Close()
		fd2.w.Close()
		return nil, nil, err
	}
	m1 := NewMux(fd1, false)
	m1.Tag = "pipemux1"
	m2 := NewMux(fd2, true)
	m2.Tag = "pipemux2"
	return m1, m2, nil
}
