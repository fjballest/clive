/*
	Dial service.

	Ds is a general purpose dialing service for clive.
	It provides tools to dial and serve nchan connections and implements
	networks such as pipe (a network within the executing process) and fifo
	(a network relying on the machine FIFO devices).

*/
package ds

// REFERENCE(x): addr(3), network address conventions

import (
	"clive/nchan"
	"clive/net/auth"
	"clive/net/fifo"
	"crypto/tls"
	"errors"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

/*
	See NewPipe.
*/
type PipeMaker func() (nchan.Conn, error)

var (
	lk    sync.Mutex
	pipes = map[string]PipeMaker{}
	svcs  = map[string]string{
		"ns":  "8000",
		"sns": "8001",
		"zx":  "8002",
	}
)

/*
	Register maker as a pipe service so we could dial it.
	Each client dialing the service makes a call to PipeMaker, which
	should return the connection to reach the server along with any error
	indication.
*/
func NewPipe(name string, maker PipeMaker) {
	lk.Lock()
	pipes[name] = maker
	lk.Unlock()
}

/*
	Define name as the name for the service at the given TCP port.
	Can be used to provide the same service through the fifo/pipe networks
	and the TCP/IP network.
	Clive relies on some predefined services, including:

		ns	8000	name space
		sns	8001	shared name spaces
		zx	8002	zx
*/
func DefSvc(name, port string) {
	lk.Lock()
	svcs[name] = port
	lk.Unlock()
}

/*
	Remove name as a service in the pipe network.
*/
func DelPipe(name string) {
	lk.Lock()
	delete(pipes, name)
	lk.Unlock()
}

/*
	Return true if the machine address given seems to address the local host.
*/
func IsLocal(mach string) bool {
	if mach=="" || mach=="*" || mach=="localhost" {
		return true
	}
	if hn, err := os.Hostname(); err==nil && hn==mach {
		return true
	}
	if mach[0]=='[' && len(mach)>2 && mach[len(mach)-1]==']' {
		mach = mach[1 : len(mach)-1]
	}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			as := strings.SplitN(a.String(), "/", 2)
			if as[0] == mach {
				return true
			}
		}
	}
	return false
}

/*
	Dial the given address and return a (nchan.Conn) connection for it.
	The connection is not multiplexed, but you can create a nchan.Mux on it
	if so desired. Usually that happens after authentication takes place.

	Addresses are of the form

		network!address!service
		address!service.

	The network/address may be "*" to use any available.

	Known networks are: pipe, fifo, tcp, tcp4, tcp6.
	The net defaults to tcp.

	The pipe/fifo network ignores the address and relies only on the svc.
*/
func Dial(addr string) (nchan.Conn, error) {
	net, mach, svc := ParseAddr(addr)
	if net == "" {
		return nchan.Conn{}, errors.New("bad address")
	}
	if net == "*" {
		if IsLocal(mach) {
			if h, err := dial("pipe", "*", svc); err == nil {
				return h, nil
			}
			if h, err := dial("fifo", "*", svc); err == nil {
				return h, nil
			}
			if mach == "*" {
				mach = "localhost"
			}
		}
		net = "tcp"
	}
	lk.Lock()
	if net!="pipe" && net!="fifo" && svcs[svc]!="" {
		svc = svcs[svc]
	}
	lk.Unlock()
	c, err := dial(net, mach, svc)
	if c.In != nil {
		c.Tag = addr
	}
	return c, err
}

/*
	Parse an address and return its network, machine adress, and service name/number.
	When the address is invalid an empty network name is returned. See Dial for
	address naming conventions.
*/
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
		return "*", "*", args[0]
	case 2:
		return "tcp", args[0], args[1]
	default:
		return args[0], args[1], args[2]
	}
}

func dial(netw, addr, svc string) (nchan.Conn, error) {
	switch netw {
	case "tcp", "tcp4", "tcp6":
		if auth.TLSclient == nil {
			taddr, err := net.ResolveTCPAddr(netw, addr+":"+svc)
			if err != nil {
				return nchan.Conn{}, err
			}
			c, err := net.DialTCP(netw, nil, taddr)
			if err != nil {
				return nchan.Conn{}, err
			}
			nc := nchan.NewConn(c, 5, nil, nil)
			return nc, nil
		}
		taddr, err := net.ResolveTCPAddr(netw, addr+":"+svc)
		if err != nil {
			return nchan.Conn{}, err
		}
		c, err := net.DialTCP(netw, nil, taddr)
		if err != nil {
			return nchan.Conn{}, err
		}
		// Beware this is not enough if you have NATs
		c.SetKeepAlivePeriod(2*time.Second)
		c.SetKeepAlive(true)
		tc := tls.Client(c, auth.TLSclient)
		nc := nchan.NewConn(tc, 5, nil, nil)
		return nc, nil
	case "pipe":
		lk.Lock()
		defer lk.Unlock()
		dialer, ok := pipes[svc]
		if !ok {
			return nchan.Conn{}, errors.New("no such pipe")
		}
		return dialer()
	case "fifo":
		return fifo.Dial(svc)
	default:
		return nchan.Conn{}, errors.New("unknown network")
	}
}
