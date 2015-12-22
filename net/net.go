/*
	Network services and tools over clive/ch chans.
*/
package net

import (
	"sync"
	"os"
	"strings"
	"net"
	"clive/ch"
	"clive/dbg"
	"errors"
	"strconv"
	"time"
	"crypto/tls"
	"fmt"
)

var (
	lk    sync.Mutex
	svcs  = map[string]string{
		"ns":  "8000",
		"sns": "8001",
		"zx":  "8002",
	}

	ErrBadAddr = errors.New("bad address")
	ErrNotLocal = errors.New("not a local address")
	ErrNoTLSCfg = errors.New("TLS not configured")

	// If these are set, the tls network will use them by default
	ClientTLSCfg, ServerTLSCfg *tls.Config
)

// Define name as the name for the service at the given TCP port.
// Can be used to provide the same service through the fifo/pipe networks
// and the TCP/IP network.
// Clive relies on some predefined services, including:
// 	ns	8000	name space
// 	sns	8001	shared name spaces
// 	zx	8002	zx
func DefSvc(name, port string) {
	lk.Lock()
	svcs[name] = port
	lk.Unlock()
}

// Return the port for a given network and service
func Port(netw, svc string) string {
	if netw == "*" || netw == "" {
		netw = "tcp"
	}
	lk.Lock()
	p, ok := svcs[svc]
	lk.Unlock()
	if !ok {
		if _, err := strconv.ParseInt(svc, 10, 64); err != nil {
			if nb, err := net.LookupPort(netw, svc); err == nil {
				p = strconv.Itoa(nb)
			}
		}
		if p == "" {
			p = svc
		}
	}
	if netw == "unix" {
		p = "/tmp/clive." + p
	}
	return p
}

// Parse an address and return its network, machine adress, and service name/number.
// When the address is invalid an empty network name is returned.
// Addresses are of the form
//	address
// 	address!service
// 	network!address!service
//
// The network/address may be "*" to use any available.
// Known networks are unix, tcp, and tls; the default is tcp.
// The service defaults to "zx".
func ParseAddr(addr string) (net, mach, svc string) {
	args := strings.Split(addr, "!")
	for i := 0; i < len(args); i++ {
		if args[i] == "" {
			args[i] = "*"
		}
	}
	switch len(args) {
	case 0:
		return // invalid fmt
	case 1:
		return "*", args[0], "zx"
	case 2:
		return "tcp", args[0], args[1]
	default:
		return args[0], args[1], args[2]
	}
}

// Return true if the machine address given seems to address the local host.
// A bad address is not considered local.
func IsLocal(host string) bool {
	if host == "" {
		return false
	}
	if host == "*" || host == "localhost" || host == "local" {
		return true
	}
	if hn, err := os.Hostname(); err == nil && hn == host {
		return true
	}
	if host[0] == '[' && len(host) > 2 && host[len(host)-1] == ']' {
		host = host[1:len(host)-1]
	}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			as := strings.SplitN(a.String(), "/", 2)
			if as[0] == host {
				return true
			}
		}
	}
	return false
}

func dialUnix(port string, tlscfg *tls.Config) (net.Conn, error) {
	tlscfg = nil
	addr, err := net.ResolveUnixAddr("unix", port)
	if err != nil {
		return nil, err
	}
	return net.DialUnix("unix", nil, addr)
}

func dialTCP(host, port string, tlscfg *tls.Config) (net.Conn, error) {
	addr, err := net.ResolveTCPAddr("tcp", host+":"+port)
	if err != nil {
		return nil, err
	}
	c, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return nil, err
	}
	// Beware this is not enough if you have NATs
	c.SetKeepAlivePeriod(2 * time.Second)
	c.SetKeepAlive(true)
	if tlscfg != nil {
		return tls.Client(c, tlscfg), nil
	}
	return c, nil
}

func dial(addr string, tlscfg *tls.Config) (c net.Conn, err error) {
	nw, host, svc := ParseAddr(addr)
	port := Port(nw, svc)
	err = ErrBadAddr
	if nw == "*" || nw == "unix" {
		if IsLocal(host) {
			if c, err = dialUnix(port, tlscfg); err == nil {
				return c, nil
			}
		} else {
			err = ErrNotLocal
		}
		if nw == "unix" {
			return nil, err
		}
	}
	if nw == "tls" && tlscfg == nil {
		tlscfg = ClientTLSCfg
		if tlscfg == nil {
			return nil, ErrNoTLSCfg
		}
	}
	if nw == "*" || nw == "tcp" || nw == "tls" {
		if host == "local" || host == "localhost" || host == "*" {
			host = "127.0.0.1"
		}
		if c, err = dialTCP(host, port, tlscfg); err == nil {
			return c, nil
		}
	}
	return nil, err
}

// Dial the given address and return a point to point connection.
// The connection is secured if tlscfg is not nil.
func Dial(addr string, tlscfg ...*tls.Config) (c ch.Conn, err error) {
	var cfg *tls.Config
	if len(tlscfg) > 0 {
		cfg = tlscfg[0]
	}
	if nc, err := dial(addr, cfg) ; err == nil {
		c = ch.NewConn(nc, 0, nil)
		c.Tag = addr
		return c, nil
	}
	return c, err
}

// Dial the given address and return a muxed connection
// The connection is secured if tlscfg is not nil.
func MuxDial(addr string, tlscfg ...*tls.Config) (m *ch.Mux, err error) {
	var cfg *tls.Config
	if len(tlscfg) > 0 {
		cfg = tlscfg[0]
	}
	if nc, err := dial(addr, cfg) ; err == nil {
		m = ch.NewMux(nc, true)
		m.Tag = addr
		return m, nil
	}
	return nil, err
}

