package srv

import (
	"clive/nchan"
	"net"
)

type ch struct {
	ncc chan *nchan.Conn
}

// Make a CliHandler that accepts new clients and sends one Conn for each client
// to the channel returned.
func NewChanHandler() (CliHandler, chan *nchan.Conn) {
	ncc := make(chan *nchan.Conn, 1)
	return ch{ncc: ncc}, ncc
}

// Handle one client, called by net.Srv.
func (h ch) HandleCli(c net.Conn, endc chan bool) {
	inc := make(chan bool)
	outc := make(chan bool)
	nc := nchan.NewConn(c, 5, inc, outc)
	nc.Tag = c.RemoteAddr().String()
	h.ncc <- &nc
	doselect {
	case <-inc:
		inc = nil
		if inc == nil && outc == nil {
			return
		}
	case <-outc:
		outc = nil
		if inc == nil && outc == nil {
			return
		}
	case <-endc:
		close(nc.In, "server closing")
		close(nc.Out, "server closing")
		c.Close()
	}
}
