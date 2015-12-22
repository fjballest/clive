package sync

import (
	"bytes"
	"clive/dbg"
	"clive/zx/fstest"
	"clive/zx/lfs"
	"fmt"
	"os"
	"strings"
	"testing"
)

type chg struct {
	Type ChgType
	Path string
}

const tdir = "/tmp/db_test"
const tdir2 = "/tmp/db_test2"

var (
	printf   = dbg.FuncPrintf(os.Stdout, testing.Verbose)
	moreverb = false
)

func chkcc(tag string, cc <-chan Chg, ccs []chg, rc chan error) {
	defer printf("%s done\n", tag)
	n := 0
	for c := range cc {
		printf("%s %s\n", tag, c)
		if ccs == nil {
			continue
		}
		if n >= len(ccs) {
			close(cc, "failed")
			rc <- fmt.Errorf("%s: unexpected %s", tag, c)
			return
		}
		cpath := c.D["path"]
		if c.Type != ccs[n].Type || cpath != ccs[n].Path {
			close(cc, "failed")
			rc <- fmt.Errorf("%s: bad change %s", tag, c)
			return
		}
		n++
	}
	if ccs != nil && n < len(ccs) {
		close(cc, "failed")
		rc <- fmt.Errorf("%s: missing %s", tag, ccs[n])
		return
	}
	if err := cerror(cc); err != nil {
		rc <- fmt.Errorf("%s: sts: %s", tag, err)
	} else {
		rc <- nil
	}
}

func TestFiles(t *testing.T) {
	fstest.ResetTime()
	fstest.MkTree(t, tdir)
	defer fstest.RmTree(t, tdir)
	fs, err := lfs.New(tdir, tdir, lfs.RW)
	if err != nil {
		t.Fatalf("new lfs: %s", err)
	}
	Debug = moreverb
	fs.Dbg = moreverb
	fs.SaveAttrs(true)
	db, err := NewDB("tdb", "", fs)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	n := 0
	for f := range db.Files() {
		printf("%s\n", f)
		n++
	}
	if n != 12 {
		t.Fatalf("got %d and not 12 entries", n)
	}
}

func TestNew(t *testing.T) {
	fstest.ResetTime()
	fstest.MkTree(t, tdir)
	defer fstest.RmTree(t, tdir)
	fs, err := lfs.New(tdir, tdir, lfs.RW)
	if err != nil {
		t.Fatalf("new lfs: %s", err)
	}
	fs.SaveAttrs(true)
	Debug = moreverb
	fs.Dbg = moreverb

	db, err := NewDB("tdb", "", fs)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	if testing.Verbose() {
		db.DumpTo(os.Stdout)
	}
	var b1 bytes.Buffer
	db.DumpTo(&b1)
	c := make(chan []byte, 1000)
	if err := db.SendTo(c); err != nil {
		t.Fatalf("recv: %s", err)
	}
	close(c)
	ndb, err := RecvDBFrom(c)
	if err != nil {
		t.Fatalf("recv: %s", err)
	}
	if testing.Verbose() {
		ndb.DumpTo(os.Stdout)
	}
	var b2 bytes.Buffer
	db.DumpTo(&b2)
	if b1.String() != b2.String() {
		t.Fatal("dbs do not match")
	}
}

func TestCmpNoChg(t *testing.T) {
	fstest.ResetTime()
	fstest.MkTree(t, tdir)
	defer fstest.RmTree(t, tdir)
	fstest.ResetTime()
	fstest.MkTree(t, tdir2)
	defer fstest.RmTree(t, tdir2)
	fs, err := lfs.New(tdir, tdir, lfs.RW)
	if err != nil {
		t.Fatalf("new lfs: %s", err)
	}
	fs2, err := lfs.New(tdir2, tdir2, lfs.RW)
	if err != nil {
		t.Fatalf("new lfs: %s", err)
	}
	Debug = moreverb
	fs.Dbg = moreverb
	fs2.Dbg = moreverb
	fs.SaveAttrs(true)
	fs2.SaveAttrs(true)
	db, err := NewDB("tdb", "", fs)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	if testing.Verbose() {
		db.DumpTo(os.Stdout)
	}
	db2, err := NewDB("tdb2", "", fs2)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	if testing.Verbose() {
		db2.DumpTo(os.Stdout)
	}

	cc := db.ChangesTo(db2)
	ec := make(chan error, 1)
	chkcc("chgs", cc, []chg{}, ec)
	if e := <-ec; e != nil {
		t.Fatal(e)
	}
}