func serveLoop(l net.Listener, rc chan ch.Conn, ec chan bool,
		addr, tag string, tlscfg *tls.Config, ismux bool) {
	go func() {
		<-ec
		l.Close()
	}()
	if strings.HasPrefix(addr, "/tmp/") {
		defer os.Remove(addr)
	}
	var err error
	for {
		fd, e := l.Accept()
		if e != nil {
			err = e
			break
		}
		raddr := fd.RemoteAddr().String()
		if n := strings.LastIndex(raddr, ":"); n > 0 {
			raddr = raddr[:n] + "!" + raddr[n+1:]
		}
		if tlscfg != nil {
			fd = tls.Server(fd, tlscfg)
		}		
		if !ismux {
			cn := ch.NewConn(fd, 0, nil)
			if ok := rc <- cn; !ok {
				err = cerror(rc)
				break
			}
			continue
		}
		mux := ch.NewMux(fd, false)
		mux.Tag = raddr
		go func() {
			<-ec
			mux.Close()
		}()
		go func() {
			for cn := range mux.In {
				if ok := rc <- cn; !ok {
					close(mux.In, cerror(rc))
					close(ec, cerror(rc))
					break
				}
			}
		}()
	}
	l.Close()
	close(rc, err)
	close(ec, err)
}

func serve1(nw, host, port string, tlscfg *tls.Config, ismux bool) (c <-chan ch.Conn, ec chan bool, err error) {
	tag := fmt.Sprintf("%s!%s!%s", nw, host, port)
	if nw == "tls" {
		nw = "tcp"
		if tlscfg == nil {
			tlscfg = ServerTLSCfg
			if tlscfg == nil {
				return nil, nil, ErrNoTLSCfg
			}
		}
	}
	if nw == "tcp" && (host == "local" || host == "*" || host == "localhost") {
		host = ""
	}
	addr := host+":"+port
	if nw == "unix" {
		addr = port
		tlscfg = nil
	}
	dbg.Warn("listen at %s (%s:%s)", tag, nw, addr)
	fd, err := net.Listen(nw, addr)
	if err != nil {
		return nil, nil, err
	}
	rc := make(chan ch.Conn)
	rec := make(chan bool)
	go serveLoop(fd, rc, rec, addr, tag, tlscfg, ismux)
	return rc, rec, nil
}

func serveBoth(c1 <-chan ch.Conn, ec1 chan<-bool, c2 <-chan ch.Conn, ec2 chan bool) (
	c <-chan ch.Conn, ec chan bool, err error) {
	xc := make(chan ch.Conn)
	xec := make(chan bool)
	go func() {
		var err error
		doselect {
		case cn, ok := <-c1:
			if !ok {
				err = cerror(c1)
				if c2 == nil {
					break
				}
				c1 = nil
			}
			if ok = xc <- cn; !ok {
				err = cerror(xc)
				break
			}
		case cn, ok := <-c2:
			err = cerror(c2)
			if !ok {
				if c1 == nil {
					break
				}
				c2 = nil
			}
			if ok = xc <- cn; !ok {
				err = cerror(xc)
				break
			}
		case <-xec:
			err = cerror(xec)
			break
		}
		close(ec1, err)
		close(ec2, err)
		close(c1, err)
		close(c2, err)
		close(xec, err)
		close(xc, err)
	}()
	return xc, xec, nil
}

func serve(addr string, tlscfg *tls.Config, ismux bool) (c <-chan ch.Conn, ec chan bool, err error) {
	nw, host, svc := ParseAddr(addr)
	if !IsLocal(host) {
		return nil, nil, ErrNotLocal
	}
	port := Port(nw, svc)
	switch nw {
	case "*":
		uc, uec, uerr := serve1("unix", host, port, tlscfg, ismux)
		if uerr != nil {
			return serve1("tcp", host, port, tlscfg, ismux)
		}
		tc, tec, terr := serve1("tcp", host, port, tlscfg, ismux)
		if terr != nil {
			return uc, uec, uerr
		}
		return serveBoth(uc, uec, tc, tec)
	case "unix", "tcp", "tls":
		return serve1(nw, host, port, tlscfg, ismux)
	default:
		return nil, nil, ErrBadAddr
	}
}

// Serve the given address and return a chan to receive connections from new clients
// and atermination channel. The termination channel can be closed by the caller
// to stop the service, and will be closed if the underlying
// transport fails and the service can't continue.
// If the requested service name is already being served or any
// other error happens, the error is returned (along with two nil channels).
// If the network is "*", the service will be started on all networks.
// The connections are secured if tlscfg is not nil.
func Serve(addr string, tlscfg ...*tls.Config) (c <-chan ch.Conn, ec chan bool, err error) {
	var cfg *tls.Config
	if len(tlscfg) > 0 {
		cfg = tlscfg[0]
	}
	return serve(addr, cfg, false)
}

// Serve the given address and return a chan to receive muxed connections from new clients
// and a termination channel. The termination channel can be closed by the caller
// to stop the service, and will be closed if the underlying
// transport fails and the service can't continue.
// If the requested service name is already being served or any
// other error happens, the error is returned (along with two nil channels).
// If the network is "*", the service will be started on all networks.
// The connections are secured if tlscfg is not nil.
func MuxServe(addr string, tlscfg ...*tls.Config) (c <-chan ch.Conn, ec chan bool, err error) {
	var cfg *tls.Config
	if len(tlscfg) > 0 {
		cfg = tlscfg[0]
	}
	return serve(addr, cfg, true)
}

// Build a TLS config for use with dialing functions provided by others.
func TLSCfg(name string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(name + ".pem", name+".key")
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}, nil
}
