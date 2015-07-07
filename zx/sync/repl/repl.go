/*
	Facade for replicated trees using the sync tools.
*/
package repl

import (
	"clive/zx/sync"
	"clive/zx/rfs"
	"clive/dbg"
	"fmt"
	"clive/zx"
	"os"
	"io/ioutil"
	"path/filepath"
	"strings"
	"errors"
	"io"
)

// A replicated tree.
// The expected ussage is:
//	1. new or load
//	2. sync or pull or push
//	3. save
type Repl {
	Name string
	Laddr, Raddr string
	Ldb *sync.DB
	Rdb *sync.DB
	lfs, rfs zx.RWTree
	DryRun bool	// Set to prevent applies and saves
	Verb bool		// Set to cause prints for each pull/push
	pred string	// predicate used to prune files in DBs

	vprintf dbg.PrintFunc
}

func (r *Repl) dial() error {
	lt, err := rfs.Import(r.Laddr)
	if err != nil {
		return  fmt.Errorf("%s: %s", r.Laddr, err)
	}
	rt, err := rfs.Import(r.Raddr)
	if err != nil {
		return  fmt.Errorf("%s: %s", r.Raddr, err)
	}
	r.lfs = lt
	r.rfs = rt
	return nil
}

// Predicate to exclude dot files from a repl.
const NoDots = sync.NoDots

// Make a new repl for the given name and local and remote tree addresses.
// Pred defaults to exclude all dot files and dirs: 'name~^\.&prune|true'
func New(name, pred, laddr, raddr string) (*Repl, error) {
	r := &Repl{
		Name: name,
		Laddr: laddr,
		Raddr: raddr,
		pred: pred,
	}
	r.vprintf = dbg.FlagPrintf(os.Stdout, &r.Verb)
	if err := r.dial(); err != nil {
		return nil, err
	}
	var err, rerr error
	r.Ldb, err = sync.NewDB(name + "[" + laddr + "]", pred, r.lfs)
	if err != nil {
		dbg.Warn("%s: %s", laddr, err)
	}
	r.Rdb, rerr = sync.NewDB(name + "[" + raddr + "]", pred, r.rfs)
	if rerr != nil {
		err = rerr
		dbg.Warn("%s: %s", raddr, rerr)
	}
	r.Ldb.Pred = r.pred
	r.Rdb.Pred = r.pred
	return r, err
}

func (r *Repl) saveCfg(file string) error {
	fname, err := filepath.Abs(file)
	if err != nil {
		return err
	}
	data := fmt.Sprintf("%s\n%s\n%s\n%s\n", r.Name, r.pred, r.Laddr, r.Raddr)
	tfname := fname + "~"
	err = ioutil.WriteFile(tfname, []byte(data), 0664)
	if err != nil {
		os.Remove(tfname)
		return err
	}
	return os.Rename(tfname, fname)
}

// Save the repl config and metadata dbs to disk.
func (r *Repl) Save(fname string) error {
	if r.DryRun {
		return errors.New("dry run")
	}
	if err := r.Ldb.Save(fname+"l.db"); err != nil {
		return err
	}
	if err := r.Rdb.Save(fname+"r.db"); err != nil {
		return err
	}
	return r.saveCfg(fname)
}

// Load the repl config and metadata dbs from disk.
func Load(fname string) (*Repl, error) {
	dat, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	cs := string(dat)
	lns := strings.Split(cs, "\n")
	if len(lns) < 4 {
		return nil, errors.New("too few lines in cfg")
	}
	r := &Repl{
		Name: lns[0],
		pred: lns[1],
		Laddr: lns[2],
		Raddr: lns[3],
	}
	r.vprintf = dbg.FlagPrintf(os.Stdout, &r.Verb)
	if err := r.dial(); err != nil {
		return nil, err
	}
	if r.Ldb, err = sync.LoadDB(fname+"l.db"); err != nil {
		return nil, err
	}
	if r.Rdb, err = sync.LoadDB(fname+"r.db"); err != nil {
		return nil, err
	}
	r.Ldb.Pred = r.pred
	r.Rdb.Pred = r.pred
	return r, nil
}

// Scan local and remote tress for updated metadata.
func (r *Repl) Update() error {
	if err := r.Ldb.Update(r.lfs); err != nil {
		return err
	}
	return r.Rdb.Update(r.rfs)
}

func errc(tag string, ec chan error, nc chan int) {
	n := 0
	for e := range ec {
		if e != nil && e.Error() != "pruned" {
			n++
			dbg.Warn("%s: %s", tag, e)
		}
	}
	nc <- n
}

// Pull changes.
// Updates the dbs on memory before pulling.
func (r *Repl) Pull() error {
	return r.sync(true, false)
}

// Push changes.
// Updates the dbs on memory both before and after pushing.
func (r *Repl) Push() error {
	return r.sync(false, true)
}

// Pull and push changes.
// Updates the dbs on memory both before and after syncing.
func (r *Repl) Sync() error {
	return r.sync(true, true)
}

func (r *Repl) sync(pulling, pushing bool) error {
	if err := r.Update(); err != nil {
		return err
	}
	pullc, pushc := sync.Changes(r.Ldb, r.Rdb)
	nc := make(chan int, 2)
	if pushing {
		pushec := make(chan error)
		go errc("push", pushec, nc)
		go func() {
			for c := range pushc {
				if c.D["err"] != "" {
					r.vprintf("ignore push %s\n", c.D["path"])
					continue
				}
				r.vprintf("push %s\n", c)
				if !r.DryRun {
					c.Apply(r.rfs, r.lfs, r.pred, pushec)
				}
			}
			close(pushec)
		}()
	} else {
		close(pushc, "not pushing")
		nc <- 0
	}
	if pulling {
		pullec := make(chan error)
		go errc("pull", pullec, nc)
		go func() {
			for c := range pullc {
				if c.D["err"] != "" {
					r.vprintf("ignore pull %s\n", c.D["path"])
					continue
				}
				r.vprintf("pull %s\n", c)
				if !r.DryRun {
					c.Apply(r.lfs, r.rfs, r.pred, pullec)
				}
			}
			close(pullec)
		}()
	} else {
		close(pullc, "not pulling")
		nc <- 0
	}
	n := <- nc
	n += <-nc
	var err error
	if !r.DryRun {
		err = r.Update()
	}
	if n == 0 {
		return err
	}
	return fmt.Errorf("%d errors", n)
}

// Debug
func (r *Repl) DumpTo(w io.Writer) {
	fmt.Fprintf(w, "%s '%s' %s %s\n", r.Name, r.pred, r.Laddr, r.Raddr)
	r.Ldb.DumpTo(w)
	r.Rdb.DumpTo(w)
}
