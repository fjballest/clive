package app

import (
	"clive/dbg"
	"clive/nspace"
	"clive/zx"
	"clive/zx/lfs"
	"errors"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

type cwd struct {
	path string // "" means use the OS one.
	lk   sync.Mutex
}

// Initialize a new dot from the os
func mkdot() *cwd {
	dot := &cwd{}
	dot.path, _ = os.Getwd()
	if dot.path == "" {
		dot.path = "/"
	}
	return dot
}

func (c *cwd) set(d string) {
	d = AbsPath(d)
	c.lk.Lock()
	defer c.lk.Unlock()
	c.path = d
}

func (c *cwd) get() string {
	c.lk.Lock()
	defer c.lk.Unlock()
	return c.path
}

// Initialize a new dot in the current context from the one given.
// If the one given is "", it is initialized from the underlying OS.
// The path must be absolute.
func NewDot(d string) {
	d = strings.TrimSpace(d)
	c := ctx()
	c.lk.Lock()
	if d == "" {
		c.dot = mkdot()
	} else {
		d = path.Clean(d)
		if d == "" || d[0] != '/' {
			dbg.Fatal("Newdot: not abs path")
		}
		c.dot = &cwd{path: d}
	}
	c.lk.Unlock()
}

func Dot() string {
	c := ctx()
	return c.Dot()
}

func (c *Ctx) Dot() string {
	c.lk.Lock()
	dot := c.dot
	c.lk.Unlock()
	return dot.get()
}

// Start using a dup of the current dot.
func DupDot() {
	NewDot(Dot())
}

func Cd(p string) {
	c := ctx()
	c.lk.Lock()
	dot := c.dot
	c.lk.Unlock()
	dot.set(p)
}

// return p as a cleaned and absolute path for the current context.
func AbsPath(p string) string {
	p = path.Clean(p)
	if len(p) == 0 || p[0] != '/' {
		dot := Dot()
		p = path.Join(dot, p)
	}
	return p
}

// create a new NS from a textual description.
func MkNS(s string) zx.Finder {
	if s != "" {
		ns, err := nspace.Parse(s)
		if err != nil {
			Fatal("ns: %s", err)
		}
		dprintf("ns is %s\n", ns)
		ns.Debug = Debug && Verb
		ns.DebugFind = Debug && Verb
		return ns
	}
	fs, err := lfs.New("lfs", "/", lfs.RW)
	if err != nil {
		Fatal("lfs: %s", err)
	}
	fs.SaveAttrs(true)
	fs.Dbg = Debug && Verb
	return fs
}

// Return the ns in this context.
func (c *Ctx) NS() zx.Finder {
	c.lk.Lock()
	defer c.lk.Unlock()
	return c.ns
}

// Return the current name space
func NS() zx.Finder {
	c := ctx()
	return c.NS()
}

// Start using a new ns.
// If the given one is nil, the ns is re-initialized from that in the underlying os
func NewNS(f zx.Finder) {
	if f == nil {
		f = MkNS(GetEnv("NS"))
	}
	c := ctx()
	c.lk.Lock()
	defer c.lk.Unlock()
	c.ns = f
}

// Try to dup the current ns.
// Works only if it's a nspace.Tree.
// If it's a lfs then it's a nop.
// Otherwise it's a fatal.
func DupNS() {
	n := NS()
	if ns, ok := n.(*nspace.Tree); ok {
		NewNS(ns.Dup())
		return
	}
	if _, ok := n.(*lfs.Lfs); !ok {
		Fatal("can't DupNS() this finder: don't know how")
	}
}

// Return the prefix resolving path, the array of trees involved
// (one if it's not a union), and the array of path names in the server of the resource.
func ResolveTree(path string) (string, []zx.RWTree, []string, error) {
	ns := NS()
	if t, ok := ns.(*lfs.Lfs); ok {
		return "/", []zx.RWTree{t}, []string{path}, nil
	}
	x := ns.(*nspace.Tree)
	return x.ResolveTree(path)
}

// Issue a find for these names ("filename,predicate")
// If no predicate is given, then "depth<1" is used.
// eg. /a -> just /a; /a, -> subtree at /a
// Errors are reported by sending an error.
// The chan error status is not nil if there's an error.
// The upath attribute in the dir entries returned mimics the paths given
// in the names.
// The rpath attribute in the dir entries provide a path relative to the one
// specified by the user.
func Dirs(names ...string) chan interface{} {
	ns := NS()
	rc := make(chan interface{})
	go func() {
		var err error
		for _, name := range names {
			if len(name) > 0 && name[0] == '#' {
				d := zx.Dir{"path": name, "name": name,
					"upath": name, "type": "c"}
				rc <- d
				continue
			}
			toks := strings.SplitN(name, ",", 2)
			if toks[0] == "" {
				toks[0] = "."
			}
			if len(toks) == 1 {
				toks = append(toks, "0")
			}
			toks[0] = path.Clean(toks[0])
			name = AbsPath(toks[0])
			dprintf("getdirs: find %s %s\n", name, toks[1])
			dc := ns.Find(name, toks[1], "/", "/", 0)
			for d := range dc {
				if d == nil {
					break
				}
				d["upath"] = d["path"]
				if toks[0] != name && zx.HasPrefix(d["path"], name) {
					u := zx.Path(toks[0], zx.Suffix(d["path"], name))
					d["upath"] = u
				}
				d["rpath"] = zx.Suffix(d["path"], name)
				if d["err"] != "" {
					if d["err"] != "pruned" {
						err = errors.New(d["err"])
						rc <- err
					}
					continue
				}
				if ok := rc <- d; !ok {
					close(dc, cerror(rc))
					return
				}
			}
			if derr := cerror(dc); derr != nil {
				err = derr
				rc <- err
			} else {
				close(dc) // in case a null was sent but no error
			}
		}
		close(rc, err)
	}()
	return rc
}

func sendioc(rc chan interface{}, n string) {
	if len(n) < 2 {
		return
	}
	nb, err := strconv.Atoi(n[1:])
	if err != nil {
		nb = -1
	}
	ioc, err := IOchan(nb)
	d := zx.Dir{"name": n, "uname": n, "path": n, "type": "c"}
	if err != nil {
		d["err"] = err.Error()
	}

	rc <- d
	if err != nil {
		rc <- err
		return
	}
	for x := range ioc {
		if b, ok := x.([]byte); ok {
			sok := rc <- b
			if !sok {
				close(ioc, cerror(rc))
				break
			}
		}
	}
	if err := cerror(ioc); err != nil {
		rc <- err
	}
}

// Like Dirs(names...) but issues a FindGet to retrieve data for all regular
// files involved.
// The result channel receives one zx.Dir per file plus a series (perhaps empty) of []byte
// for that file.
// Errors on an argument are reported by sending an error.
// The chan error status is not nil if there's an error.
func Files(names ...string) chan interface{} {
	ns := NS()
	rc := make(chan interface{})
	go func() {
		var err error
	Loop:
		for _, name := range names {
			if len(name) > 0 && name[0] == '#' {
				sendioc(rc, name)
				continue
			}
			toks := strings.SplitN(name, ",", 2)
			if toks[0] == "" {
				toks[0] = "."
			}
			if len(toks) == 1 {
				toks = append(toks, "0")
			}
			toks[0] = path.Clean(toks[0])
			name = AbsPath(toks[0])
			dprintf("getfiles: findget %s %s\n", name, toks[1])
			gc := ns.FindGet(name, toks[1], "/", "/", 0)
			for g := range gc {
				dprintf("getfiles: findget -> %v\n", g)
				d := g.Dir
				if d == nil {
					break
				}
				d["upath"] = d["path"]
				if toks[0] != name && zx.HasPrefix(d["path"], name) {
					u := zx.Path(toks[0], zx.Suffix(d["path"], name))
					d["upath"] = u
				}
				d["rpath"] = zx.Suffix(d["path"], name)
				if d["err"] != "" {
					if d["err"] != "pruned" {
						err = errors.New(d["err"])
						rc <- err
					}
					continue
				}
				if ok := rc <- g.Dir; !ok {
					close(gc, cerror(rc))
					if g.Datac != nil {
						close(g.Datac, cerror(rc))
					}
					return
				}
				if g.Datac != nil {
					for x := range g.Datac {
						if ok := rc <- x; !ok {
							close(gc, cerror(rc))
							close(g.Datac, cerror(rc))
							return
						}
					}
					if derr := cerror(g.Datac); derr != nil {
						close(gc, derr)
						close(rc, derr)
						err = derr
						rc <- derr
						continue Loop
					}
				}
			}
			if derr := cerror(gc); derr != nil {
				err = derr
				rc <- err
			} else {
				close(gc) // in case a null was sent but no error
			}
		}
		close(rc, err)
	}()
	return rc
}
