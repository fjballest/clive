package main

import (
	"bytes"
	"fmt"
	"clive/cmd"
)

var (
	bltin = map[string] func(*Cmd, ...string) {
		"cmds": bcmds,
		"X": bX,
		"cd": bcd,
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

func (ed *Ed) menuLine() string {
	switch {
	case ed.temp:
		return "/ " + ed.tag
	case ed.win.IsDirty():
		return "* " + ed.tag
	default:
		return "- " + ed.tag
	}
}

func bX(c *Cmd, args ...string) {
	var out bytes.Buffer
	ix.Lock()
	if len(ix.eds) == 0 {
		fmt.Fprintf(&out, "no edits\n")
	}
	if ix.dot != nil {
		fmt.Fprintf(&out, "%s\n", ix.dot.menuLine())
	}
	for _, e := range ix.eds {
		if e != ix.dot {
			fmt.Fprintf(&out, "%s\n", e.menuLine())
		}
	}
	ix.Unlock()
	s := out.String()
	c.printf("%s", s)
}

func bcd(c *Cmd, args ...string) {
	if len(args) == 1 {
		c.printf("missing destination dir\n")
		return
	}
	if err := cmd.Cd(args[1]); err != nil {
		c.printf("cd: %s\n", err)
	} else {
		c.printf("dot: %s\n", cmd.Dot())
	}
}
