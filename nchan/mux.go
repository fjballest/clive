package nchan

import (
	"clive/dbg"
	"encoding/binary"
	"errors"
	"os"
	"sync"
)

const (
	/*
		The tag space is divided so that the caller has
		even tags and the callee odd tags.
		Calls that expect a reply using the tag of the request.
	*/
	endtag  uint64 = 0x8000000000000000
	rpctag  uint64 = 0x4000000000000000
	huptag  uint64 = 0x2000000000000000
	tagmask        = endtag | rpctag | huptag
)

// TODO(?): We can't control buffering with this interface.
// The mux does not have access to the r/w underlying its Conn,
// so all the new control placed in nchan regarding when to flush and
// what to buffer can't be used by mux.
// But the present interface is very convenient to be changed, we rely on
// Conns to perform auth before a mux is pushed, and we don't care
// where messages are being written, which is good.

/*
	Muxes a Conn so that there can be multiple in/out channels
	within the same connection.
	One side of the connection is said to be the caller and the other
	the callee, this is indicated when the Mux is created.
*/
type Mux struct {
	In chan Conn

	outlk sync.Mutex
	c     Conn

	taglk sync.Mutex
	tag   uint64
	tags  map[uint64]chan []byte
	hups  map[uint64]bool
	rtags map[uint64]chan []byte
	hupc  chan error

	err error

	Tag     string
	Debug   bool
	dprintf dbg.PrintFunc
}

/*
	Create a mux on c.
	The given c should not be used by the caller afterwards.
*/
func NewMux(c Conn, iscaller bool) *Mux {
	m := &Mux{
		Tag:   c.Tag,
		In:    make(chan Conn),
		c:     c,
		tags:  map[uint64]chan []byte{},
		hups:  map[uint64]bool{},
		rtags: map[uint64]chan []byte{},
		hupc:  make(chan error, 1),
	}
	if !iscaller {
		m.tag = 1
	}
	m.dprintf = dbg.FlagPrintf(os.Stdout, &m.Debug)
	go m.demux()
	return m
}

/*
	Close all input and output channels with the given error.
*/
func (m *Mux) Close(err error) {
	m.err = err
	m.dprintf("mux %s closing (err %v)\n", m.Tag, err)
	close(m.c.In, err)
	close(m.c.Out, err)
	if err == nil {
		m.err = errors.New("mux closed")
	}
}

func (m *Mux) close(err error) {
	m.dprintf("mux %s exiting (err %v)\n", m.Tag, err)
	m.taglk.Lock()
	for _, c := range m.tags {
		close(c, err)
	}
	for _, c := range m.rtags {
		close(c, err)
	}
	m.err = err
	if err == nil {
		m.err = errors.New("mux closed")
	}
	m.taglk.Unlock()
	close(m.In, err)
}

func (m *Mux) send(tag uint64, data []byte) error {
	m.outlk.Lock()
	defer m.outlk.Unlock()
	var n [8]byte
	binary.LittleEndian.PutUint64(n[:], tag)
	if ok := m.c.Out <- n[:]; !ok {
		return cerror(m.c.Out)
	}
	if ok := m.c.Out <- data; !ok {
		return cerror(m.c.Out)
	}
	return nil
}

func (m *Mux) out(tag uint64, oc chan []byte) {
	for c := range oc {
		if err := m.send(tag, c); err != nil {
			close(oc, err)
			return
		}
	}
	err := cerror(oc)
	var emsg []byte
	if err != nil {
		emsg = []byte(err.Error())
	}
	m.send(tag|endtag, emsg)
}

/*
	Ask for a channel to issue a new call.
	The request is considered complete when the
	returned channel is closed.
	If the returned channel is closed with an error,
	the error is conveyed to the other end.
*/
func (m *Mux) Out() chan<- []byte {
	m.taglk.Lock()
	defer m.taglk.Unlock()
	m.tag += 2
	m.dprintf("mux %s: out %d\n", m.Tag, m.tag)
	oc := make(chan []byte)
	if m.err != nil {
		close(oc, m.err)
	} else {
		go m.out(m.tag, oc)
	}
	return oc
}

/*
	Ask for a channel to issue a new call to be replied
	and for the channel to receive the reply or replies.
	Simillar to Out(), but for RPCs.
*/
func (m *Mux) Rpc() (outc chan<- []byte, repc <-chan []byte) {
	m.taglk.Lock()
	defer m.taglk.Unlock()
	m.tag += 2
	m.dprintf("mux %s: out rpc %d\n", m.Tag, m.tag)
	oc := make(chan []byte)
	ic := make(chan []byte)
	if m.err != nil {
		close(oc, m.err)
		close(ic, m.err)
	} else {
		m.tags[m.tag] = ic
		go m.out(m.tag|rpctag, oc)
	}
	return oc, ic
}

/*
	Ask for a channel that will be closed when there's
	an error in the underlying connection.
*/
func (m *Mux) Hup() <-chan error {
	return m.hupc
}

func (m *Mux) demux() {
	var err error
	for {
		tm, ok := <-m.c.In
		if !ok {
			err = cerror(m.c.In)
			close(m.c.Out, err)
			break
		}
		if len(tm) != 8 {
			err = errors.New("short header in input channel (auth required?)")
			break
		}
		tag := binary.LittleEndian.Uint64(tm)
		isend := tag&endtag != 0
		isrpc := tag&rpctag != 0
		ishup := tag&huptag != 0
		tag &^= tagmask
		m.dprintf("mux %s: in tag %x end %v rpc %v hup %v\n",
			m.Tag, tag, isend, isrpc, ishup)
		data, ok := <-m.c.In
		if !ok {
			err = cerror(m.c.In)
			close(m.c.Out, err)
			break
		}

		m.taglk.Lock()
		if ishup {
			c, ok := m.rtags[tag]
			if ok {
				m.dprintf("mux %s: remote hup on %d\n", m.Tag, tag)
				close(c, "hup")
				delete(m.rtags, tag)
			}
			m.taglk.Unlock()
			continue
		}
		c, ok := m.tags[tag]
		if !ok {
			m.dprintf("mux %s: remote tag %d\n", m.Tag, tag)
			c = make(chan []byte)
			m.tags[tag] = c
		}
		m.taglk.Unlock()
		if !ok {
			var rc chan []byte
			if isrpc {
				rc = make(chan []byte)
				m.taglk.Lock()
				m.rtags[tag] = rc
				m.taglk.Unlock()
				go m.out(tag, rc)
			}
			ic := Conn{In: c, Out: rc}
			if iok := m.In <- ic; !iok {
				m.dprintf("mux %s: inc closed\n", m.Tag)
				err = cerror(m.In)
				break
			}
		}
		if isend {
			var merr error
			if len(data) > 0 {
				merr = errors.New(string(data))
			}
			close(c, merr)
			m.taglk.Lock()
			m.dprintf("mux %s: remote tag %d done\n", m.Tag, tag)
			delete(m.tags, tag)
			delete(m.hups, tag)
			m.taglk.Unlock()
		} else {
			if ok := c <- data; !ok {
				if _, ok := m.hups[tag]; !ok {
					m.dprintf("mux %s: hup on %d\n", m.Tag, tag)
					m.hups[tag] = true
					go m.send(tag|huptag, nil)
				}
			}
		}
	}
	m.hupc <- err
	close(m.hupc, err)
	m.close(err)
}
