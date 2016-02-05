package main

import (
	"bytes"
	"fmt"
)

var (
	bltin = map[string] func(*Cmd, ...string) {
		"cmds": bcmds,
	}
)

func bcmds(c *Cmd, args ...string) {
	ed := c.ed
	ix := ed.ix
	var out bytes.Buffer
	ix.Lock()
	if len(ix.cmds) == 0 {
		fmt.Fprintf(&out, "no commands\n")
	}
	for i, c := range ix.cmds {
		fmt.Fprintf(&out, "%d\t%s\n", i, c.name)
	}
	ix.Unlock()
	s := out.String()
	c.printf("%s", s)
}
