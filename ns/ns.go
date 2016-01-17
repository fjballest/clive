/*
	name spaces.

	A name space goes from a textual representation of paths
	and directory entries (perhaps referring to a remote mounted dir)
	to a finder interface.

	It's a prefix table where the longest prefix wins.
	There are no binds and no unions.
*/
package ns

import (
	"bytes"
	"clive/dbg"
	"clive/zx"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	fpath "path"
)

// Mount flags
type Flag int

const (
	Repl   Flag = iota // replace previous mounted dirs.
	Before             // mount before previous mounted dirs.
	After              // mount after previous mounted dirs.
)

// A binder is a name space that binds prefixes to directory entries.
// The implementor usually supports the Finder interface to navigate the resulting
// tree.
interface Binder {
	// Add an entry at the given d["path"] prefix for the given directory entry.
	// If flag is Before or After, previous contents of the path are preserved and
	// the new entry is added before or after them.
	// Despite Repl mounts, other mounts that are suffixes of the given prefix
	// remain mounted.
	Mount(d zx.Dir, flag Flag) error

	// Remove entries for the given path prefix (all if d is nil, or those matching
	// them.
	// Other mounts that are suffixes of the given prefix
	// remain mounted.
	Unmount(name string, d ...zx.Dir) error
}

struct prefix {
	ns   *NS
	name string
	mnt  []zx.Dir
}

// A clive name space tree.
// It implements both the binder and finder interfaces.
struct NS {
	dbg.Flag
	Verb bool	// verbose debug diags

	lk   sync.RWMutex
	pref []*prefix
}

// Create a new empty name space. It has a single entry for an empty
// directory mounted at "/"
func New() *NS {
	ns := &NS{
		pref: []*prefix{
			{name: "/"},
		},
	}
	ns.Tag = "ns"
	ns.pref[0].ns = ns
	return ns
}

