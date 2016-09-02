/*
	Tools to synchronize zx file trees
*/
package repl

import (
	"clive/dbg"
	"clive/ch"
	"clive/zx"
	"clive/zx/rzx"
	"clive/net/auth"
	"clive/cmd"
	fpath "path"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// A DB for a fs tree
struct DB {
	Name     string // of the repl
	Path	string // for / of repl
	Excl	[]string // exclude exprs.
	rfs	*rzx.Fs
	dbg.Flag
	Root     *File  // root
	lastpf   *File
	lastpdir string
}

// a File in the metadata DB
struct File {
	D     zx.Dir  // for the file
	Child []*File // for directories
}

func splitaddr(addr string) (string, string) {
	n := strings.LastIndexByte(addr, '!')
	if n < 0 {
		panic("bad address")
	}
	return addr[:n], addr[n+1:]
}

// Create a DB for the given tree with the given name.
// if path contains '!', it's assumed to be a remote tree address
// and the db operates on a remote ZX fs
// In this case, the last component of the address must be a path
func NewDB(name, path string, excl ...string) (*DB, error) {
	if strings.HasPrefix(path, "zx!") {
		path = path[3:]
	}
	t := &DB{
		Name: name,
		Path: path,
		Excl: excl,
	}
	if strings.ContainsRune(path, '!') {
		addr, rpath := splitaddr(path)
		rfs, err := rzx.Dial(addr, auth.TLSclient)
		if err != nil {
			return nil, err
		}
		t.rfs = rfs
		t.Path = rpath
	}
	return t, nil
}

func (db *DB) Close() error {
	if db.rfs == nil {
		return nil
	}
	return db.rfs.Close()
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
	return f.D.DbFmt()
}

func (db *DB) String() string {
	if db == nil {
		return "<nil db>"
	}
	return db.Name
}

func (f *File) files(rc chan<- *File) error {
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

func (b byName) Len() int           { return len(b) }
func (b byName) Less(i, j int) bool { return b[i].D["name"] < b[j].D["name"] }
func (b byName) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

// add or update the entry for a  dir into db.
func (db *DB) Add(d zx.Dir) error {
	d = d.Dup();
	f := &File{
		D: d,
	}
	elems := zx.Elems(d["path"])
	if db.Root == nil {
		db.Root = f
		return nil
	}
	parent := elems[0 : len(elems)-1]
	name := elems[len(elems)-1]
	pdir := fpath.Join(parent...)
	if pdir != db.lastpdir || db.lastpf == nil {
		pf, err := db.Walk(parent...)
		if err != nil {
			db.Dprintf("add: can't walk: %s\n", err)
			return err
		}
		db.lastpdir = pdir
		db.lastpf = pf
	}
	db.Dprintf("add %s to %s\n", name, db.lastpf)
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

// Scan the underlying tree and re-build the metadata db.
// Dials the tree if necessary.
// Only the first error is reported.
func (db *DB) Scan() error {
	db.Dprintf("scan %s\n", db.Files)
	if db.rfs != nil {
		ic := db.rfs.Find(db.Path, "", "", "", 0)
		dc := make(chan face{})
		go func() {
			for d := range ic {
				if ok := dc <- d; !ok {
					close(ic, cerror(dc))
				}
			}
			close(dc, cerror(ic))
		}()
		return db.scan(dc)
	}
	return db.scan(cmd.Dirs(db.Path+","))
}

func (db *DB) scan(dc <-chan face{}) error {
	db.lastpdir = ""
	db.lastpf = nil
	db.Root = nil
	var err error
	for d := range dc {
		d, ok := d.(zx.Dir)
		if !ok {
			continue
		}
		d["path"] = zx.Suffix(d["path"], db.Path)
		if d["path"] == "/Ctl" || d["path"] == "/Chg" {
			continue
		}
		db.Dprintf("add %s\n", d)
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
func (db *DB) sendTo(c chan<- face{}) error {
	if ok := c <- []byte(db.Name); !ok {
		return cerror(c)
	}
	if ok := c <- []byte(db.Path); !ok {
		return cerror(c)
	}
	if ok := c <- []byte(strings.Join(db.Excl, "\n")); !ok {
		return cerror(c)
	}
	fc := db.Files()
	var err error
	for f := range fc {
		if f == nil || f.D == nil {
			err = errors.New("nil file sent")
			break
		}
		if ok := c <- f.D.Bytes(); !ok {
			return cerror(c)
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

func gbytes(c <- chan face{}) ([]byte, bool) {
	m, ok := <- c
	if !ok {
		return nil, false
	}
	b, ok := m.([]byte)
	return b, ok
}

// Receive a db from c assuming it was sent in the same format used by SendTo.
func recvDBFrom(c <-chan face{}) (*DB, error) {
	nm, ok1 := gbytes(c)
	path, ok2 := gbytes(c)
	strs, ok3 := gbytes(c)
	if !ok1 || !ok2 || !ok3 {
		close(c, "unexpected msg");
		return nil, cerror(c)
	}
	db := &DB{
		Name: string(nm),
		Path: string(path),
		Excl: strings.SplitN(string(strs), "\n", -1),
	}
	db.lastpdir = ""
	db.lastpf = nil
	db.Root = nil
	for m := range c {
		m, ok := m.([]byte)
		if !ok {
			return db, errors.New("unexpected msg")
		}
		if len(m) == 0 {
			return db, nil
		}
		_, d, err := zx.UnpackDir(m)
		if err != nil {
			return db, err
		}
		if d["path"] == "/Ctl" || d["path"] == "/Chg" {
			continue
		}
		db.Dprintf("add %s\n", d)
		if err := db.Add(d); err != nil {
			return db, err
		}
	}
	return db, nil
}

// Save a db to a local file
func (db *DB) Save(fname string) error {
	tname := fname + "~"
	fd, err := os.Create(tname)
	if err != nil {
		return err
	}
	dc := make(chan face{})
	go func() {
		close(dc, db.sendTo(dc))
	}()
	_, _, err = ch.WriteMsgs(fd, 1, dc)
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
	dc := make(chan face{})
	go func() {
		_, _, err := ch.ReadMsgs(fd, dc)
		close(dc, err)
	}()
	db, err := recvDBFrom(dc)
	close(dc, err)
	return db, err
}
