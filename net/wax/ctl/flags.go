package ctl

import (
	"clive/net/wax"
	"fmt"
)

type Control struct {
	names []string
	set   []func(string)
	get   []func() string
}

func NewControl() *Control {
	return &Control{}
}

func (c *Control) Flag(name string, set func(string), get func() string) {
	c.names = append(c.names, name)
	c.set = append(c.set, set)
	c.get = append(c.get, get)
}

func (c *Control) BoolFlag(name string, v *bool) {
	c.Flag(name,
		func(to string) { *v = to == "on" },
		func() string {
			if *v {
				return "on"
			}
			return "off"
		})

}

func (c *Control) CmdFlag(name string, v func()) {
	c.Flag(name,
		func(to string) {
			if to == "on" {
				v()
			}
		},
		nil)

}

func (c *Control) Bar(id string) *Bar {
	id += "_0"
	b := []interface{}{}
	evc := make(chan *wax.Ev)
	updc := make(chan *wax.Ev, len(c.names))
	for i := 0; i < len(c.names); i++ {
		if c.get[i] != nil {
			b = append(b, Check(c.names[i]))
			bid := fmt.Sprintf("%s_%d", id, i)
			on := c.get[i]()
			updc <- &wax.Ev{Id: bid, Args: []string{"Set", on}}
		} else {
			b = append(b, Button(c.names[i]))
		}
	}
	tb := NewBar(evc, updc, b...)
	go func() {
		for ev := range evc {
			if ev == nil {
				break
			}
			args := ev.Args
			if len(args) < 2 {
				continue
			}
			i := ev.CtlId()
			if i >= 0 && i < len(c.names) {
				fmt.Printf("set %s %s\n", c.names[i], ev.Args[1])
				c.set[i](ev.Args[1])
			}
		}
	}()
	return tb
}
