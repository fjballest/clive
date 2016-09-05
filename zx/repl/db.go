/*
	Tools to synchronize zx file trees
*/
package repl

import (
	"clive/dbg"
	"clive/cmd"
	"clive/ch"
	"clive/zx"
	"clive/zx/zux"
	"clive/zx/rzx"
	"clive/net/auth"
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
	Addr	string // addr!path or path
	Excl	[]string // exclude exprs.
	rpath	string	// path to repl root in fs
	Fs	zx.Fs	// keeping the db files
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


func isExcl(path string, excl ...string) bool {
	if path == "/" {
		return false
	}
	for _, e := range excl {
		if zx.PathPrefixMatch(path, e) {
			return true
		}
	}
	return false
}

func (db *DB) setFs(path string) error {
	addr := path
	if strings.HasPrefix(path, "zx!") {
		path = path[3:]
	}
	if strings.ContainsRune(path, '!') {
		addr, rpath := splitaddr(addr)
		rfs, err := rzx.Dial(addr, auth.TLSclient)
		if err != nil {
			return err
		}
		db.Fs = rfs
		db.rpath = rpath
	} else {
		fs, err := zux.NewZX("/")
		if err != nil {
			return err
		}
		db.Fs = fs
		db.rpath = path
	}
	db.Addr = addr
	return nil
}

// Create a DB for the given tree with the given name.
// if path contains '!', it's assumed to be a remote tree address
// and the db operates on a remote ZX fs
// In this case, the last component of the address must be a path.
// The DB is not scanned, unlike in ScanNewDB
func NewDB(name, path string, excl ...string) (*DB, error) {
	excl = append(excl, ".Ctl", ".Chg", ".zx")
	db := &DB{
		Name: name,
		Excl: excl,
	}
	db.Tag = db.Name
	c := cmd.AppCtx()
	db.Debug = c.Debug
	if err := db.setFs(path); err != nil {
		return nil, err
	}
	d, err := zx.Stat(db.Fs, db.rpath)
	if err != nil {
		db.Close()
		return nil, err
	}
	d["path"] = "/"
	d["name"] = "/"
	db.Root = &File{D: d}
	return db, nil
}

// Like NewDB() and then Scan()
func ScanNewDB(name, path string, excl ...string) (*DB, error) {
	db, err := NewDB(name, path, excl...)
	if err != nil {
		return nil, err
	}
	if err = db.Scan(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) Close() error {
	if db.Fs == nil {
		return nil
	}
	if fs, ok := db.Fs.(io.Closer); ok {
		return fs.Close()
	}
	db.Fs = nil
	return nil
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
	s := f.D.Fmt()
	if f.D["err"] != "" {
		s += fmt.Sprintf("E<%s>", f.D["err"])
	}
	if f.D["rm"] != "" {
		s += " DEL"
	}
	return s
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

// Enumerate all files in db.
// Removed files known to be removed have Dir["rm"] set to  "y"
// and are also reported.
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
	if db == nil {
		fmt.Fprintf(w, "<nil db>\n");
		return
	}
	fmt.Fprintf(w, "db %s %s\n", db.Name, db.Addr)
	if len(db.Excl) > 0 {
		fmt.Fprintf(w, "exclude:")
		for _, e := range db.Excl {
			fmt.Fprintf(w, " '%s'", e)
		}
		fmt.Fprintf(w, "\n")
	}
	fc := db.Files()
	for f := range fc {
		fmt.Fprintf(w, "%s\n", f)
	}
	fmt.Fprintf(w, "\n")
}

// Walk to the given path
func (db *DB) Walk(elems ...string) (*File, error) {
	f := db.Root
	for _, e := range elems {
		cf, err := f.Walk1(e)
		// db.Dprintf("\t%s walk %s -> %s\n", f.D["name"], e, cf)
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
// If d has "rm" or "err" set, then the file is flagged as such and children are discarded.
func (db *DB) Add(d zx.Dir) error {
	if isExcl(d["path"], db.Excl...) {
		db.Dprintf("db add: excluded: %s\n", d.Fmt())
		return nil
	}
	d = d.Dup();
	f := &File{
		D: d,
	}
	elems := zx.Elems(d["path"])
	if db.Root == nil || d["path"] == "/" {
		if db.Root != nil {
			f.Child = db.Root.Child
		}
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
	//db.Dprintf("add %s to %s\n", name, db.lastpf)
	child := db.lastpf.Child
	for _, cf := range child {
		if cf.D["name"] == name {
			cf.D = d
			if cf.D["type"] != "d" || cf.D["err"] != "" || cf.D["rm"] != "" {
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
// Beware that this drops "removed file" entries.
// Dials the tree if necessary.
// Only the first error is reported.
func (db *DB) Scan() error {
	db.Dprintf("scan %s\n", db.Addr)
	if db.Fs == nil {
		return errors.New("no fs")
	}
	fs, ok := db.Fs.(zx.Finder)
	if !ok {
		return errors.New("can't find in fs")
	}
	ic := fs.Find(db.rpath, "", db.rpath, "/", 0)
	dc := make(chan face{})
	go func() {
		for d := range ic {
			if isExcl(d["path"], db.Excl...) {
				continue
			}
			if ok := dc <- d; !ok {
				close(ic, cerror(dc))
			}
		}
		close(dc, cerror(ic))
	}()
	return db.scan(dc)
}

// Beware that this drops "removed file" entries.
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
		if strings.HasSuffix(d["path"], "/Ctl") || 
		   strings.HasSuffix(d["path"],"/.zx") ||
		   strings.HasSuffix(d["path"],"/Chg") ||
		   isExcl(d["path"], db.Excl...) {
			continue
		}
		// db.Dprintf("scan %s\n", d)
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
	if ok := c <- []byte(db.Addr); !ok {
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
	addr, ok2 := gbytes(c)
	strs, ok3 := gbytes(c)
	if !ok1 || !ok2 || !ok3 {
		close(c, "unexpected msg");
		return nil, cerror(c)
	}
	db := &DB{
		Name: string(nm),
		Addr: string(addr),
		Excl: strings.SplitN(string(strs), "\n", -1),
	}
	db.Tag = db.Name
	ctx := cmd.AppCtx()
	db.Debug = ctx.Debug
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
		if isExcl(d["path"], db.Excl...) {
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

// Load a DB from a (unix) file.
// The DB is not dialed and Dial() must be called before making a scan.
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

// For DBs loaded from a file, dial the DB fs.
func (db *DB) Dial() error {
	db.Close()
	return db.setFs(db.Addr)
}
