/*
	name spaces.

	A name space goes from a textual representation of paths
	and directory entries (perhaps referring to a remote mounted dir)
	to a finder interface.

	It's a prefix table where the longest prefix wins.
	There are no binds and no unions.
*/
package nspace

import (
	"bytes"
	"clive/dbg"
	"clive/zx"
	"clive/zx/rfs"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
)

/*
	Mount flags
*/
type Flag int

const (
	Repl   Flag = iota // replace previous mounted dirs.
	Before             // mount before previous mounted dirs.
	After              // mount after previous mounted dirs.
)

/*
	A binder is a name space that binds prefixes to directory entries.
	The implementor usually supports the Finder interface to navigate the resulting
	tree.

	The interface is intented to permit streaming of requests, hence most results
	are sent through channels, including error indications.
*/
type Binder interface {
	// Add an entry at the given path prefix for the given directory entry.
	// If flag is Before or After, previous contents of the path are preserved and
	// the new entry is added before or after them.
	Mount(name string, d zx.Dir, flag Flag) <-chan error

	// Remove entries for the given path prefix (all if d is nil, or those matching
	// if it is not nil).
	Unmount(name string, d zx.Dir) <-chan error
}

type prefix  {
	ns   *Tree
	name string
	mnt  []zx.Dir
}

/*
	A clive name space tree.
	It implements both the binder and finder interfaces.
*/
type Tree  {
	Debug     bool // enable debug diagnostics.
	DebugFind bool // enable debug diagnostics for find requests (chatty).
	dprintf   dbg.PrintFunc
	dfprintf  dbg.PrintFunc

	lk   sync.RWMutex
	pref []*prefix
}

/*
	Create a new empty name space. It has a single entry for an empty
	directory mounted at "/"
*/
func New() *Tree {
	ns := &Tree{
		pref: []*prefix{
			{name: "/"},
		},
	}
	ns.pref[0].ns = ns
	ns.dprintf = dbg.FlagPrintf(os.Stderr, &ns.Debug)
	ns.dfprintf = dbg.FlagPrintf(os.Stderr, &ns.DebugFind)
	return ns
}

func (f Flag) String() string {
	switch f {
	case Before:
		return "Before"
	case After:
		return "After"
	default:
		return "Repl"
	}
}

// Return an array of mount entries for ns. The "path" attribute in them indicates
// the prefix where they are mounted.
func (ns *Tree) Entries() []zx.Dir {
	ents := []zx.Dir{}
	if !ns.Debug {
		ns.lk.Lock()
		defer ns.lk.Unlock()
	}
	hasroot := false
	for _, p := range ns.pref {
		hasroot = hasroot || p.name=="/"
		if len(p.mnt) == 0 {
			d := zx.Dir{"path": p.name, "mode": "0644", "type": "p"}
			ents = append(ents, d)
			continue
		}
		for _, d := range p.mnt {
			nd := d.Dup()
			if nd["type"] == "" {
				nd["type"] = "p"
			}
			if nd["mode"] == "" {
				nd["mode"] = "0644"
			}
			nd["path"] = p.name
			ents = append(ents, nd)
		}
	}
	if !hasroot {
		rd := zx.Dir{
			"path": "/",
			"mode": "0644",
			"type": "p",
		}
		return append([]zx.Dir{rd}, ents...)
	}
	return ents
}

func (ns *Tree) String() string {
	if ns==nil || ns.pref==nil {
		return "/\n"
	}
	var buf bytes.Buffer
	ents := ns.Entries()
	for _, p := range ents {
		delete(p, "Uid")
		delete(p, "Gid")
		delete(p, "Wuid")
		delete(p, "Sum")
		delete(p, "mtime")
		fmt.Fprintf(&buf, "%s\n", p)
	}
	return buf.String()
}

// Nop. But implemented to make ns implement the Finder interface.
func (ns *Tree) Fsys(name string) <-chan error {
	c := make(chan error, 1)
	close(c)
	return c
}

// If ln is "path addr" and addr is of the form
// net ! addr ! proto ! tree ! path
// or
// /one/path
// the dial the tree and walk to the path.
func specialForm(ln string) (string, zx.Dir) {
	if len(ln)==0 || ln[0]!='/' {
		return "", nil
	}
	toks := strings.Fields(ln)
	if len(toks)!=2 || len(toks[0])==0 || len(toks[1])==0 {
		return "", nil
	}
	p, addr := toks[0], toks[1]
	if addr[0] == '/' {
		addr = "*!*!lfs!main!" + addr
	}
	atoks := strings.SplitN(addr, "!", -1)
	if len(atoks) < 2 {
		return "", nil
	}
	t, err := rfs.Import(addr)
	if err != nil {
		dbg.Warn("ns: %s: import: %s", ln, err)
		return "", nil
	}
	path := "/"
	if len(atoks)>=5 && atoks[2]!="lfs" {
		path = atoks[4]
	}
	d, err := zx.Stat(t, path)
	if err != nil {
		dbg.Warn("ns: %s: stat: %s", ln, err)
		return "", nil
	}
	return p, d
}

/*
	Recreate a name space provided its printed representation.
	It accepts the special line formats
		path addr
		path filepath
	to dial the given addr or use the given lfs filepath and mount it at path.
*/
func Parse(s string) (*Tree, error) {
	lns := strings.SplitN(s, "\n", -1)
	ns := New()
	for _, ln := range lns {
		if len(ln)==0 || ln[0]=='#' {
			continue
		}
		p, d := specialForm(ln)
		if d == nil {
			d, _ = zx.ParseDirString(ln)
		} else {
			d["path"] = p
		}
		if len(d)==0 || d["path"]=="" {
			dbg.Warn("ns: bad entry: %s", ln)
			continue
		}
		if err := <-ns.Mount(d["path"], d, After); err != nil {
			return nil, err
		}
	}
	return ns, nil
}

