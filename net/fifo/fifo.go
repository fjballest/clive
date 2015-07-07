/*
	Fifo implements a network using system FIFOS and knows how to Dial/Serve there.

	This package is seldom used directly. It is more convenient to
	rely on the net/ds package to dial and/or serve services on the
	fifo network.
*/
package fifo

import (
	"clive/dbg"
	"clive/nchan"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// Change these only before using the package, at init time.
var (

	// Timeout for fifo dials.
	DialTimeout = 5*time.Second

	// directory where fifos are kept.
	Dir = "/tmp/ds.fifo"

	// enable debug diagnostics.
	Debug bool

	dprintf = dbg.FlagPrintf(os.Stderr, &Debug)
	fifos   = map[string]bool{}
	fifoslk sync.Mutex
	idc     chan uint32
)

// A network server handling each client on a different process.
type Srv  {
	name    string
	port    string
	hndlr   CliHandler
	endc    chan bool
	haltedc chan bool

	// Serve at most one client and then exit.
	Once bool

	// Report activity.
	Verbose bool
	vprintf dbg.PrintFunc

	// Enable debug diagnostics.
	Debug   bool
	dprintf dbg.PrintFunc

	cl []string
	nc chan string
	ec chan string
}

// Implementor of this interface provides a handler for each client.
// This type is returned by NewChanHandler and given to New to create new servers.
type CliHandler interface {
	HandleCli(tag string, in io.ReadCloser, out io.WriteCloser, endc chan bool)
	Close()
}

type ch  {
	ncc chan *nchan.Conn
}

// Make a CliHandler that accepts new clients and sends one Conn for each client
// to the channel returned.
func NewChanHandler() (CliHandler, chan *nchan.Conn) {
	ncc := make(chan *nchan.Conn, 1)
	return ch{ncc: ncc}, ncc
}

func (h ch) Close() {
	close(h.ncc, "exiting")
}

// Handle one client, called by fifo.Srv.
func (h ch) HandleCli(tag string, in io.ReadCloser, out io.WriteCloser, endc chan bool) {
	inc := make(chan bool)
	outc := make(chan bool)
	nc := nchan.NewSplitConn(in, out, 5, inc, outc)
	nc.Tag = tag
	h.ncc <- &nc
	doselect {
	case <-inc:
		inc = nil
		if inc==nil && outc==nil {
			return
		}
	case <-outc:
		outc = nil
		if inc==nil && outc==nil {
			return
		}
	case <-endc:
		close(nc.In, "server closing")
		close(nc.Out, "server closing")
		in.Close()
		out.Close()
	}
}

// Creates a new server to listen at the service name (svc)
// using the given client handler. The name argument is used for diagnostics.
func New(name, svc string, h CliHandler) *Srv {
	p := path.Join(Dir, path.Base(svc))
	s := &Srv{
		name:    name,
		port:    p,
		hndlr:   h,
		endc:    make(chan bool),
		haltedc: make(chan bool, 1),
		nc:      make(chan string),
		ec:      make(chan string),
	}
	s.dprintf = dbg.FlagPrintf(os.Stdout, &s.Debug)
	s.vprintf = dbg.FlagPrintf(os.Stdout, &s.Verbose)
	return s
}

func (s *Srv) done() bool {
	select {
	case <-s.endc:
		return true
	default:
		return false
	}
}

func (s *Srv) handleCli(c string) {
	defer rmfifo(c + ".in")
	defer rmfifo(c + ".out")
	ifd, err := os.OpenFile(c+".in", os.O_RDONLY, 0600)
	if err != nil {
		s.vprintf("%s: client open: %s\n", s.name, err)
		return
	}
	defer ifd.Close()
	ofd, err := os.OpenFile(c+".out", os.O_WRONLY, 0600)
	if err != nil {
		rmfifo(c + ".in")
		s.vprintf("%s: client open: %s\n", s.name, err)
		return
	}
	defer ofd.Close()
	s.vprintf("%s: new client %s\n", s.name, c)
	s.hndlr.HandleCli("fifo!*!"+path.Base(c), ifd, ofd, s.endc)
	s.vprintf("%s: gone client %s\n", s.name, c)
	rmfifo(c + ".in")
	s.ec <- c
}

// Start listening for clients, using a different process for each one.
// Returns after listening with the error status.
func (s *Srv) Serve() error {
	if runtime.GOOS == "openbsd" {
		return errors.New("go fifos are buggered for open bsd")
	}
	os.MkdirAll(Dir, 0750)
	if err := mkfifo(s.port); err != nil {
		return fmt.Errorf("%s: %s", s.port, err)
	}
	go func() {
		doselect {
		case c := <-s.nc:
			s.cl = append(s.cl, c)
		case c := <-s.ec:
			for i := 0; i < len(s.cl); i++ {
				if s.cl[i] == c {
					s.cl = append(s.cl[0:i],
						s.cl[i+1:]...)
					break
				}
			}
		case <-s.endc:
			s.dprintf("%s: closing...\n", s.name)
			rmfifo(s.port)
			for _, c := range s.cl {
				rmfifo(c + ".in")
				rmfifo(c + ".out")
			}
			s.hndlr.Close()
			s.haltedc <- true
			return
		}
	}()
	go func() {
		nb := 0
		defer rmfifo(s.port)
		s.vprintf("%s: listening on %s\n", s.name, s.port)
		lfd, err := os.OpenFile(s.port, os.O_RDONLY, 0600)
		if err != nil {
			s.dprintf("%s: open: exiting\n", s.name)
			return
		}
		neofs := 0
		for !s.done() {
			var buf [128]byte

			nr, err := lfd.Read(buf[0:])
			if err != nil {
				if err == io.EOF {
					if neofs++; neofs < 5 {
						s.dprintf("%s: accept: in: %s\n", s.name, err)
						continue
					}
				}
				s.vprintf("%s: accept: in: %s\n", s.name, err)
				break
			}
			neofs = 0
			if s.done() {
				break
			}
			fn := path.Join(Dir, path.Base(string(buf[0:nr])))
			s.nc <- fn
			if s.Once {
				s.handleCli(fn)
				s.dprintf("%s: once: exiting\n", s.name)
				break
			}
			go s.handleCli(fn)
			nb++
			lfd.Close()
			lfd, err = os.OpenFile(s.port, os.O_RDONLY, 0600)
			if err != nil {
				s.dprintf("%s: open: exiting\n", s.name)
				break
			}
		}
		lfd.Close()
		s.vprintf("%s: server done\n", s.name)
	}()
	return nil
}

// Stop the server and wait for cleanup if wait is true.
func (s *Srv) Stop(wait bool) {
	close(s.endc)
	if wait {
		<-s.haltedc
	}
}

/*
	Dial the named fifo service.
*/
func Dial(svc string) (nchan.Conn, error) {
	if runtime.GOOS == "openbsd" {
		return nchan.Conn{}, errors.New("go fifos are buggered for open bsd")
	}
	cc := make(chan nchan.Conn)
	go func() {
		xc, err := dial(svc)
		if err != nil {
			close(cc, err)
		} else {
			cc <- xc
			close(cc)
		}
	}()
	select {
	case xc := <-cc:
		return xc, cerror(cc)
	case <-time.After(DialTimeout):
		return nchan.Conn{}, errors.New("fifo dial timed out")
	}
}

func dial(svc string) (nchan.Conn, error) {
	fn := path.Join(Dir, path.Base(svc))
	fd, err := os.OpenFile(fn, os.O_WRONLY, 0600)
	if err != nil {
		return nchan.Conn{}, err
	}
	defer fd.Close()
	for i := 0; ; i++ {
		dfn := fmt.Sprintf("%s.%d", fn, <-idc)
		ifn := dfn + ".in"
		ofn := dfn + ".out"
		if err := mkfifo(ifn); err != nil {
			if i > 1000 {
				return nchan.Conn{}, err
			}
			continue
		}
		if err := mkfifo(ofn); err != nil {
			rmfifo(ifn)
			continue
		}
		if _, err := fd.Write([]byte(dfn)); err != nil {
			rmfifo(ifn)
			rmfifo(ofn)
			return nchan.Conn{}, err
		}
		fd.Close()
		ofd, err := os.OpenFile(ifn, os.O_WRONLY, 0600)
		if err != nil {
			rmfifo(ifn)
			rmfifo(ofn)
			return nchan.Conn{}, err
		}
		ifd, err := os.OpenFile(ofn, os.O_RDONLY, 0600)
		if err != nil {
			ofd.Close()
			rmfifo(ifn)
			rmfifo(ofn)
			return nchan.Conn{}, err
		}
		ec := make(chan bool, 1)
		c := nchan.NewSplitConn(ifd, ofd, 0, nil, ec)
		go func() {
			<-ec
			ofd.Close()
			rmfifo(ofn)
		}()
		return c, nil
	}
}

func mkfifo(fname string) error {
	os.Remove(fname)
	if err := syscall.Mkfifo(fname, 0660); err != nil {
		dprintf("failed fifo %s: %s\n", fname, err)
		return err
	}
	fifoslk.Lock()
	dprintf("new fifo %s\n", fname)
	fifos[fname] = true
	fifoslk.Unlock()
	return nil
}

func rmfifo(fname string) error {
	dprintf("del fifo %s\n", fname)
	err := os.Remove(fname)
	fifoslk.Lock()
	delete(fifos, fname)
	fifoslk.Unlock()
	return err
}

func init() {
	fname := fmt.Sprintf("fifo.%s.%s", dbg.Sys, dbg.Usr)
	Dir = path.Join(dbg.Tmp, fname)
	fn := func() {
		dprintf("removing fifos...\n")
		for fname := range fifos {
			dprintf("\t%s\n", fname)
			os.Remove(fname)
		}
	}
	dbg.AtExit(fn)
	idc = make(chan uint32)
	go func() {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for {
			idc <- r.Uint32()
		}
	}()

}
