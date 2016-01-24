/*
	Clive web interfaces and tools
*/
package web

import (
	"net/http"
	"clive/cmd"
	"clive/x/code.google.com/p/go.net/websocket"
	"encoding/json"
	"sync"
	"fmt"
	"io"
)

// Events to/from a control
// Args[0] is the event name
// If the name starts with uppercase, it does reflect and all views
// get an automatic copy of the event.
struct Ev  {
	Id, Src string   // element id and source view id (eg txt1, txt1_3)
	Vers    int      // version of the control the event is for
	Args    []string // events with string arguments
	Data    []byte   // all other events
}

struct view {
	Id string		// set by the eid event
	out chan *Ev	// events from/to this view
}

// Element controler, provides a chan interface for a page interface element,
// running over a web socket to the element.
// Supports multiple views and reflects events to synchronize them.
// All controls export this public interface.
struct Ctlr {
	Id string	// unique id for the controlled element
	closec chan bool
	in, out chan *Ev	// input events (from the page), and output events
	evs chan *Ev
	sync.Mutex
	nb int
	views map[*view]bool
}

// HTML headers to be included in pages using this interface.
var headers = `
<link rel="stylesheet" href="/js/jquery-ui/jquery-ui.css">
<script src="/js/jquery-2.2.0.js"></script>
<script type="text/javascript" src="/js/clive.js"></script>
<script type="text/javascript" src="/js/txt.js"></script>
<script type="text/javascript" src="/js/txt.js"></script>
<script type="text/javascript" src="/js/button.js"></script>
<script type="text/javascript" src="/js/radio.js"></script>
<script src="/js/jquery-ui/jquery-ui.js"></script>
<script src="/js/jquery.get-word-by-event.js"></script>
`

var (
	idgen int
	idlk sync.Mutex
)

// Write headers to a page so it can support controls.
// Not needed for pages created with NewPg.
func WriteHeaders(w io.Writer) {
	io.WriteString(w, headers)
}

func newId() int {
	idlk.Lock()
	defer idlk.Unlock()
	idgen++
	return idgen
}

// parse a event
func parseEv(data []byte) (*Ev, error) {
	ev := &Ev{}
	err := json.Unmarshal(data, ev)
	return ev, err
}

// Create a new control.
// This is done by all controls during their creation.
func newCtlr(tag string) *Ctlr {
	c := &Ctlr{
		Id: fmt.Sprintf("%s%d", tag, newId()),
		in: make(chan *Ev),
		out: make(chan *Ev),
		views: make(map[*view]bool),
		closec: make(chan bool),
	}
	http.Handle("/ws/" + c.Id, websocket.Handler(c.server))
	go c.reflector()
	return c
}

// Terminate the operation of the control and remove it from pages.
func (c *Ctlr) Close() error {
	close(c.closec)
	close(c.in, "closed")
	close(c.out, "closed")
	close(c.evs, "closed")
	http.Handle("/ws" + c.Id, nil)
	return nil
}

// Wait for the control to be closed.
func (c *Ctlr) Wait() {
	<-c.closec
}

// Close the view of this control with the given id.
func (c *Ctlr) CloseView(id string) {
	c.Lock()
	defer c.Unlock()
	for v := range c.views {
		if v.Id == id {
			v.out <- &Ev{Id: v.Id, Src: v.Id, Args: []string{"close"}}
			return
		}
	}
}

// Return true if the control is closed.
func (c *Ctlr) Closed() bool {
	select {
	case <-c.closec:
		return true
	default:
		return false
	}
}

// Return the (application) event channel for the control.
func (c *Ctlr) Events() <-chan *Ev {
	c.Lock()
	defer c.Unlock()
	if c.evs == nil {
		c.evs = make(chan *Ev)
	}
	return c.evs
}

func (c *Ctlr) post(ev *Ev) error {
	c.Lock()
	ec := c.evs
	c.Unlock()
	if ec == nil {
		return nil
	}
	if ok := ec <- ev; !ok {
		return cerror(ec)
	}
	return nil
}

