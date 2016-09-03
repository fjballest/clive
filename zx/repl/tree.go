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

func scanDbs(name, path, rpath string, excl ...string) (db *DB, rdb *DB, err error) {
	db, err = NewDB(name, path, excl...)
	if err != nil {
		return nil, nil, err
	}
	rdb, err = NewDB(name, rpath, excl...)
	if err != nil {
		return nil, nil, err
	}
	err = db.Scan()
	if err2 := rdb.Scan(); err == nil {
		err = err2
	}
	if err != nil {
		db.Close()
		rdb.Close()
		return nil, nil, err
	}
	return db, rdb, nil
}

// Create a new replicated tree with the given name and replica
// paths.
// If a path contains '!', it's assumed to be a remote tree address
// and the db operates on a remote ZX fs
// In this case, the last component of the address must be a path
// If one of the replicas is empty, it's to be populated with the other one.
func New(name, path, rpath string, excl ...string) (*Tree, error) {
	db, rdb, err := scanDbs(name, path, rpath, excl...)
	if err != nil {
		return nil, err
	}
	t := &Tree {
		Ldb: db,
		Rdb: rdb,
		lpath: path,
		rpath: rpath,
		excl: excl,
		Flag: &db.Flag,
	}
	return t, nil
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
	db, err := NewDB(old.Name, path, t.excl...)
	if err != nil {
		return nil, err
	}
	if err = db.Scan(); err != nil {
		return nil, err
	}
	return db.changesFrom(old, w), nil
}

// Report remote changes that must be applied locally to sync
func (t *Tree) PullChanges() (<-chan Chg, error) {
	return t.mustChange(t.rpath, t.Rdb, Remote)
}

// Pull changes and apply them
func (t  *Tree) Pull() error {
	pc, err := t.PullChanges()
	if err != nil {
		return err
	}
	return ApplyAll(t.Ldb, t.Rdb, pc, Remote)
}

// Report local changes that must be applied to the remote to sync
func (t *Tree) PushChanges() (<-chan Chg, error) {
	return t.mustChange(t.lpath, t.Ldb, Local)
}

// Push changes and apply them
func (t  *Tree) Push() error {
	pc, err := t.PushChanges()
	if err != nil {
		return err
	}
	return ApplyAll(t.Ldb, t.Rdb, pc, Local)
}

// Sync changes and apply them
func (t  *Tree) Sync() error {
	pc, err := t.Changes()
	if err != nil {
		return err
	}
	return ApplyAll(t.Ldb, t.Rdb, pc, Both)
}

// Report pull and push changes that must be made to sync
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
		if c1.Type == None {
			c, ok := <-pullc
			if !ok {
				if c2.Type != None {
					syncc <- c2
				}
				fwd(pushc, syncc)
				break
			}
			c1 = c
		}
		if c2.Type == None {
			c, ok := <-pushc
			if !ok {
				if c1.Type != None {
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
		if last.Type == None {
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
		case Add, Del, DirFile:
			if zx.HasPrefix(c.D["path"], last.D["path"]) {
				t.Dprintf("discard suff. %s\n", c)
			}
		}
		if ok := rc <- last; !ok {
			close(mc, cerror(rc))
		}
		last = c;
	}
	if last.Type != None {
		rc <- last
	}
	close(rc, cerror(mc))
}
