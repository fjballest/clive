package ds

import (
	"clive/dbg"
	"clive/nchan"
	"clive/net/fifo"
	"clive/net/srv"
	"errors"
	"fmt"
	"sync"
	"time"
)

/*
	Serve the given address and return a chan to receive connections to new clients
	from it and termination channel. The termination channel can be closed by the caller
	to stop the service. If the requested service name is already being served or any
	other error happens, the error is returned (along with two nil channels).

	The address may use "*" in the network and/or machine address to serve on
	multiple networks. In this case, if some of the networks work, a connection chan is
	returned, and the error returned is the first error found while
	trying to serve on the failing networks.

	The connection chan returned is closed only
	when all the known networks are gone, with the first error received from
	any of them.
*/
func Serve(tag, addr string) (<-chan *nchan.Conn, chan<- error, error) {
	net, mach, svc := ParseAddr(addr)
	switch net {
	case "pipe":
		hc := make(chan *nchan.Conn)
		ec := make(chan error)
		go func() {
			<-ec
			DelPipe(svc)
		}()
		NewPipe(svc, func() (nchan.Conn, error) {
			c1, c2 := nchan.NewConnPipe(1)
			cc := &nchan.Conn{}
			*cc = c1
			cc.Tag = "pipe"
			hc <- cc
			return c2, nil
		})
		return hc, ec, nil
	case "fifo":
		hs, hc := fifo.NewChanHandler()
		s := fifo.New(tag, svc, hs)
		if err := s.Serve(); err != nil {
			close(hc, err)
			return nil, nil, err
		}
		ec := make(chan error)
		go func() {
			<-ec
			s.Stop(false)
		}()
		return hc, ec, nil
	case "tcp", "tcp4", "tcp6":
		lk.Lock()
		if svcs[svc] != "" {
			svc = svcs[svc]
		}
		lk.Unlock()
		hs, hc := srv.NewChanHandler()
		s := srv.New(tag, net, mach, svc, hs)
		if err := s.Serve(); err != nil {
			return nil, nil, err
		}
		ec := make(chan error)
		go func() {
			<-ec
			s.Stop(false)
		}()
		return hc, ec, nil
	case "*":
		nets := [...]string{"pipe", "fifo", "tcp"}
		var err error
		var ccs []<-chan *nchan.Conn
		var ecs []chan<- error
		for _, net = range nets {
			addr := fmt.Sprintf("%s!%s!%s", net, mach, svc)
			cc, ec, serr := Serve(tag, addr)
			if serr != nil {
				dbg.Warn("serve %s: %s", addr, serr)
			}
			if serr!=nil && err==nil {
				err = serr
			}
			if cc != nil {
				dbg.Warn("listen %s", addr)
				ccs = append(ccs, cc)
				ecs = append(ecs, ec)
			}
		}
		switch len(ccs) {
		case 0:
			return nil, nil, err
		case 1:
			return ccs[0], ecs[0], nil
		}
		hc := make(chan *nchan.Conn)
		ec := make(chan error, len(ccs))
		for i := 0; i < len(ccs); i++ {
			go func(i int) {
				for x := range ccs[i] {
					if ok := hc <- x; !ok {
						close(ccs[i], cerror(hc))
						close(ecs[i], cerror(hc))
						break
					}
				}
				ec <- cerror(ccs[i])
			}(i)
		}
		uec := make(chan error)
		go func() {
			var err error
			for i := 0; i < len(ccs); i++ {
				if e := <-ec; e != nil {
					err = e
				}
			}
			close(hc, err)
			close(uec, err)
		}()
		go func() {
			<-uec
			for i := 0; i < len(ecs); i++ {
				close(ecs[i], cerror(uec))
			}
		}()
		return hc, uec, nil
	}
	return nil, nil, fmt.Errorf("ds: serve: unknown network '%s'", net)
}

