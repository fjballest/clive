package net

import (
	"clive/ch"
	"clive/dbg"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
)

// Dial the given address and return a muxed connection
// The connection is secured if tlscfg is not nil.
func MuxDial(addr string, tlscfg ...*tls.Config) (m *ch.Mux, err error) {
	var cfg *tls.Config
	if len(tlscfg) > 0 {
		cfg = tlscfg[0]
	}
	nc, err := dial(addr, cfg)
	if err == nil {
		m = ch.NewMux(nc, true)
		m.Tag = addr
		go func() {
			for _ = range m.In {
			}
		}()
		return m, nil
	}
	return nil, err
}

func serveMuxLoop(l net.Listener, rc chan *ch.Mux, ec chan bool,
	addr, tag string, tlscfg *tls.Config) {
	if strings.HasPrefix(addr, "/tmp/") {
		defer os.Remove(addr)
	}
	closes := map[io.Closer]bool{}
	var closeslk sync.Mutex
	go func() {
		<-ec
		l.Close()
		closeslk.Lock()
		for c := range closes {
			c.Close()
		}
		closeslk.Unlock()
	}()
	var err error
	ncli := 0
	for {
		fd, e := l.Accept()
		if e != nil {
			err = e
			break
		}
		closeslk.Lock()
		var cfd io.Closer = fd
		closes[cfd] = true
		closeslk.Unlock()
		raddr := fd.RemoteAddr().String()
		if raddr == "" {
			// unix sockets do not provide raddr
			raddr = fmt.Sprintf("local!%d", ncli)
			ncli++
		} else {
			if n := strings.LastIndex(raddr, ":"); n > 0 {
				raddr = raddr[:n] + "!" + raddr[n+1:]
			}
		}
		if tlscfg != nil {
			fd = tls.Server(fd, tlscfg)
		}
		mux := ch.NewMux(fd, false)
		mux.Tag = raddr
		if ok := rc <- mux; !ok {
			close(mux.In, cerror(rc))
			close(ec, cerror(rc))
			break
		}
	}
	close(rc, err)
	close(ec, err)
}

func serveMux1(nw, host, port string, tlscfg *tls.Config) (c <-chan *ch.Mux, ec chan bool, err error) {
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
	addr := host + ":" + port
	if nw == "unix" {
		addr = port
		tlscfg = nil
		os.Remove(port)
	}
	dbg.Warn("listen at %s (%s:%s)", tag, nw, addr)
	fd, err := net.Listen(nw, addr)
	if err != nil {
		return nil, nil, err
	}
	rc := make(chan *ch.Mux)
	rec := make(chan bool)
	go serveMuxLoop(fd, rc, rec, addr, tag, tlscfg)
	return rc, rec, nil
}

func serveMuxBoth(c1 <-chan *ch.Mux, ec1 chan<- bool,
	c2 <-chan *ch.Mux, ec2 chan bool) (c <-chan *ch.Mux, ec chan bool, err error) {
	xc := make(chan *ch.Mux)
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

// Serve the given address and return a chan to receive muxed connections from new clients
// and a termination channel. The termination channel can be closed by the caller
// to stop the service, and will be closed if the underlying
// transport fails and the service can't continue.
// If the requested service name is already being served or any
// other error happens, the error is returned (along with two nil channels).
// If the network is "*", the service will be started on all networks.
// The connections are secured if tlscfg is not nil.
func MuxServe(addr string, tlscfg ...*tls.Config) (c <-chan *ch.Mux, ec chan bool, err error) {
	var cfg *tls.Config
	if len(tlscfg) > 0 {
		cfg = tlscfg[0]
	}
	nw, host, svc := ParseAddr(addr)
	if !IsLocal(host) {
		return nil, nil, ErrNotLocal
	}
	switch nw {
	case "*":
		port := Port("unix", svc)
		uc, uec, uerr := serveMux1("unix", host, port, cfg)
		if uerr != nil {
			return serveMux1("tcp", host, port, cfg)
		}
		port = Port("tcp", svc)
		tc, tec, terr := serveMux1("tcp", host, port, cfg)
		if terr != nil {
			return uc, uec, uerr
		}
		return serveMuxBoth(uc, uec, tc, tec)
	case "unix", "tcp", "tls":
		port := Port("unix", svc)
		return serveMux1(nw, host, port, cfg)
	default:
		return nil, nil, ErrBadAddr
	}
}
