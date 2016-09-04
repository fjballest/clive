package repl

import (
	"clive/zx"
	"clive/dbg"
)

// A replicated tree
struct Tree {
	Ldb, Rdb *DB
	*dbg.Flag
	lpath, rpath string
	excl []string
}

func newDbs(scan bool, name, path, rpath string, excl ...string) (db *DB, rdb *DB, err error) {
	if scan {
		db, err = ScanNewDB(name, path, excl...)
	} else {
		db, err = NewDB(name, path, excl...)
	}
	if err != nil {
		return nil, nil, err
	}
	if scan {
		rdb, err = ScanNewDB(name, rpath, excl...)
	} else {
		rdb, err = NewDB(name, rpath, excl...)
	}
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return db, rdb, nil
}

// Create a new replicated tree with the given name and replica
// paths.
// Both replicas are assumed to be already synced, so that only
// new changes made will be propagated.
// If that's not the case, you can always use Tree.PullAll or PushAll
// to make one synced wrt the other before further pulls/pushes/syncs.
// If a path contains '!', it's assumed to be a remote tree address
// and the db operates on a remote ZX fs
// In this case, the last component of the address must be a path
func New(name, path, rpath string, excl ...string) (*Tree, error) {
	db, rdb, err := newDbs(true, name, path, rpath, excl...)
	if err != nil {
		return nil, err
	}
	return mkTree(db, rdb), nil
}

func mkTree(ldb, rdb *DB) *Tree {
	t := &Tree {
		Ldb: ldb,
		Rdb: rdb,
		lpath: ldb.Addr,
		rpath: rdb.Addr,
		excl: ldb.Excl,
		Flag: &ldb.Flag,
	}
	return t
}


func (t *Tree) Close() error {
	err := t.Ldb.Close()
	if err2 := t.Rdb.Close(); err == nil {
		err = err2
	}
	return err	
}

// Report remote changes that must be applied to sync
func (t *Tree) mustChange(path string, old *DB, w Where) (<-chan Chg, error) {
	db, err := ScanNewDB(old.Name, path, t.excl...)
	if err != nil {
		return nil, err
	}
	return db.changesFrom(old, w), nil
}

// Report remote changes that must be applied locally to sync
func (t *Tree) PullChanges() (<-chan Chg, error) {
	return t.mustChange(t.rpath, t.Rdb, Remote)
}

// Report local changes that must be applied to the remote to sync
func (t *Tree) PushChanges() (<-chan Chg, error) {
	return t.mustChange(t.lpath, t.Ldb, Local)
}

// Report all replica differences as changes that may be pulled
func (t *Tree) AllPullChanges()  (<-chan Chg, error) {
	ldb, rdb, err := newDbs(true, t.Ldb.Name, t.lpath, t.rpath, t.excl...)
	if err != nil {
		return nil, err
	}
	return rdb.changesFrom(ldb, Remote), nil
}

// Report all replica differences as changes that may be pushed
func (t *Tree) AllPushChanges()  (<-chan Chg, error) {
	ldb, rdb, err := newDbs(true, t.Ldb.Name, t.lpath, t.rpath, t.excl...)
	if err != nil {
		return nil, err
	}
	return ldb.changesFrom(rdb, Local), nil
}

// Report pull and push changes that must be made to sync.
// If there's a conflict, the latest change wins.
func (t *Tree) Changes() (<-chan Chg, error) {
	pullc, err := t.PullChanges()
	if err != nil {
		return nil, err
	}
	pushc, err := t.PushChanges()
	if err != nil {
		close(pullc, "can't push")
		return nil, err
	}
	// Changes are reported in order, so we must
	// go one by one merging them and deciding what to
	// pull, push, or ignore
	mergec := make(chan Chg)
	syncc := make(chan Chg)
	go t.merge(pullc, pushc, mergec)
	go t.resolve(mergec, syncc)
	return syncc, nil
}

func fwd(c <-chan Chg, into chan<- Chg) {
	for x := range c {
		ok := into <- x
		if !ok {
			close(c, cerror(into))
		}
	}
	close(into, cerror(c))
}

// merge changes according to names
func (t *Tree) merge(pullc, pushc <-chan Chg, syncc chan<- Chg) {
	var c1, c2 Chg
	for {
		if c1.Type == zx.None {
			c, ok := <-pullc
			if !ok {
				if c2.Type != zx.None {
					syncc <- c2
				}
				fwd(pushc, syncc)
				break
			}
			c1 = c
		}
		if c2.Type == zx.None {
			c, ok := <-pushc
			if !ok {
				if c1.Type != zx.None {
					syncc <- c1
				}
				fwd(pullc, syncc)
				break
			}
			c2 = c
		}
		cmp := zx.PathCmp(c1.D["path"], c2.D["path"])
		if cmp <= 0 {
			ok := syncc <- c1
			if !ok {
				close(pullc, cerror(syncc))
				close(pullc, cerror(syncc))
			}
			c1 = Chg{}
		}
		if cmp >= 0 {
			ok := syncc <- c2
			if !ok {
				close(pushc, cerror(syncc))
				close(pullc, cerror(syncc))
			}
			c2 = Chg{}
		}
	}
	close(syncc)
}