/*
	Like Serve(), but enforcing at most one connection to each peer.
	It returns dialpeerc, where addresses may be sent to ask for a dial to a peer;
	hangpeerc, where addresses may be sent to close the connection to that peer;
	and newpeerc, where connections to new peers are posted.

	Peers are known either because they dial us or because we
	dial them (their addresses are sent to the address chan).

	Dialed or accepted, connections to new peers are sent to the newpeerc chan.
	Dial errors are notified by a zero nchan.Conn sent to the newpeerc chan
	with the tag "<failed addr> <error string>".

	Dialing an already connected peer is not an error, and it is ignored.
*/
func Peer(tag, addr string) (dialpeerc chan<- string, hangpeerc chan<- string, newpeerc <-chan *nchan.Conn, err error) {
	hc, ec, err := Serve(tag, addr)
	if err != nil {
		return nil, nil, hc, err
	}
	peerc := make(chan string)
	hangc := make(chan string)
	pc := make(chan *nchan.Conn)
	go func() {
		me := []byte(fmt.Sprintf("%s.%d", tag, time.Now().UnixNano()))
		var peerslk sync.Mutex
		peers := map[string]*nchan.Conn{"": {}} // reserve "" as used
		doselect {
		case h, ok := <-hc:
			if !ok {
				close(ec, cerror(hc))
				close(peerc, cerror(hc))
				close(hangc, cerror(peerc))
				return
			}
			go func() {
				msg := <-h.In
				id := string(msg)
				peerslk.Lock()
				already := peers[id] != nil
				if !already {
					peers[id] = h
				}
				peerslk.Unlock()
				if already {
					close(h.Out, "already")
					close(h.In, "already")
					return
				}
				h.Out <- me
				in := make(chan []byte)
				nc := &nchan.Conn{Tag: h.Tag + "!" + id, In: in, Out: h.Out}
				pc <- nc
				go func() {
					for m := range h.In {
						in <- m
					}
					close(in, cerror(h.In))
					peerslk.Lock()
					delete(peers, id)
					peerslk.Unlock()
				}()
			}()
		case addr := <-hangc:
			peerslk.Lock()
			if addr != "" {
				h := peers[addr]
				delete(peers, addr)
				if h != nil {
					close(h.In, "hangup")
					close(h.Out, "hangup")
				}
			}
			peerslk.Unlock()
		case addr, ok := <-peerc:
			if !ok {
				close(ec, cerror(hc))
				close(hc, cerror(peerc))
				close(hangc, cerror(peerc))
				return
			}
			go func() {
				c, err := Dial(addr)
				if err != nil {
					pc <- &nchan.Conn{Tag: addr + " " + err.Error()}
					return
				}
				if ok := c.Out <- me; !ok {
					close(c.In, "hangup")
					err := cerror(c.Out)
					if err == nil {
						err = errors.New("remote hangup")
					}
					if err.Error() != "already" {
						pc <- &nchan.Conn{Tag: addr + " " + err.Error()}
					}
					return
				}

				mid, ok := <-c.In
				if !ok {
					close(c.In, "hangup")
					close(c.Out, "hangup")
					err := cerror(c.In)
					if err == nil {
						err = errors.New("remote hangup")
					}
					if err.Error() != "already" {
						pc <- &nchan.Conn{Tag: addr + " " + err.Error()}
					}
					return
				}
				id := string(mid)
				peerslk.Lock()
				already := peers[id] != nil
				if !already {
					peers[id] = &c
				}
				peerslk.Unlock()
				if already {
					close(c.In, "already")
					close(c.Out, "already")
					return
				}
				in := make(chan []byte)
				nc := &nchan.Conn{Tag: c.Tag + "!" + id, In: in, Out: c.Out}
				pc <- nc
				go func() {
					for m := range c.In {
						in <- m
					}
					close(in, cerror(c.In))
					peerslk.Lock()
					delete(peers, id)
					peerslk.Unlock()
				}()
			}()
		}
	}()
	return peerc, hangc, pc, nil
}
