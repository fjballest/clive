/*
	Cfs cache states and info.

	This works along with cfs to avoid races in the usage of the
	cache.

	The cache relies on these attributes:
		- d["Rtime"] is the refresh time for the cached entry.
		used to see if an entry is stale.
		- d["Sum"] MAY be set by the underlying trees for each file,
			and then and is used to see if the data changed.
			type, size, and mtime are using otherwise
		- d["Cache"] is set to "unread" if the file is not yet read.
		- The data structure implemented in this package.

	FUSE issues multiple writes per file, so using Sum means O(n^2)
	at least, and it's better to use lfs/mfs with DoSum not enabled.

	The cache states for files are:

		unread: we only have metadata
			-> clean		when data is read
			-> unreadmeta	when meta is changed
			-> data		when data is changed (creation)
			-> del		file has been removed
		clean: it's ok, perhaps stale
			-> unread		when invalidated
			-> meta		when meta is changed
			-> data		when data is changed
			-> del		file has been removed

		unreadmeta: meta pending for sync but not yet read
			-> meta		when data is read
			-> data		when data is changed
			-> unread		when meta is synced
			-> del		file has been removed

		meta: meta pending for sync
			-> data		when data is changed
			-> unreadmeta	data invalidated
			-> clean		when meta is synced
			-> del		file has been removed

		data: data pending for sync
			-> clean		when data is synced
			-> del		file has been removed

		del: file has been removed
			-> unread		created in the server
			-> new		created locally and pending for sync

		new: created; data pending for sync
			-> clean		synced
			-> none		removed (we forget about it)

	Files in unread have d["Cache"] == "unread" and are not in the map

	Files in clean do not have d["Cache"] == "unread" and are not in the map

	All other states are recorded in the map and the value of d["Cache"] is
	irrelevant for them.

	The state in the map records two values, the cache state and a Busy flag.

	The Busy flag is set to prevent the syncer from syncing the file before it's
	a good time to do it (eg., before it's fully written).

*/
package cache

/*
	When a file goes from unread to any other we set d["Cache"] instead of
	removing it from d, so that a further wstat gets rid of the unread value.
	we could also try with d["Cache"] == "" but that's harder to spot in bugs.
*/

import (
	"clive/dbg"
	"clive/work"
	"clive/zx"
	"clive/zx/cfs/locks"
	"fmt"
	"os"
	"sync"
)

type State int

const (
	CNone   State = iota
	CUnread       // kept using Dir
	CClean        // kept using Dir
	CUnreadMeta
	CMeta
	CNew
	CData
	CDel
)

type FileInfo struct {
	path             string
	state            State
	busy, wasdel     bool // don't sync by now; issue a remove before sync
	child            []*FileInfo
	lfs              zx.RWTree // local fs to sync
	rfs              zx.RWTree // rem fs to sync into
	dprintf, vprintf func(fstr string, args ...interface{})
}

type Info struct {
	lk   sync.Mutex // locks only the tree structure; entries are locked by Cfs.locks
	root *FileInfo  // not used, but for child[]
	Dbg  *bool
	Verb bool // report syncs
	Tag  string
	*locks.Set
	dirtyc, closedc chan bool

	pool *work.Pool // see syncproc() and FileInfo.sync()
}

func New() *Info {
	dflag := false
	ci := &Info{
		Set:     &locks.Set{},
		Tag:     "cache",
		Dbg:     &dflag,
		dirtyc:  make(chan bool, 1),
		closedc: make(chan bool),
		root:    &FileInfo{path: "/", state: CUnread},
	}
	ci.root.dprintf = ci.dprintf
	ci.root.vprintf = ci.vprintf
	go ci.syncproc()
	return ci
}

func (ci *Info) dprintf(fstr string, args ...interface{}) {
	if ci != nil && *ci.Dbg {
		fmt.Fprintf(os.Stderr, ci.Tag+":ci: "+fstr, args...)
	}
}