// Return the list of identifiers of the current views of the control.
func (c *Ctlr) Views() []string {
	c.Lock()
	defer c.Unlock()
	vs := make([]string, 0, len(c.views))
	for v := range c.views {
		if v.Id != "" {
			vs = append(vs, v.Id)
		}
	}
	return vs
}

func (c *Ctlr) viewOut(id string) chan<- *Ev {
	c.Lock()
	defer c.Unlock()
	for v := range c.views {
		if v.Id == id {
			return v.out
		}
	}
	rc := make(chan *Ev)
	close(rc)
	return rc
}

func (c *Ctlr) newViewId() string {
	c.Lock()
	defer c.Unlock()
	c.nb++
	return  fmt.Sprintf("%s_%d", c.Id, c.nb)
}

func (e *Ev) reflects() bool {
	if e==nil || len(e.Args)==0 || len(e.Args[0])==0 {
		return false
	}
	return e.Args[0][0]>='A' && e.Args[0][0]<='Z'
}

func (c *Ctlr) reflector() {
	for ev := range c.out {
		ev := ev
		c.Lock()
		for v := range c.views {
			if ev.Src != v.Id {
				cmd.Dprintf("%s: reflecting %v\n", v.Id, ev.Args)
				go func(v *view) {
					v.out <- ev
				}(v)
			}
		}
		c.Unlock()
	}
	c.Lock()
	err := cerror(c.out)
	for v := range c.views {
		close(v.out, err)
	}
	close(c.evs, err)
	c.Unlock()
}

func (c *Ctlr) newView() *view {
	c.Lock()
	defer c.Unlock()
	v := &view{
		out: make(chan *Ev),
	}
	c.views[v] = true
	return v
}

func (c *Ctlr) delView(v *view) {
	close(v.out, "closed")
	c.Lock()
	delete(c.views, v)
	c.Unlock()
}

func (c *Ctlr) server(ws *websocket.Conn) {
	cmd.Dprintf("%s: ws started\n", c.Id)
	v := c.newView()
	defer func() {
		cmd.Dprintf("%s: ws reader done\n", c.Id)
		ws.Close()
		c.delView(v)
	}()
	go func() {
		defer cmd.Dprintf("%s: ws writer done\n", c.Id)
		defer c.delView(v)
		for ev := range v.out {
			m, err := json.Marshal(ev)
			if err != nil {
				cmd.Dprintf("%s: update: marshal: %s\n", c.Id, err)
				close(v.out, err)
				break
			}
			cmd.Dprintf("%s: update: %v\n", c.Id, ev)
			if _, err := ws.Write(m); err != nil {
				cmd.Dprintf("%s: update: %v wr: %s\n", c.Id, ev, err)
				close(v.out, err)
				break
			}
		}
	}()
	var buf [8*1024]byte
	for {
		n, err := ws.Read(buf[0:])
		if err != nil {
			cmd.Dprintf("%s: server read: %s\n", c.Id, err)
			close(v.out, err)
			break
		}
		if n == 0 {
			continue
		}
		ev, err := parseEv(buf[:n])
		if err != nil {
			cmd.Dprintf("%s: ev parse: %s\n", c.Id, err)
			continue
		}
		cmd.Dprintf("%s: ev %v\n", c.Id, ev)
		if len(ev.Args) == 1 && ev.Args[0] == "id" && v.Id == "" {
			v.Id = ev.Src
			c.in <- &Ev{Id: c.Id, Src: v.Id, Args: []string{"start"}}
			continue
		}
		if ok := c.in <- ev; !ok {
			err := cerror(c.in)
			cmd.Dprintf("%s: in closed %v", c.Id, err)
			close(v.out, err)
			break
		}
		if ev.reflects() {
			c.out <- ev
		}
	}
	if v.Id != "" {
		c.in <- &Ev{Id: c.Id, Src: v.Id, Args: []string{"end"}}
	}
}

