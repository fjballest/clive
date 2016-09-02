package repl

import (
	"clive/zx"
)

// A replicated tree
struct Tree {
	Ldb, Rdb *DB
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

// Report remote changes that must be applied locally to sync
func (t *Tree) mustChange(path string, old *DB) (<-chan Chg, error) {
	db, err := NewDB(old.Name, path, t.excl...)
	if err != nil {
		return nil, err
	}
	if err = db.Scan(); err != nil {
		return nil, err
	}
	return db.ChangesFrom(old), nil
}

// Report remote changes that must be applied locally to sync
func (t *Tree) MustPull() (<-chan Chg, error) {
	return t.mustChange(t.rpath, t.Rdb)
}

// Report local changes that must be applied to the remote to sync
func (t *Tree) MustPush() (<-chan Chg, error) {
	return t.mustChange(t.lpath, t.Ldb)
}

// Report pull and push changes that must be made to sync
// If there's a conflict, the latest change wins.
func (t *Tree) MustSync() (pullc <-chan Chg, pushc <-chan Chg, err error) {
	pullc, err = t.MustPull()
	if err != nil {
		return nil, nil, err
	}
	pushc, err = t.MustPush()
	if err != nil {
		close(pullc, "can't push")
		return nil, nil, err
	}
	// Changes are reported in order, so we must
	// go one by one merging them and deciding what to
	// pull, push, or ignore
	pullokc := make(chan Chg)
	pushokc := make(chan Chg)
	go t.merge(pullc, pushc, pullokc, pushokc)
	return pullokc, pushokc, err
}

func fwd(c <-chan Chg, into chan Chg) {
	for x := range c {
		ok := into <- x
		if !ok {
			close(c, cerror(into))
		}
	}
	close(into, cerror(c))
}

func (t *Tree) merge(pullc, pushc <-chan Chg, pullokc, pushokc chan Chg) {
	var c1, c2 Chg
	for {
		if c1.Type == None {
			c, ok := <-pullc
			if !ok {
				close(pullokc)
				fwd(pushc, pushokc)
				break
			}
			c1 = c
		}
		if c2.Type == None {
			c, ok := <-pushc
			if !ok {
				close(pushokc)
				fwd(pullc, pullokc)
				break
			}
			c2 = c
		}
		// Problems here when we choose:
		// 	add /a, del /a, dirfile /a
		//		must ignore peer changes under /a
		cmp := zx.PathCmp(c1.D["path"], c2.D["path"])
		switch cmp {
		case -1:
			ok := pullokc <- c1
			if !ok {
				close(pullc, cerror(pullokc))
				close(pushc, "can't pull")
			}
			c1 = Chg{}
		case 0:
			if c2.Time.Before(c1.Time) {
				ok := pullokc <- c1
				if !ok {
					close(pullc, cerror(pullokc))
					close(pushc, "can't pull")
				}
			} else {
			}
			c1 = Chg{}
			c2 = Chg{}
		case 1:
			ok := pushokc <- c2
			if !ok {
				close(pushc, cerror(pushokc))
				close(pullc, "can't push")
			}
			c2 = Chg{}
		}
	}
}