func (ns *NS) vprintf(f string, args ...interface{}) (n int, err error) {
	if !ns.Verb {
		return 0, nil
	}
	return ns.Dprintf(f, args...)
	
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
func (ns *NS) Entries() []zx.Dir {
	ns.lk.Lock()
	defer ns.lk.Unlock()
	ents := []zx.Dir{}
	hasroot := false
	for _, p := range ns.pref {
		hasroot = hasroot || p.name == "/"
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

func (ns *NS) String() string {
	if ns == nil || ns.pref == nil {
		return "/\n"
	}
	var buf bytes.Buffer
	ents := ns.Entries()
	for _, p := range ents {
		if p["type"] != "p" {
			fmt.Fprintf(&buf, "%s\n", p)
			continue
		}
		path := p["path"]
		if a := p["addr"]; strings.HasPrefix(a, "lfs!") {
			toks := strings.Split(a, "!")
			if len(toks) == 3 && path == toks[1] && toks[2] == "/" {
				fmt.Fprintf(&buf, "%s\n", path)
			} else if len(toks) == 3 && toks[2] == "/" {
				fmt.Fprintf(&buf, "%s\t%s\n", path, toks[1])
			} else {
				fmt.Fprintf(&buf, "%s\t%s\n", path, a)
			}
		} else {
			pd := zx.Dir{
				"path": path,
				"name": fpath.Base(path),
				"mode": "0644",
				"type": "p",
				"addr": a,
			}
			if zx.EqualDirs(pd, p) {
				fmt.Fprintf(&buf, "%s\t%s\n", p["path"], p["addr"])
			} else {
				fmt.Fprintf(&buf, "%s\n", p)
			}
		}
	}
	return buf.String()
}

// If ln is "path addr" and addr is of the form
// proto ! ... ! path
// or
// /one/path
// then return a Dir for that entry
func specialForm(ln string) zx.Dir {
	if len(ln) == 0 || ln[0] != '/' {
		return nil
	}
	n := strings.IndexRune(ln, ':')
	if n > 0 {
		return nil
	}
	toks := strings.Fields(ln)
	if len(toks) > 2 {
		return nil
	}
	last := toks[len(toks)-1]
	if n = strings.IndexRune(last, '!'); n < 0 {
		last = fmt.Sprintf("lfs!%s!/", last)
	}
	return zx.Dir{
		"path": toks[0],
		"addr": last,
		"name": fpath.Base(toks[0]),
	}
}

// Recreate a name space provided its printed representation.
// It accepts the special line formats
// 	path addr
// 	path filepath
// to dial the given addr or use the given lfs filepath and mount it at path.
func Parse(s string) (*NS, error) {
	lns := strings.SplitN(s, "\n", -1)
	ns := New()
	for _, ln := range lns {
		ln = strings.TrimSpace(ln)
		if len(ln) == 0 || ln[0] == '#' {
			continue
		}
		d := specialForm(ln)
		p := d["path"]
		if d == nil {
			d, _ = zx.ParseDir(ln)
			p = d["path"]
		}
		if len(d) == 0 || p == "" {
			return nil, fmt.Errorf("bad ns entry for dir <%s>", d)
		}
		if err := ns.Mount(d, After); err != nil {
			return nil, err
		}
	}
	return ns, nil
}

// Create a copy of the ns.
func (ns *NS) Dup() *NS {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s", ns)
	x, err := Parse(b.String())
	if err != nil {
		panic("NS.Dup: didn't parse a correct dir string")
	}
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

func (ns *NS) mount(d zx.Dir, flag Flag) error {
	name := d["path"]
	ns.Dprintf("mount %s %s %s\n", name, d, flag)
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

func (ns *NS) Mount(d zx.Dir, flag Flag) error {
	fname := d["path"]
	name, err := zx.UseAbsPath(fname)
	if err != nil {
		return fmt.Errorf("mount: %s", err)
	}
	d = d.Dup()
	d["path"] = name
	d["name"] = path.Base(name)
	ns.lk.Lock()
	defer ns.lk.Unlock()
	return ns.mount(d, flag)
}

func (p *prefix) unmount(d zx.Dir) bool {
	some := false
	for i := 0; i < len(p.mnt); {
		if d == nil || p.mnt[i].Matches(d) {
			some = true
			p.mnt = append(p.mnt[:i], p.mnt[i+1:]...)
		} else {
			i++
		}
	}
	return some
}

func (ns *NS) unmount(name string, d zx.Dir) error {
	ns.Dprintf("unmount %s %s\n", name, d)
	some := false
	for i := 0; i < len(ns.pref); {
		p := ns.pref[i]
		if name == "" || name == p.name {
			if p.unmount(d) {
				some = true
			}
		}
		if len(p.mnt) == 0 && p.name != "/" {
			ns.pref = append(ns.pref[:i], ns.pref[i+1:]...)
		} else {
			i++
		}
	}

	if !some {
		return fmt.Errorf("unmount: no matching mount for '%s'", name)
	}
	return nil
}

// See the Binder.Unmount operation.
func (ns *NS) Unmount(fname string, d zx.Dir) error {
	name := ""
	var err error
	if fname != "" {
		name, err = zx.UseAbsPath(fname)
		if err != nil {
			return fmt.Errorf("unmount: %s", err)
		}
	}
	ns.lk.Lock()
	defer ns.lk.Unlock()
	return ns.unmount(name, d)
}

// Resolve a name and return the prefix path and the array of mount points for it.
// The "addr" attribute for each mount point returned is adjusted to refer to the path
// in the server for the resource resolved.
// The path must be absolute.
func (ns *NS) Resolve(name string) (pref string, mnts []zx.Dir, err error) {
	path, err := zx.UseAbsPath(name)
	if err != nil {
		return "", nil, fmt.Errorf("resolve: %s", err)
	}
	ns.Dprintf("resolve %s\n", path)
	ns.lk.RLock()
	defer ns.lk.RUnlock()
	var p *prefix
	for _, np := range ns.pref {
		if zx.HasPrefix(path, np.name) {
			ns.Dprintf("\thasprefix %s %s\n", path, np.name)
			p = np
		} else {
			ns.Dprintf("\tnoprefix %s %s\n", path, np.name)
		}
	}
	if p == nil {
		ns.Dprintf("\tno prefixes\n")
		return "", nil, fmt.Errorf("resolve: %s: %s", name, zx.ErrNotExist)
	}
	suff := zx.Suffix(path, p.name)
	mnts = make([]zx.Dir, 0, len(p.mnt))
	for _, d := range p.mnt {
		if d.IsFinder() || suff == "" || suff == "/" {
			d = d.Dup()
			if suff != "/" && suff != "" {
				if a := d["addr"]; len(a) > 0 && a[len(a)-1] == '/' {
					if suff[0] == '/' {
						d["addr"] = a+suff[1:]
					} else {
						d["addr"] += suff
					}
				} else {
					d["addr"] += suff
				}
			}
			mnts = append(mnts, d)
			ns.Dprintf("\td=%s\n", d)
		} else {
			ns.Dprintf("\tskip %s\n", d)
		}
	}
	if len(mnts) == 0 {
		ns.Dprintf("\tno prefixes left\n")
		return "", nil, fmt.Errorf("resolve: %s: %s", name, zx.ErrNotExist)
	}
	return p.name, mnts, nil
}