func (ci *Info) vprintf(fstr string, args ...interface{}) {
	if ci != nil {
		if ci.Verb {
			dbg.Warn(fstr, args...)
		} else if *ci.Dbg {
			fmt.Fprintf(os.Stderr, ci.Tag+":ci: "+fstr, args...)
		}
	}
}

func (st State) String() string {
	switch st {
	case CNone:
		return "none"
	case CUnread:
		return "unread"
	case CClean:
		return "clean"
	case CUnreadMeta:
		return "unreadmeta"
	case CMeta:
		return "meta"
	case CNew:
		return "new"
	case CData:
		return "data"
	case CDel:
		return "del"
	default:
		panic("unknown cache state")
	}
}

// add n to child, but move siblings that are suffixes of n into n.
func (c *FileInfo) addchild(n *FileInfo) *FileInfo {
	old := c.child
	for i := 0; i < len(old); {
		if zx.HasPrefix(old[i].path, n.path) {
			n.child = append(n.child, old[i])
			copy(old[i:], old[i+1:])
			old = old[:len(old)-1]
			continue
		}
		i++
	}
	c.child = append(old, n)
	return n
}

func (c *FileInfo) lookup(path string, mkit, rmit, rmall bool) *FileInfo {
	if c.path == path {
		return c
	}
	for i, cc := range c.child {
		if (rmall || rmit) && cc.path == path {
			cc.state = CDel
			cc.wasdel = true
			cc.busy = false
			if rmall || len(cc.child) == 0 {
				cc.child = nil
				c.child[i] = c.child[len(c.child)-1]
				c.child = c.child[:len(c.child)-1]
			}
			return cc
		}
		if zx.HasPrefix(path, cc.path) {
			return cc.lookup(path, mkit, rmit, rmall)
		}
	}
	if mkit {
		fi := &FileInfo{path: path, dprintf: c.dprintf, vprintf: c.vprintf}
		return c.addchild(fi)
	}
	return nil
}

func (ci *Info) lookup(path string, mkit, rmit, rmall bool) *FileInfo {
	if path == "" {
		return nil
	}
	if ci.root == nil {
		ci.root = &FileInfo{
			dprintf: ci.dprintf,
			vprintf: ci.vprintf,
		}
	}
	return ci.root.lookup(path, mkit, rmit, rmall)
}

func (ci *Info) Lookup(path string) *FileInfo {
	ci.lk.Lock()
	defer ci.lk.Unlock()
	return ci.lookup(path, false, false, false)
}

func (ci *Info) State(d zx.Dir) State {
	if d == nil {
		return CClean
	}
	ci.lk.Lock()
	defer ci.lk.Unlock()
	f := ci.lookup(d["path"], false, false, false)
	if f == nil {
		if d["Cache"] == "unread" {
			return CUnread
		}
		return CClean
	}
	return f.state
}

// We know our data (if any) is stale and want to discard any change
func (ci *Info) InvalData(d zx.Dir) {
	d["Cache"] = "unread"
	ci.lk.Lock()
	defer ci.lk.Unlock()
	ci.dprintf("%s: invaldata\n", d["path"])
	f := ci.lookup(d["path"], false, false, false)
	if f == nil || f.state == CUnread || f.state == CClean {
		if f != nil && len(f.child) == 0 {
			ci.lookup(f.path, false, false, true)
		}
		return
	}
	if f.state == CMeta {
		f.state = CUnreadMeta
		return
	}
	f.state = CUnread
	f.wasdel = false
	f.busy = false
	if len(f.child) == 0 {
		ci.lookup(f.path, false, false, true)
	}
}

// We know our data is clean.
func (ci *Info) Clean(d zx.Dir) {
	d["Cache"] = "read" // anything other than "unread"
	ci.lk.Lock()
	defer ci.lk.Unlock()
	ci.dprintf("%s: clean\n", d["path"])
	f := ci.lookup(d["path"], false, false, false)
	if f != nil {
		f.state = CClean
		f.wasdel = false
		f.busy = false
		if len(f.child) == 0 {
			ci.lookup(f.path, false, false, true)
		}
	}
}

