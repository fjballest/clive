package cache

import (
	"time"
	"clive/dbg"
	"clive/work"
	"clive/zx"
	"strings"
	"sync/atomic"
)

const (
	// wait for this period of inactivity to sync
	SyncDelay = 5*time.Second

	// wait at most this to start syncing
	MaxSyncDelay = 2*SyncDelay

	// max # of concurrent syncs
	Nsyncers = 5
)

func (ci *Info) dirty() {
	select {
	case ci.dirtyc <- true:
	default:
	}
}

func (ci *Info) Close(e error) {
	ci.dprintf("closing... (sts %v)\n", e)
	close(ci.dirtyc, e)
	<-ci.closedc
}

func (ci *Info) syncproc() {
	ci.dprintf("syncproc started\n")
	defer ci.dprintf("syncproc done\n")
	ci.pool = work.NewPool(Nsyncers)
	for {
		// idle
		if _, ok := <-ci.dirtyc; !ok {
			ci.Sync(nil)
			ci.pool.Wait()
			close(ci.closedc)
			return
		}
		// work to do, perhaps
		tout := time.After(MaxSyncDelay)
		doselect {
		case _, ok := <-ci.dirtyc:
			if !ok {
				ci.Sync(nil)
				close(ci.closedc)
				return
			}
			continue
		case <-time.After(SyncDelay):
			ci.dprintf("system sync\n")
			break
		case <-tout:
			ci.dprintf("system sync\n")
			break
		}
		ci.Sync(nil)
	}
}

func (c *FileInfo) checkClean() {
	if c.state != CClean && c.state != CUnread && !c.busy {
		dbg.Warn("%s: not clean and not busy: %s", c.path, c.state)
		panic("bugs")
	}
	for _, cc := range c.child {
		cc.checkClean()
	}
}

var bname = map[bool] string{true: " busy", false:""}
var dname = map[bool] string{true: " was del", false:""}

func (c *FileInfo) dump(n int) {
	tabs := strings.Repeat("    ", n)
	c.dprintf("%s%s = %s%s%s\n", tabs, c.path, c.state, bname[c.busy], dname[c.wasdel])
	for _, cc := range c.child {
		cc.dump(n+1)
	}
}

// Do a sync with everything locked and run ufn if given while it's quiescent.
// If fn is not nil, we force the sync even for busy files.
func (ci *Info) Sync(fn func()) int {
	var n int32
	ci.QuiescentRun(func() {
		var nbusy int32
		ci.lk.Lock()		// not really needed
		defer ci.lk.Unlock()
		if *ci.Dbg {
			ci.dprintf("pre sync:\n")
			ci.root.dump(0)
		}

		n, nbusy = ci.root.sync(ci.pool, fn != nil)
		ci.dprintf("sync: %d files %d busy\n", n, nbusy)

		if *ci.Dbg {
			ci.root.checkClean()
		}
		if len(ci.root.child) > 0 && nbusy == 0 {
			ci.root.child = ci.root.child[:0]
		}
		if *ci.Dbg {
			ci.dprintf("post sync:\n")
			ci.root.dump(0)
		}
		if fn != nil {
			fn()
		}
	})
	return int(n)
}

var iname = map[bool]string{true:" ignored", false:""}

// sync and return how many files we synced
func (c *FileInfo) sync(p *work.Pool, ignbusy bool) (int32, int32) {
	var n, nbusy int32
	if c.busy {
		c.dprintf("%s: sync busy%s\n", c.path, iname[ignbusy])
		nbusy++
	}
	if !c.busy || ignbusy {
		switch c.state {
		case CMeta, CUnreadMeta:
			c.vprintf("%s: sync meta\n", c.path)
			c.syncMeta()
			if !c.busy {
				n++
			}
		case CData, CNew:
			c.vprintf("%s: sync data\n", c.path)
			c.syncData()
			if !c.busy {
				n++
			}
		case CDel:
			c.vprintf("%s: sync del\n", c.path)
			c.syncDel()
			if !c.busy {
				n++
			}
		}
	}
	rc := make(chan bool, len(c.child))
	for i := range c.child {
		cc := c.child[i]
		p.Call(rc, func() {
			cn, cnbusy := cc.sync(p, ignbusy)
			atomic.AddInt32(&n,cn)
			atomic.AddInt32(&nbusy,cnbusy)
		})
	}
	for range c.child {
		<-rc
	}
	if nbusy == 0 && len(c.child) > 0 {
		c.child = c.child[:0]
	}
	return n, nbusy
}

func (c *FileInfo) invalid() {
	c.state = CUnread
	<-c.lfs.Wstat(c.path, zx.Dir{"Cache": "unread", "Rtime": "0"})
}

func (c *FileInfo) syncMeta() {
	d, err := zx.Stat(c.lfs, c.path)
	if err == nil {
		d = d.UsrAttrs()
		delete(d, "mtime")
		delete(d, "size")
		delete(d, "Rtime")
		delete(d, "Cache")
		d["Mode"] = d["mode"]
		err = <-c.rfs.Wstat(c.path, d)
	}
	if err != nil {
		c.dprintf("%s: sync meta: %s\n", c.path, err)
		c.invalid()
	} else if c.state == CUnreadMeta {
		c.state = CUnread
	} else {
		c.state = CClean
	}
	c.wasdel = false
}

func (c *FileInfo) syncData() {
	d, err := zx.Stat(c.lfs, c.path)
	if err != nil {
		c.dprintf("sync %s: %s\n", c.path, err)
		c.invalid()
		return
	}
	if c.wasdel {
		if c.path == "" || c.path == "/" {
			panic("remove all /")
		}
		<-c.rfs.RemoveAll(c.path) // and ignore errors
	}
	delete(d, "Rtime")
	delete(d, "Cache")
	d["Mode"] = d["mode"]
	if d["type"] == "d" {
		err = <-c.rfs.Mkdir(c.path, d)
	} else {
		datc := c.lfs.Get(c.path, 0, zx.All, "")
		rc := c.rfs.Put(c.path, d, 0, datc, "")
		<-rc
		err = cerror(rc)
	}
	c.state = CClean
	if err != nil {
		c.dprintf("%s: sync meta: %s\n", c.path, err)
		c.invalid()
	}
	c.wasdel = false
}

func (c *FileInfo) syncDel() {
	if c.path == "" || c.path == "/" {
		panic("sync: DEL /")
		return
	}
	// There's a potential problem if a file was created remotelly
	// since we removed a subtree locally, but it's a race anyway.
	err := <- c.rfs.RemoveAll(c.path)
	if err != nil && !dbg.IsNotExist(err) {
		c.dprintf("%s: sync del: %s\n", c.path, err)
		c.state = CUnread
	}
	c.wasdel = false
}