/*
	Create a copy of the ns.
*/
func (ns *Tree) Dup() *Tree {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s", ns)
	x, _ := Parse(b.String())
	return x
}

func (p *prefix) mount(d zx.Dir, flag Flag) error {
	nl := []zx.Dir{d}
	switch flag {
	case Repl:
		p.mnt = nl
	case Before:
		p.mnt = append(nl, p.mnt...)
	case After:
		p.mnt = append(p.mnt, nl...)
	}
	return nil
}

type byName []*prefix

func (b byName) Less(i, j int) bool {
	return zx.PathCmp(b[i].name, b[j].name) < 0
}

func (b byName) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byName) Len() int {
	return len(b)
}

func (ns *Tree) mount(d zx.Dir, flag Flag) error {
	name := d["path"]
	ns.dprintf("mount %s %s %s\n", name, d, flag)
	defer ns.dprintf("%s\n", ns)
	for _, p := range ns.pref {
		if p.name == name {
			return p.mount(d, flag)
		}
	}
	np := &prefix{
		name: name,
		ns:   ns,
		mnt:  []zx.Dir{d},
	}
	ns.pref = append(ns.pref, np)
	sort.Sort(byName(ns.pref))
	return nil
}

// See the Binder.Mount operation.
func (ns *Tree) Mount(fname string, d zx.Dir, flag Flag) <-chan error {
	c := make(chan error, 1)
	go func() {
		name, err := zx.AbsPath(fname)
		if err==nil && d==nil {
			err = errors.New("no mounted dir")
		}
		if err != nil {
			c <- err
			close(c, err)
			return
		}
		d = d.Dup()
		delete(d, "mtime")
		delete(d, "size")
		d["path"] = name
		d["name"] = path.Base(name)
		ns.lk.Lock()
		defer ns.lk.Unlock()
		c <- ns.mount(d, flag)
		close(c, err)
	}()
	return c
}

func (p *prefix) unmount(d zx.Dir) {
	for i := 0; i < len(p.mnt); {
		if d==nil || p.mnt[i].Matches(d) {
			p.mnt = append(p.mnt[:i], p.mnt[i+1:]...)
		} else {
			i++
		}
	}
}

func (ns *Tree) unmount(name string, d zx.Dir) error {
	ns.dprintf("unmount %s %s\n", name, d)
	defer ns.dprintf("%s\n", ns)
	for i := 0; i < len(ns.pref); {
		p := ns.pref[i]
		if name=="" || name==p.name {
			p.unmount(d)
		}
		if len(p.mnt)==0 && p.name!="/" {
			ns.pref = append(ns.pref[:i], ns.pref[i+1:]...)
		} else {
			i++
		}
	}
	return nil
}

/*
	See the Binder.Unmount operation.
*/
func (ns *Tree) Unmount(fname string, d zx.Dir) <-chan error {
	c := make(chan error, 1)
	go func() {
		name := ""
		var err error
		if fname != "" {
			name, err = zx.AbsPath(fname)
			if err != nil {
				close(c, err)
				return
			}
		}
		ns.lk.Lock()
		defer ns.lk.Unlock()
		c <- ns.unmount(name, d)
		close(c, err)
	}()
	return c
}

// Resolve a name and return the prefix path, the array of mount points for it and the server paths
// for each mount point.
// The path must be absolute.
func (ns *Tree) Resolve(name string) (pref string, mnts []zx.Dir, spaths []string, err error) {
	path, err := zx.AbsPath(name)
	if err != nil {
		return "", nil, nil, err
	}
	ns.dprintf("resolve %s\n", path)
	ns.lk.RLock()
	defer ns.lk.RUnlock()
	var p *prefix
	for _, np := range ns.pref {
		if zx.HasPrefix(path, np.name) {
			ns.dprintf("\thasprefix %s %s\n", path, np.name)
			p = np
		}
	}
	if p == nil {
		ns.dprintf("\tno prefixes\n")
		return "", nil, nil, dbg.ErrNotExist
	}
	suff := zx.Suffix(path, p.name)
	mnts = make([]zx.Dir, 0, len(p.mnt))
	spaths = []string{}
	for _, d := range p.mnt {
		if isfinder(d) || suff=="" || suff=="/" {
			mnts = append(mnts, d.Dup())
			spath := zx.Path(suff, d["spath"])
			spaths = append(spaths, spath)
			ns.dprintf("\ts='%s' d=%s\n", spath, d)
		} else {
			ns.dprintf("\tskip %s\n", d)
		}
	}
	if len(mnts) == 0 {
		ns.dprintf("\tno prefixes left\n")
		return "", nil, nil, dbg.ErrNotExist
	}
	return p.name, mnts, spaths, nil
}

// Resolve a name and return the prefix, the zx.RWTrees for it and the server paths
// for each one. If the trees are read-only, null write methods are supplied
// using a zx.Partial tree.
// If there are errors, the last one is reproted and any partial results are returned.
// The path must be absolute.
func (ns *Tree) ResolveTree(name string) (pref string, ts []zx.RWTree, spaths []string, err error) {
	ppath, mnts, paths, err := ns.Resolve(name)
	if err != nil {
		return "", nil, nil, err
	}
	ts = make([]zx.RWTree, 0, len(mnts))
	spaths = make([]string, 0, len(paths))
	for i, d := range mnts {
		t, terr := zx.RWDirTree(d)
		if err != nil {
			err = terr
		} else {
			ts = append(ts, zx.RWTreeFor(t))
			spaths = append(spaths, paths[i])
		}
	}
	if len(ts) == 0 && err == nil {
		err = errors.New("no mounted tree")
	}
	return ppath, ts, spaths, err
}
