package wax

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

/*
	Someone that knows how to present  and accepts events and
	updates.
	For example, data types in wax/ctl implement this interface and
	can be presented by using $name$ where name refers to any of them.
*/
type Controller interface {
	Presenter
	Mux(evc, updc chan *Ev) string // carried on events/updates for this ctlr.
}

/*
	For wax controls implementor.

	An event or update message
	It has the id for the control responsible for the event or update plus
	raw event data. The event data is encoded in JSON if it's a complex object
	(other than a string, []string or []byte) and the entire event is also encoded in JSON.
*/
type Ev  {
	Id, Src string   // target id and source
	Vers    int      // version of the control the event is for
	Args    []string // events with string arguments
	Data    []byte   // all other events
}

/*
	For wax controls implementor.

	Make a new event for the given id and data
*/
func NewEvMsg(id string, data interface{}) ([]byte, error) {
	ev := Ev{Id: id}
	switch v := data.(type) {
	case string:
		ev.Args = []string{v}
	case []string:
		ev.Args = v
	case []byte:
		ev.Data = v
	default:
		raw, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		ev.Data = raw
	}
	return json.Marshal(ev)
}

/*
	For wax controls implementor.

	Parse a received event
*/
func ParseEv(data []byte) (*Ev, error) {
	ev := &Ev{}
	err := json.Unmarshal(data, ev)
	return ev, err
}

func (e *Ev) reflects() bool {
	if e==nil || len(e.Args)==0 || len(e.Args[0])==0 {
		return false
	}
	return e.Args[0][0]>='A' && e.Args[0][0]<='Z'
}

/*
	For wax controls implementor.

	A connection to a muxed upper control.
	Can be used as part of a control and provides a
	Mux() method. The id reported  is
	Conn.Id + "_0".
*/
type Conn  {
	Id        string
	Evc, Updc chan *Ev
	DoClose   bool
	sync.Mutex
	once sync.Once
	outc []chan *Ev
}

func (c *Conn) wants(id string) bool {
	cid := c.Id + "_0"
	return c.Id=="" || cid==id ||
		len(cid)<len(id) && id[len(cid)]=='_'
}

/*
	Adds evc as an event chan to the mux and updc as an
	update chan to the mux. Either might be nil.
	This method is usually called multiple times to mux multiple chans.

	Events sent to evc are sent to c.Evc and updates sent to
	c.Updc are sent to updc as well.

	If c.DoClose is set, the updc added to the mux is closed when
	c.Updc is done.
*/
func (c *Conn) Mux(evc, updc chan *Ev) string {
	c.Lock()
	defer c.Unlock()
	if c.Updc != nil {
		if c.outc == nil {
			c.outc = []chan *Ev{}
			go func() {
				for x := range c.Updc {
					//fmt.Printf("mux %q upd %v\n", c.Id, x)
					c.Lock()
					for i := 0; i < len(c.outc); i++ {
						cc := c.outc[i]
						if cc == nil {
							continue
						}
						c.Unlock()
						if ok := cc <- x; !ok {
							c.outc[i] = nil
						}
						c.Lock()
					}
					c.Unlock()
				}
				if !c.DoClose {
					return
				}
				c.Lock()
				err := cerror(c.Updc)
				if err == nil {
					err = fmt.Errorf("Updc closed")
				}
				for i := 0; i < len(c.outc); i++ {
					if c.outc[i] != nil {
						close(c.outc[i], err)
					}
				}
				c.Unlock()
			}()
		}
		c.outc = append(c.outc, updc)
	} else if c.DoClose && updc!=nil {
		close(updc, "no Updc")
	}

	if evc != nil {
		cevc := c.Evc
		go func() {
			for x := range evc {
				//fmt.Printf("mux %q ev %v\n", c.Id, x)
				if cevc!=nil && x!=nil && c.wants(x.Id) {
					if ok := cevc <- x; !ok {
						if c.DoClose {
							close(evc, cerror(c.Evc))
						}
						return
					}
				}
			}
		}()
	}
	return c.Id + "_0"
}

/*
	For wax control implementors.

	get the class id for a ctl id.
*/
func ClassId(id string) string {
	els := strings.SplitN(id, "_", 3)
	switch len(els) {
	case 2:
		return els[0] + "_0"
	case 3:
		return fmt.Sprintf("%s_0_%s", els[0], els[2])
	default:
		return id
	}
}

/*
	For wax control implementors.

	get the last id from the event id.
*/
func (e *Ev) CtlId() int {
	els := strings.Split(e.Id, "_")
	if len(els) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(els[len(els)-1])
	return n
}