func (ci *Info) DirtyMeta(lfs, rfs zx.RWTree, d zx.Dir) {
	d["Cache"] = "dirty" // anything other than "unread"
	ci.lk.Lock()
	defer ci.lk.Unlock()
	ci.dprintf("%s: dirtymeta\n", d["path"])
	f := ci.lookup(d["path"], true, false, false)
	if d["Cache"] == "unread" {
		f.state = CUnreadMeta
	} else if f.state != CData && f.state != CNew {
		f.state = CMeta
	}
	f.lfs = lfs
	f.rfs = rfs
	ci.dirty()
}

func (ci *Info) Created(lfs, rfs zx.RWTree, d zx.Dir) {
	d["Cache"] = "dirty" // anything other than "unread"
	ci.lk.Lock()
	defer ci.lk.Unlock()
	ci.dprintf("%s: created\n", d["path"])
	f := ci.lookup(d["path"], true, false, false)
	f.state = CNew
	f.lfs = lfs
	f.rfs = rfs
	ci.dirty()
}

func (ci *Info) CreatedBusy(lfs, rfs zx.RWTree, d zx.Dir) {
	d["Cache"] = "dirty" // anything other than "unread"
	ci.lk.Lock()
	defer ci.lk.Unlock()
	ci.dprintf("%s: created\n", d["path"])
	f := ci.lookup(d["path"], true, false, false)
	f.state = CNew
	f.busy = true
	f.lfs = lfs
	f.rfs = rfs
}

func (ci *Info) DirtyData(lfs, rfs zx.RWTree, d zx.Dir) {
	d["Cache"] = "dirty" // anything other than "unread"
	ci.lk.Lock()
	defer ci.lk.Unlock()
	ci.dprintf("%s: dirtydata\n", d["path"])
	f := ci.lookup(d["path"], true, false, false)
	if f.state != CNew {
		f.state = CData
	}
	f.lfs = lfs
	f.rfs = rfs
	ci.dirty()
}

func (ci *Info) DirtyDataBusy(lfs, rfs zx.RWTree, d zx.Dir) {
	d["Cache"] = "dirty" // anything other than "unread"
	ci.lk.Lock()
	defer ci.lk.Unlock()
	ci.dprintf("%s: dirtydata\n", d["path"])
	f := ci.lookup(d["path"], true, false, false)
	if f.state != CNew {
		f.state = CData
	}
	f.lfs = lfs
	f.rfs = rfs
	f.busy = true
}

func (ci *Info) NotBusy(d zx.Dir) {
	ci.lk.Lock()
	defer ci.lk.Unlock()
	f := ci.lookup(d["path"], false, false, false)
	if f != nil {
		f.busy = false
		ci.dirty()
	}
}

// File is gone from the server, discard it
func (ci *Info) Gone(d zx.Dir) {
	d["Cache"] = "gone" // anything other than "unread"
	d["rm"] = "y"
	ci.lk.Lock()
	defer ci.lk.Unlock()
	ci.dprintf("%s: gone\n", d["path"])
	ci.lookup(d["path"], false, false, true)
}

// File has been removed by the client
func (ci *Info) Removed(lfs, rfs zx.RWTree, d zx.Dir) {
	d["Cache"] = "gone" // anything other than "unread"
	d["rm"] = "y"
	ci.lk.Lock()
	defer ci.lk.Unlock()
	ci.dprintf("%s: removed\n", d["path"])
	f := ci.lookup(d["path"], true, false, false)
	if f.state == CNew {
		ci.lookup(d["path"], false, false, true)
		return
	}
	f.state = CDel
	f.wasdel = true
	f.busy = false
	f.lfs = lfs
	f.rfs = rfs
	ci.dirty()
}
