/*
	Support to serve on TCP/IP networks.

	This package is seldom used directly. It is more convenient to
	rely on the net/ds package to dial and/or serve services on the
	tcp network.

	The package relies on clive/auth to push TLS on the connections when enabled.
*/
package srv

import (
	"clive/dbg"
	"clive/net/auth"
	"crypto/tls"
	"net"
	"os"
	"strings"
	"time"
)

// A network server handling each client on a different process.
type Srv  {
	name string
	net  string
	mach string
	port string

	// Serve at most one client and then exit.
	Once bool

	hndlr   CliHandler
	endc    chan bool
	haltedc chan bool

	cl []net.Conn
	nc chan net.Conn
	ec chan net.Conn
}

// Implementor of this interface provides a handler for each client.
// This type is returned by NewChanHandler and given to New to create new servers.
type CliHandler interface {
	HandleCli(c net.Conn, endc chan bool)
}

// Can be set to enable debug diagnostics and to report activity.
var Debug, Verbose bool

// Print diagnostic if verbose is set. Can be used from outside.
var Vprintf = dbg.FlagPrintf(os.Stderr, &Verbose)

// Print diagnostic if debug is set. Can be used from outside.
var Dprintf = dbg.FlagPrintf(os.Stderr, &Debug)

// This is the standard tls.Dial, modified to use a timeout. Should probably go from here.
func TlsDialTimeout(network, addr string, config *tls.Config, tout time.Duration) (*tls.Conn, error) {
	raddr := addr
	c, err := net.DialTimeout(network, raddr, tout)
	if err != nil {
		return nil, err
	}
	if tc, ok := c.(*net.TCPConn); ok {
		tc.SetKeepAlivePeriod(2*time.Second)
		tc.SetKeepAlive(true)
	}
	colonPos := strings.LastIndex(raddr, ":")
	if colonPos == -1 {
		colonPos = len(raddr)
	}
	hostname := raddr[:colonPos]

	// If no ServerName is set, infer the ServerName
	// from the hostname we're connecting to.
	if config.ServerName == "" {
		// Make a copy to avoid polluting argument or default.
		c := *config
		c.ServerName = hostname
		config = &c
	}
	conn := tls.Client(c, config)
	if err = conn.Handshake(); err != nil {
		c.Close()
		return nil, err
	}
	return conn, nil
}

// Creates a new server to listen at the given (tcp) port (e.g., "9999")
// using the given client handler. The name is used for log messages.
func New(name, netaddr, mach, port string, h CliHandler) *Srv {
	if mach == "*" {
		mach = ""
	}
	return &Srv{
		name:    name,
		net:     netaddr,
		mach:    mach,
		port:    port,
		hndlr:   h,
		endc:    make(chan bool),
		haltedc: make(chan bool, 1),
		nc:      make(chan net.Conn),
		ec:      make(chan net.Conn),
	}
}

// Convenience wrapper to dial a server using TLS.
func TlsDial(addr, pem, key string, tout time.Duration) (net.Conn, error) {
	cert, err := tls.LoadX509KeyPair(pem, key)
	if err != nil {
		return nil, err
	}
	cfg := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}
	return TlsDialTimeout("tcp", addr, cfg, tout)
}

func (s *Srv) done() bool {
	select {
	case <-s.endc:
		return true
	default:
		return false
	}
}

func (s *Srv) handleCli(c net.Conn) {
	Vprintf("%s: new client %s\n", s.name, c.RemoteAddr())
	s.hndlr.HandleCli(c, s.endc)
	Vprintf("%s: gone client %s\n", s.name, c.RemoteAddr())
	s.ec <- c
}

// Start listening for clients, using a different process for each one.
// Returns after listening with the error status.
func (s *Srv) Serve() error {
	var l net.Listener
	var err error
	usetls := auth.TLSserver != nil
	l, err = net.Listen(s.net, s.mach+":"+s.port)
	if err != nil {
		return err
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
			Dprintf("%s: closing...\n", s.name)
			l.Close()
			for _, c := range s.cl {
				c.Close()
			}
			s.haltedc <- true
			return
		}
	}()
	go func() {
		Vprintf("%s: listening\n", s.name)
		for !s.done() {
			c, err := l.Accept()
			if err != nil {
				Dprintf("%s: accept: %s\n", s.name, err)
				continue
			}
			if tc, ok := c.(*net.TCPConn); ok {
				// Beware this is not enough if you have NATs
				tc.SetKeepAlivePeriod(2*time.Second)
				tc.SetKeepAlive(true)
			}
			if usetls {
				c = tls.Server(c, auth.TLSserver)
			}
			if s.done() {
				c.Close()
				break
			}
			s.nc <- c
			if s.Once {
				s.handleCli(c)
				Dprintf("%s: once: exiting\n", s.name)
				break
			}
			go s.handleCli(c)
		}
		Vprintf("%s: server done\n", s.name)
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
