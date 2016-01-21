/*
	Clive web interfaces and tools
*/
package web

import (
	"net/http"
	"clive/cmd"
	"clive/x/code.google.com/p/go.net/websocket"
	"encoding/json"
)

// events to/from the page
struct Ev  {
	Id, Src string   // target id and source
	Vers    int      // version of the control the event is for
	Args    []string // events with string arguments
	Data    []byte   // all other events
}

// parse a event
func ParseEv(data []byte) (*Ev, error) {
	ev := &Ev{}
	err := json.Unmarshal(data, ev)
	return ev, err
}

// element controler, provides a chan interface for a websocket
// connection to the element involved.
struct Ctlr {
	Id string	// unique id for the controlled element
	In <-chan *Ev	// input events (from the page)
	Out chan<- *Ev	// output events (sent to the page)
	in, out chan *Ev	// to send to Out
	Ev <-chan *Ev	// dup of events for the user
}

func NewCtlr(id string) *Ctlr {
	out := make(chan *Ev)
	in := make(chan *Ev)
	c := &Ctlr{
		Id: id,
		In: in,
		in: in,
		out: out,
		Out: out,
	}
	http.Handle("/ws/" + c.Id, websocket.Handler(c.server))
	return c
}

func (c *Ctlr) server(ws *websocket.Conn) {
	cmd.Dprintf("%s: ws started\n", c.Id)
	defer cmd.Dprintf("%s: ws reader done\n", c.Id)
	defer ws.Close()
	go func() {
		defer cmd.Dprintf("%s: ws writer done\n", c.Id)
		for ev := range c.out {
			m, err := json.Marshal(ev)
			if err != nil {
				cmd.Dprintf("%s: update: marshal: %s\n", c.Id, err)
				return
			}
			cmd.Dprintf("%s: update: %v\n", c.Id, ev)
			if _, err := ws.Write(m); err != nil {
				cmd.Dprintf("%s: update: %v wr: %s\n", c.Id, ev, err)
				return
			}
		}
		close(c.In, cerror(c.out))
		close(c.Ev, cerror(c.out))
	}()
	var buf [8*1024]byte
	for {
		n, err := ws.Read(buf[0:])
		if err != nil {
			cmd.Dprintf("%s: server read: %s\n", c.Id, err)
			return
		}
		if n == 0 {
			continue
		}
		ev, err := ParseEv(buf[:n])
		if err != nil {
			cmd.Dprintf("%s: ev parse: %s\n", c.Id, err)
			continue
		}
		cmd.Dprintf("%s: ev %v\n", c.Id, ev)
		if ok := c.in <- ev; !ok {
			err := cerror(c.in)
			cmd.Dprintf("%s: in closed %v", c.Id, err)
			close(c.out, err)
			close(c.Ev, err)
			break
		}
	}
}