func TestCmpChgs(t *testing.T) {
	fstest.ResetTime()
	fstest.MkTree(t, tdir)
	defer fstest.RmTree(t, tdir)
	fstest.ResetTime()
	fstest.MkTree(t, tdir2)
	defer fstest.RmTree(t, tdir2)
	fstest.MkChgs(t, tdir2)
	fs, err := lfs.New(tdir, tdir, lfs.RW)
	if err != nil {
		t.Fatalf("new lfs: %s", err)
	}
	fs2, err := lfs.New(tdir2, tdir2, lfs.RW)
	if err != nil {
		t.Fatalf("new lfs: %s", err)
	}
	Debug = moreverb
	fs.Dbg = moreverb
	fs2.Dbg = moreverb
	fs.SaveAttrs(true)
	fs2.SaveAttrs(true)

	db, err := NewDB("tdb", "", fs)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	if testing.Verbose() {
		db.DumpTo(os.Stdout)
	}
	db2, err := NewDB("tdb2", "", fs2)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	if testing.Verbose() {
		db2.DumpTo(os.Stdout)
	}
	chgs := []chg{
		chg{Type: Data, Path: "/a/a1"},
		chg{Type: Meta, Path: "/a/a2"},
		chg{Type: Del, Path: "/a/b/c"},
		chg{Type: Add, Path: "/a/n"},
	}
	ec := make(chan error)
	go chkcc("chgs", db.ChangesTo(db2), chgs, ec)
	if e := <-ec; e != nil {
		t.Fatal(e)
	}
	if testing.Verbose() {
		db2.DumpTo(os.Stdout)
	}
	ec = make(chan error)
	go chkcc("chgs", db.ChangesTo(db2), chgs, ec)
	if e := <-ec; e != nil {
		t.Fatal(e)
	}
}

func TestSync0Chgs(t *testing.T) {
	fstest.ResetTime()
	fstest.MkTree(t, tdir)
	defer fstest.RmTree(t, tdir)
	fstest.ResetTime()
	fstest.MkTree(t, tdir2)
	defer fstest.RmTree(t, tdir2)
	fstest.MkChgs(t, tdir2)
	fs, err := lfs.New(tdir, tdir, lfs.RW)
	if err != nil {
		t.Fatalf("new lfs: %s", err)
	}
	fs2, err := lfs.New(tdir2, tdir2, lfs.RW)
	if err != nil {
		t.Fatalf("new lfs: %s", err)
	}
	Debug = moreverb
	fs.Dbg = moreverb
	fs2.Dbg = moreverb
	fs.SaveAttrs(true)
	fs2.SaveAttrs(true)

	db, err := NewDB("tdb", "", fs)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	if testing.Verbose() {
		db.DumpTo(os.Stdout)
	}
	db2, err := NewDB("tdb2", "", fs2)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	if testing.Verbose() {
		db2.DumpTo(os.Stdout)
	}
	pulls := []chg{
		chg{Type: Data, Path: "/a/a1"},
		chg{Type: Meta, Path: "/a/a2"},
		chg{Type: Add, Path: "/a/n"},
	}
	pushes := []chg{
		chg{Type: Add, Path: "/a/b/c"},
	}
	pullc, pushc := Changes(db, db2)
	ec := make(chan error, 2)
	go chkcc("pull", pullc, pulls, ec)
	go chkcc("push", pushc, pushes, ec)
	if e := <-ec; e != nil {
		t.Fatal(e)
	}
	if e := <-ec; e != nil {
		t.Fatal(e)
	}
	if testing.Verbose() {
		db.DumpTo(os.Stdout)
		db2.DumpTo(os.Stdout)
	}
}