// resolve a merged change stream.
// if a prefix is removed or added this takes precedence over peer changes
// if the same path is changed in both sites, the later change wins.
func (t *Tree) resolve(mc <-chan Chg, rc chan<- Chg) {
	var last Chg
	for c := range mc {
		if last.Type == zx.None {
			last = c
			continue
		}
		if last.D["path"] == c.D["path"] {
			if c.Time.Before(last.Time) {
				t.Dprintf("discard on conflict %s\n", c)
				continue
			}
			t.Dprintf("discard on conflict %s\n", last)
			last = c
			continue
		}
		switch last.Type {
		case zx.Add, zx.Del, zx.DirFile:
			if zx.HasPrefix(c.D["path"], last.D["path"]) {
				t.Dprintf("discard suff. %s\n", c)
			}
		}
		if ok := rc <- last; !ok {
			close(mc, cerror(rc))
		}
		last = c;
	}
	if last.Type != zx.None {
		rc <- last
	}
	close(rc, cerror(mc))
}

// Pull changes and apply them, w/o paying attention to any local change made.
// If cc is not nil, report changes applied there.
// Failed changes have dir["err"] set to the error status.
func (t  *Tree) BlindPull(cc chan<- Chg) error {
	pc, err := t.PullChanges()
	if err != nil {
		close(cc, err)
		return err
	}
	return t.ApplyAll(pc, Remote, cc)
}

// Sync changes and apply just pulls.
// If cc is not nil, report changes applied there.
// Failed changes have dir["err"] set to the error status.
func (t  *Tree) Pull(cc chan<- Chg) error {
	pc, err := t.Changes()
	if err != nil {
		close(cc, err)
		return err
	}
	return t.ApplyAll(pc, Remote, cc)
}

// Pull all changes (not just new ones) to make the
// local replica become like the remote one.
// If cc is not nil, report changes applied there.
// Failed changes have dir["err"] set to the error status.
func (t *Tree) PullAll(cc chan<- Chg) error {
	pc, err := t.AllPullChanges()
	if err != nil {
		close(cc, err)
		return err
	}
	return t.ApplyAll(pc, Remote, cc)
}

// Push changes and apply them, w/o paying attention to any remote change made.
// If cc is not nil, report changes applied there.
// Failed changes have dir["err"] set to the error status
func (t  *Tree) BlindPush(cc chan<- Chg) error {
	pc, err := t.PushChanges()
	if err != nil {
		return err
	}
	return t.ApplyAll(pc, Local, cc)
}

// Sync changes and apply just pushes.
// If cc is not nil, report changes applied there.
// Failed changes have dir["err"] set to the error status
func (t  *Tree) Push(cc chan<- Chg) error {
	pc, err := t.Changes()
	if err != nil {
		return err
	}
	return t.ApplyAll(pc, Local, cc)
}

// Push all changes (not just new ones) to make the
// remote replica become like the local one.
// If cc is not nil, report changes applied there.
// Failed changes have dir["err"] set to the error status.
func (t *Tree) PushAll(cc chan<- Chg) error {
	pc, err := t.AllPushChanges()
	if err != nil {
		close(cc, err)
		return err
	}
	return t.ApplyAll(pc, Local, cc)
}


// Sync changes and apply them.
// If there's a create/remote, it wins wrt inner files changed at the peer.
// If there's a conflict, the newest change wins.
// If cc is not nil, report changes applied there.
// Failed changes have dir["err"] set to the error status
func (t  *Tree) Sync(cc chan<- Chg) error {
	pc, err := t.Changes()
	if err != nil {
		return err
	}
	return t.ApplyAll(pc, Both, cc)
}

// Load a replica configuration from the given (unix) files.
// Its DBs are dialed and the tree is ready to pull/push/sync.
// Files are named <fname>.ldb and <fname>.rdb
func Load(fname string) (*Tree, error) {
	ldb, err := LoadDB(fname+".ldb")
	if err != nil {
		return nil, err
	}
	if err = ldb.Dial(); err != nil {
		return nil, err
	}
	rdb, err := LoadDB(fname+".rdb")
	if err == nil {
		err = rdb.Dial()
	}
	if err != nil {
		ldb.Close()
		return nil, err
	}
	return mkTree(ldb, rdb), nil
}

// Save a replica configuration to the given (unix) files.
// Files are named <fname>.ldb and <fname>.rdb
func (t *Tree) Save(fname string) error {
	if err := t.Ldb.Save(fname+".ldb"); err != nil {
		return err
	}
	return t.Rdb.Save(fname+".rdb")
}
