/*
	Tools to synchronize zx file trees
*/
package sync

import (
	"fmt"
	"io"
	"clive/zx"
	"clive/nchan"
	"os"
	"clive/dbg"
	"errors"
	"sort"
)

// A DB for a fs tree
type DB  {
	Name   string    // of the db
	Root   *File     // root
	Pred string	// find predicate.
	lastpf   *File
	lastpdir string
}

// a File in the metadata DB
type File  {
	D     zx.Dir  // for the file
	Child []*File // for directories
}

var (
	Debug, Verb bool
	dprintf = dbg.FlagPrintf(os.Stderr, &Debug)
	vprintf = dbg.FlagPrintf(os.Stderr, &Verb)
)

// predicate to exclude dot files from dbs.
const NoDots = `name~^\.&prune|true`

// Create a DB for the given tree with the given name.
func NewDB(name, pred string, fs Finder) (*DB, error) {
	t := &DB{
		Name: name,
		Pred: pred,
	}
	err := t.scan(fs)
	return t, err
}

func (f *File) Walk1(name string) (*File, error) {
	for _, c := range f.Child {
		if c.D["name"] == name {
			return c, nil
		}
	}
	return nil, fmt.Errorf("%s: %s: file not found", f.D["path"], name)
}

func (f *File) String() string {
	if f == nil || f.D == nil {
		return "<nil file>"
	}
	return DbFmt(f.D)
}

func (db *DB) String() string {
	if db == nil {
		return "<nil db>"
	}
	return db.Name
}

func (f *File) files(rc chan<- *File)  error {
	if ok := rc <- f; !ok {
		return cerror(rc)
	}
	for _, c := range f.Child {
		if err := c.files(rc); err != nil {
			return err
		}
	}
	return nil
}


// Enumerate all files in db
func (db *DB) Files() <-chan *File {
	rc := make(chan *File)
	if db == nil || db.Root == nil {
		close(rc)
	} else {
		go func() {
			close(rc, db.Root.files(rc))
		}()
	}
	return rc
}

// Debug dump
func (db *DB) DumpTo(w io.Writer) {
	fmt.Fprintf(w, "%s\n", db)
	if db == nil {
		return
	}
	fc := db.Files()
	for f := range fc {
		fmt.Fprintf(w, "%s\n", f)
	}
}

// Walk to the given path
func (db *DB) Walk(elems ...string) (*File, error) {
	f := db.Root
	for _, e := range elems {
		cf, err := f.Walk1(e)
		if err != nil {
			return nil, err
		}
		f = cf
	}
	return f, nil
}

type byName []*File

func (b byName) Len() int { return len(b) }
func (b byName) Less(i, j int) bool { return b[i].D["name"] < b[j].D["name"] }
func (b byName) Swap(i, j int) { b[i], b[j] = b[j], b[i] }

// add or update the entry for a  dir into db.
func (db *DB) Add(d zx.Dir) error {
	f := &File{
		D: d,
	}
	elems := zx.Elems(d["path"])
	if len(elems) == 0 {
		db.Root = f
		return nil
	}
	parent := elems[0 : len(elems)-1]
	name := elems[len(elems)-1]
	pdir := zx.Path(parent...)
	if pdir != db.lastpdir || db.lastpf == nil {
		pf, err := db.Walk(parent...)
		if err != nil {
			dprintf("add: can't walk: %s\n", err)
			return err
		}
		db.lastpdir = pdir
		db.lastpf = pf
	}
	vprintf("add %s to %s\n", name, db.lastpf)
	child := db.lastpf.Child
	for _, cf := range child {
		if cf.D["name"] == name {
			cf.D = d
			if cf.D["type"] != "d" {
				cf.Child = nil
			}
			return nil
		}
	}
	
	child = append(child, f)
	if n := len(child); n > 1 && child[n-1].D["name"] < child[n-2].D["name"] {
		sort.Sort(byName(child))
	}
	db.lastpf.Child = child
	return nil
}

// part of zx.Finder
type Finder interface {
	Find(path, pred string, spref, dpref string, depth0 int) <-chan zx.Dir
}

// Scan the underlying tree and re-build the metadata db.
// Dials the tree if necessary.
// Only the first error is reported.
func (db *DB) scan(fs Finder) error {
	dprintf("scan /,%s\n", db.Pred)
	dc := fs.Find("/", db.Pred, "", "", 0)
	db.lastpdir = ""
	db.lastpf = nil
	db.Root = nil
	var err error
	for d := range dc {
		if d["path"] == "/Ctl" || d["path"] == "/Chg" {
			continue
		}
		vprintf("add %s\n", d)
		if e := db.Add(d); err == nil && e != nil {
			err = nil
		}
	}
	if err != nil {
		return err
	}
	return cerror(dc)	
}

// Send the db through c.
// The channel must preserve message boundaries and is not closed by this function. 
// The db name is first sent and then one packed dir per file recorded in the db.
// An empty msg is sent to signal the end of the stream of dir entries
func (db *DB) SendTo(c chan<- []byte) error {
	if ok := c <- []byte(db.Name); !ok {
		return cerror(c)
	}
	if ok := c <- []byte(db.Pred); !ok {
		return cerror(c)
	}
	fc := db.Files()
	var err error
	for f := range fc {
		if f == nil || f.D == nil {
			err = errors.New("nil file sent")
			break
		}
		if _, err = f.D.Send(c); err != nil {
			break
		}
	}
	if err != nil {
		close(fc, err)
	} else {
		err = cerror(fc)
	}
	c <- []byte{}
	return err
}

// Receive a db from c assuming it was sent in the same format used by SendTo.
func RecvDBFrom(c <-chan []byte) (*DB, error) {
	nm, ok1 := <-c
	pred, ok2 := <-c
	if !ok1 || !ok2 {
		return nil, cerror(c)
	}
	db := &DB{
		Name: string(nm),
		Pred: string(pred),
	}
	db.lastpdir = ""
	db.lastpf = nil
	db.Root = nil
	for {
		d, err := zx.RecvDir(c)
		if err != nil || len(d) == 0 {
			return db, err
		}
		if d["path"] == "/Ctl" || d["path"] == "/Chg" {
			continue
		}
		dprintf("add %s\n", d)
		if err := db.Add(d); err != nil {
			return db, err
		}
	}
}

// Save a db to a local file
func (db *DB) Save(fname string) error {
	tname := fname + "~"
	fd, err := os.Create(tname)
	if err != nil {
		return err
	}
	dc := make(chan []byte)
	go func() {
		close(dc, db.SendTo(dc))
	}()
	_, _, err = nchan.WriteMsgsTo(fd, dc)
	fd.Close()
	close(dc, err)
	if err != nil {
		return err
	}
	return os.Rename(tname, fname)
}

func LoadDB(fname string) (*DB, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	dc := make(chan []byte)
	go func() {
		_, _, err := nchan.ReadMsgsFrom(fd, dc)
		close(dc, err)
	}()
	db, err := RecvDBFrom(dc)
	close(dc, err)
	return db, err
}
