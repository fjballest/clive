/*
	A caching zx fs built upon two zx trees.

	One is used as a cache of the other.
	The cache is write-behind.
	No cached content is ever evicted.
	It is wise not to use cfs to cache a dump file tree.
*/
package cfs

import (
	"clive/dbg"
	"clive/net/auth"
	"clive/zx"
	"clive/zx/cfs/cache"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// Arguments to New
const (
	RO = true
	RW = false
)

// RO, RW are also used to acquire r and w locks in the lock functions below.
// There they refer to the r or w nature of the locks, not to what's going to be done
// to the files.

// Users
type cUsers struct {
	who map[string]time.Time
	sync.Mutex
}

// A caching file system. Uses one tree to cache another.
type Cfs struct {
	Tag           string
	lfs, rfs, cfs zx.RWTree // cfs is lfs with no ai, to wstat uids from rfs

	*zx.Flags

	rdonly, noinvalproto bool
	ai                   *auth.Info
	ci                   *zx.ClientInfo
	*cUsers
	tpath string

	lktrz *lockTrzs
	cache *cache.Info
	*invalQ
	closedc, polldonec chan bool
}

// TODO:
// Add a Redial method to zx/rfs.Rfs and try to redial rfs if there's an error.
// Puts in cache/sync are almost idempotent (but for directory creation and we can ignore
// already exists, and for removal, and we can ignore does not exist).
// The Get() in needData is the most complex one, but we might loop that part of needData
// and redial if it's needed.
// There are zx.Stat() calls that we could wrap to redial and retry.
// The alternative is to let the user redial by hand, as we do now.

var (
	// Timeout in cache before refreshing
	CacheTout = 5 * time.Second

	// polling interval when we don't have /Chg in the cached fs.
	// i.e., to detect external changes made to the server not going through cfs
	PollIval = time.Minute

	// Timeout while posting invalidations to clients
	IvalTout = 2 * time.Second

	// default debug values for new trees
	Debug bool

	// if this is true, new trees trace lock deadlocks.
	// This slows down cfs a bit.
	DebugLocks bool

	// set to disable the inval protocol
	// used only during cfs setup time.
	NoInvalProto bool
)

// Nop. But implemented to make cfs implement the Finder interface.
func (fs *Cfs) Fsys(name string) <-chan error {
	c := make(chan error, 1)
	close(c)
	return c
}

func (fs *Cfs) Stats() *zx.IOstats {
	return fs.IOstats
}

func (fs *Cfs) String() string {
	return fs.Tag
}

func (fs *Cfs) Name() string {
	return fs.Tag
}

func (f *cFile) String() string {
	return f.d["path"]
}

func (f *cFile) dprintf(ts string, args ...interface{}) {
	ts = f.d["path"] + ": " + ts
	f.t.dprintf(ts, args...)
}

func (fs *Cfs) Sync() {
	fs.cache.Sync(nil)
}

func chkok(fs zx.Tree, attr string) error {
	d, err := zx.Stat(fs, "/")
	if err != nil {
		return fmt.Errorf("%s: %s", fs.Name(), err)
	}
	rwfs, ok := fs.(zx.RWTree)
	if !ok {
		return nil
	}
	if err := <-rwfs.Wstat("/", zx.Dir{attr: "666"}); err != nil {
		return err
	}
	d, err = zx.Stat(fs, "/")
	if err != nil {
		return fmt.Errorf("%s: %s", fs.Name(), err)
	}
	if d[attr] != "666" {
		return fmt.Errorf("%s: does not preserve %s", fs.Name(), attr)
	}
	if err := <-rwfs.Wstat("/", zx.Dir{attr: ""}); err != nil {
		return err
	}
	return nil
}

// Create a new cfs given a local tree used as a cache of a remote one.
// Operations are performed on behalf of each file owner.
// The lfs tree must have permission checking disabled
// (cfs will want to update everything in it no matter the user who makes the requests)
func New(tag string, lfs zx.RWTree, rfs zx.Tree, rdonly bool) (*Cfs, error) {
	if lfs == nil || rfs == nil {
		return nil, errors.New("no lfs or rfs")
	}
	rwrfs, ok := rfs.(zx.RWTree)
	if !ok && !rdonly {
		rdonly = true
		rwrfs = zx.ROTreeFor(rfs)
		dbg.Warn("remote %T not rw: rdonly set", rfs)
	}
	if !rdonly {
		if err := chkok(lfs, "Rtime"); err != nil {
			return nil, err
		}
	}
	if tag == "" {
		tag = "cfs!" + rfs.Name()
	}
	fs := &Cfs{
		Tag:          tag,
		lfs:          lfs,
		cfs:          lfs, // but will keep its ai as nil to wstat uids
		rfs:          rwrfs,
		rdonly:       rdonly,
		noinvalproto: NoInvalProto,
		Flags:        &zx.Flags{Dbg: Debug},
		cUsers: &cUsers{
			who: make(map[string]time.Time),
		},
		cache:     cache.New(),
		closedc:   make(chan bool),
		polldonec: make(chan bool),
	}
	if DebugLocks {
		fs.lktrz = &lockTrzs{}
	}
	fs.cache.Tag = tag
	fs.cache.Dbg = &fs.Flags.Dbg
	fs.tpath = fmt.Sprintf("cfs%p", fs)
	if fs.noinvalproto {
		fs.invalQ, _ = newInvalQ(tag, &fs.Flags.Dbg, nil)
		close(fs.polldonec)
	} else {
		var mustpoll bool
		fs.invalQ, mustpoll = newInvalQ(tag, &fs.Flags.Dbg, fs.rfs)
		if mustpoll {
			dbg.Warn("%s: polling %s for external changes", tag, fs.rfs)
			// runs with no ai
			go fs.pollproc()
		} else {
			close(fs.polldonec)
		}
	}
	go fs.invalproc()
	fs.Flags.Add("verbsync", &fs.cache.Verb)
	fs.Flags.Add("debug", &fs.Flags.Dbg)
	if d, ok := lfs.(zx.Debugger); ok {
		fs.Flags.Add("ldebug", d.Debug())
	}
	if d, ok := rfs.(zx.Debugger); ok {
		fs.Flags.Add("rdebug", d.Debug())
	}
	fs.Flags.AddRO("rdonly", &fs.rdonly)
	fs.Flags.AddRO("noperm", &fs.NoPermCheck)
	fs.Flags.Add("clear", func(...string) error {
		fs.IOstats.Clear()
		return nil
	})
	return fs, nil
}

func (fs *Cfs) dprintf(fstr string, args ...interface{}) {
	if fs != nil && fs.Flags.Dbg {
		fmt.Fprintf(os.Stderr, fs.Tag+": "+fstr, args...)
	}
}

func (fs *Cfs) Close(e error) {
	fs.dprintf("close sts %v\n", e)
	close(fs.closedc)
	<-fs.polldonec
	fs.dprintf("poll closed\n")
	fs.CloseInvals()
	fs.dprintf("invals closed\n")
	fs.cache.Close(e)
	fs.dprintf("cache closed\n")
	fs.lfs.Close(e)
	fs.rfs.Close(e)
}

// Implement the rfs LogInOutTree interface.
func (u *cUsers) LogIn(who string) {
	u.Lock()
	u.who[who] = time.Now()
	u.Unlock()
}

// Implement the rfs LogInOutTree interface.
func (u *cUsers) LogOut(who string) {
	u.Lock()
	delete(u.who, who)
	u.Unlock()
}

func (u *cUsers) Users() []string {
	u.Lock()
	defer u.Unlock()
	us := []string{}
	for k, v := range u.who {
		u := fmt.Sprintf("%s %s", k, v.Format(time.RFC822))
		us = append(us, u)
	}
	return us
}

// Return a Cfs sharing everything with cfs, but performing its requests
// on behalf of the given auth info. Implements the zx.AuthTree interface.
func (fs *Cfs) AuthFor(ai *auth.Info) (zx.Tree, error) {
	ncfs := &Cfs{}
	*ncfs = *fs
	ncfs.ai = ai
	if ai != nil {
		fs.dprintf("auth for %s\n", ai.Uid)
	}
	// fs.cfs keeps ai as nil
	if afs, ok := fs.lfs.(zx.AuthTree); ok {
		aifs, err := afs.AuthFor(ai)
		if err != nil {
			if ai != nil {
				fs.dprintf("auth failed for %s\n", ai.Uid)
			}
			return nil, err
		}
		ncfs.lfs = aifs.(zx.RWTree)
	}
	if afs, ok := fs.rfs.(zx.AuthTree); ok {
		arfs, err := afs.AuthFor(ai)
		if err != nil {
			if ai != nil {
				fs.dprintf("auth failed for %s\n", ai.Uid)
			}
			return nil, err
		}
		ncfs.rfs = arfs.(zx.RWTree)
	}
	return ncfs, nil
}

// Return a Cfs sharing everything with cfs, but performing its requests
// on behalf of the given client info. Implements the zx/rfs.ServerTree interface.
func (fs *Cfs) ServerFor(ci *zx.ClientInfo) (zx.Tree, error) {
	ncfs := &Cfs{}
	*ncfs = *fs
	if ci != nil {
		uid := "none"
		if ci.Ai != nil {
			uid = ci.Ai.Uid
			ncfs.ai = ci.Ai
		}
		fs.dprintf("serve for %s %d %s\n", ci.Tag, ci.Id, uid)
	}
	ncfs.ci = ci
	if ci.Ai != nil {
		return ncfs.AuthFor(ci.Ai)
	}
	return ncfs, nil
}
