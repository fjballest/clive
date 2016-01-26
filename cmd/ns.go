package cmd

import (
	"clive/u"
	"clive/ns"
	"clive/dbg"
	"sync"
	"os"
	fpath "path"
	"fmt"
	"clive/zx"
)

type cwd struct {
	path string // "" means use the OS one.
	sync.Mutex
}

// return p as a cleaned and absolute path for the current context.
func AbsPath(p string) string {
	p = fpath.Clean(p)
	if len(p) == 0 || p[0] != '/' {
		d := Dot()
		p = fpath.Join(d, p)
	}
	return p
}

// Initialize a  new dot from the os
func mkDot() *cwd {
	dot := &cwd{}
	dot.path, _ = os.Getwd()
	if dot.path == "" {
		dot.path = "/"
	}
	return dot
}

func (c *cwd) set(d string) error {
	d = AbsPath(d)
	nd, err := Stat(d)
	if err != nil {
		return err
	}
	if nd["type"] != "d" {
		return fmt.Errorf("%s: %s", d, zx.ErrNotDir)
	}
	c.Lock()
	defer c.Unlock()
	c.path = d
	os.Chdir(d)	// in case it exists and we exec...
	return nil
}

func (c *cwd) get() string {
	c.Lock()
	defer c.Unlock()
	return c.path
}

func (c *cwd) dup() *cwd {
	c.Lock()
	defer c.Unlock()
	nc := &cwd{path: c.path}
	return nc
}

func mkNS() *ns.NS {
	s := GetEnv("NS")
	if s == ""  {
		nsf := fpath.Join(u.Home, "NS")
		if fi, err := os.Stat(nsf); err == nil && !fi.IsDir() {
			s = nsf
		} else {
			s = "/"
		}
		SetEnv("NS", s)
	}
	n, err := ns.Parse(s)
	if err != nil {
		dbg.Warn("mkNS: %s", err)
		n, _ = ns.Parse("/")
	}
dbg.Warn("ns %v", n)
	return n
}