func TestUpdate(t *testing.T) {
	fstest.ResetTime()
	fstest.MkTree(t, tdir)
	defer fstest.RmTree(t, tdir)
	fstest.ResetTime()
	fstest.MkTree(t, tdir2)
	defer fstest.RmTree(t, tdir2)
	fs, err := lfs.New(tdir, tdir, lfs.RW)
	if err != nil {
		t.Fatalf("new lfs: %s", err)
	}
	fs2, err := lfs.New(tdir2, tdir2, lfs.RW)
	if err != nil {
		t.Fatalf("new lfs: %s", err)
	}
	Debug = moreverb
	fs.Dbg = moreverb
	fs2.Dbg = moreverb
	fs.SaveAttrs(true)
	fs2.SaveAttrs(true)

	// Initial dbs
	db, err := NewDB("tdb", "", fs)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	if testing.Verbose() {
		db.DumpTo(os.Stdout)
	}
	db2, err := NewDB("tdb2", "", fs2)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	if testing.Verbose() {
		db2.DumpTo(os.Stdout)
	}

	// Make changes to db2 and update its db
	fstest.MkChgs(t, tdir2)
	ndb2, err := NewDB("tndb2", "", fs2)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	cc := db2.ChangesTo(ndb2)
	ec := make(chan error, 1)
	go chkcc("upd2", cc, nil, ec)
	if e := <-ec; e != nil {
		t.Fatal(e)
	}
	if testing.Verbose() {
		ndb2.DumpTo(os.Stdout)
	}

	if err := db2.Update(fs2); err != nil {
		t.Fatalf("update: %s", err)
	}
	var b1, b2 bytes.Buffer
	ndb2.Name = db2.Name
	ndb2.DumpTo(&b1)
	db2.DumpTo(&b2)
	if b1.String() != b2.String() {
		t.Fatalf("dbs do not match")
	}
}

func TestSyncChgs(t *testing.T) {
	fstest.ResetTime()
	fstest.MkTree(t, tdir)
	defer fstest.RmTree(t, tdir)
	fstest.ResetTime()
	fstest.MkTree(t, tdir2)
	defer fstest.RmTree(t, tdir2)
	fs, err := lfs.New(tdir, tdir, lfs.RW)
	if err != nil {
		t.Fatalf("new lfs: %s", err)
	}
	fs2, err := lfs.New(tdir2, tdir2, lfs.RW)
	if err != nil {
		t.Fatalf("new lfs: %s", err)
	}
	Debug = moreverb
	fs.Dbg = moreverb
	fs2.Dbg = moreverb
	fs.SaveAttrs(true)
	fs2.SaveAttrs(true)

	// Initial dbs
	db, err := NewDB("tdb", "", fs)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	if testing.Verbose() {
		db.DumpTo(os.Stdout)
	}
	db2, err := NewDB("tdb2", "", fs2)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	if testing.Verbose() {
		db2.DumpTo(os.Stdout)
	}

	// Make changes to db and update its db
	fstest.MkChgs2(t, tdir)
	ndb, err := NewDB("tndb", "", fs)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	cc := db.ChangesTo(ndb)
	ec := make(chan error, 1)
	go chkcc("upd", cc, nil, ec)
	if e := <-ec; e != nil {
		t.Fatal(e)
	}
	db = ndb
	if testing.Verbose() {
		db.DumpTo(os.Stdout)
	}

	// Make changes to db2 and update its db
	fstest.MkChgs(t, tdir2)
	ndb2, err := NewDB("tndb2", "", fs2)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	cc = db2.ChangesTo(ndb2)
	ec = make(chan error, 1)
	go chkcc("upd2", cc, nil, ec)
	if e := <-ec; e != nil {
		t.Fatal(e)
	}
	db2 = ndb2
	if testing.Verbose() {
		db2.DumpTo(os.Stdout)
	}

	// Now sync
	pulls := []chg{
		chg{Type: Data, Path: "/a/a1"},
		chg{Type: Meta, Path: "/a/a2"},
		chg{Type: Del, Path: "/a/b/c"},
		chg{Type: Add, Path: "/a/n"},
	}
	pushes := []chg{
		chg{Type: Data, Path: "/1"},
		chg{Type: DirFile, Path: "/2"},
	}

	pullc, pushc := Changes(db, db2)
	ec = make(chan error, 2)
	go chkcc("pull", pullc, pulls, ec)
	go chkcc("push", pushc, pushes, ec)
	if e := <-ec; e != nil {
		t.Fatal(e)
	}
	if e := <-ec; e != nil {
		t.Fatal(e)
	}
	if testing.Verbose() {
		db.DumpTo(os.Stdout)
		db2.DumpTo(os.Stdout)
	}

}

func TestApplyChgs(t *testing.T) {
	fstest.ResetTime()
	fstest.MkTree(t, tdir)
	defer fstest.RmTree(t, tdir)
	fstest.ResetTime()
	fstest.MkTree(t, tdir2)
	defer fstest.RmTree(t, tdir2)
	fs, err := lfs.New(tdir, tdir, lfs.RW)
	if err != nil {
		t.Fatalf("new lfs: %s", err)
	}
	fs2, err := lfs.New(tdir2, tdir2, lfs.RW)
	if err != nil {
		t.Fatalf("new lfs: %s", err)
	}
	Debug = moreverb
	fs.Dbg = moreverb
	fs2.Dbg = moreverb
	fs.SaveAttrs(true)
	fs2.SaveAttrs(true)

	// Initial dbs
	db, err := NewDB("tdb", "", fs)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	db2, err := NewDB("tdb2", "", fs2)
	if err != nil {
		t.Fatalf("new: %s", err)
	}

	// Make changes to db and update its db
	fstest.MkChgs2(t, tdir)
	ndb, err := NewDB("tndb", "", fs)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	cc := db.ChangesTo(ndb)
	ec := make(chan error, 1)
	go chkcc("upd", cc, nil, ec)
	if e := <-ec; e != nil {
		t.Fatal(e)
	}
	db = ndb
	if testing.Verbose() {
		db.DumpTo(os.Stdout)
	}

	// Make changes to db2 and update its db
	fstest.MkChgs(t, tdir2)
	ndb2, err := NewDB("tndb2", "", fs2)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	cc = db2.ChangesTo(ndb2)
	ec = make(chan error, 1)
	go chkcc("upd2", cc, nil, ec)
	if e := <-ec; e != nil {
		t.Fatal(e)
	}
	db2 = ndb2
	if testing.Verbose() {
		db2.DumpTo(os.Stdout)
	}

	// Now apply changes
	pullc, pushc := Changes(db, db2)
	ec = make(chan error, 2)
	errsc := make(chan error)
	go func() {
		for e := range errsc {
			printf("err %s\n", e)
		}
	}()
	go func() {
		for c := range pullc {
			printf("pull %s\n", c)
			if err := c.Apply(fs, fs2, "", errsc); err != nil {
				close(pullc, err)
				ec <- err
				return
			}
		}
		ec <- nil
	}()
	go func() {
		for c := range pushc {
			printf("push %s\n", c)
			if err := c.Apply(fs2, fs, "", errsc); err != nil {
				close(pushc, err)
				ec <- err
				return
			}
		}
		ec <- nil
	}()
	if e := <-ec; e != nil {
		t.Fatal(e)
	}
	if e := <-ec; e != nil {
		t.Fatal(e)
	}
	close(errsc)
	db.Update(fs)
	db2.Update(fs2)
	if testing.Verbose() {
		db.DumpTo(os.Stdout)
		db2.DumpTo(os.Stdout)
	}
	var b, b2 bytes.Buffer
	db2.Name = db.Name
	db.DumpTo(&b)
	db2.DumpTo(&b2)
	if strings.Replace(b.String(), dbg.Usr, "none", -1) !=
		strings.Replace(b2.String(), dbg.Usr, "none", -1) {
		t.Fatal("dbs do not match")
	}
}
