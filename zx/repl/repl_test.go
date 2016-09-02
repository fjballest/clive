package repl

import (
	"clive/cmd"
	"clive/net/auth"
	"clive/zx/zux"
	"clive/zx/rzx"
	"clive/zx"
	"clive/u"
	"clive/zx/fstest"
	"os"
	"testing"
	"bytes"
)

const (
	tdir = "/tmp/repl_test"
	rtdir = "/tmp/repl_testrzx"
	tdb = "/tmp/repl_test.db"
)

var (
	ai   = &auth.Info{Uid: u.Uid, SpeaksFor: u.Uid, Ok: true}
)

func mkdb(t *testing.T, tdir string) *DB {
	db, err := NewDB("adb", tdir)
	if err != nil {
		t.Fatalf("failed %v", err)
	}
	db.Debug = testing.Verbose()
	t.Logf("db is %s\n", db)
	err = db.Scan()
	if err != nil {
		t.Fatalf("failed %v", err)
	}
	if testing.Verbose() {
		db.DumpTo(os.Stderr)
	}
	return db
}

func mktest(t *testing.T, tdir string) (*DB, func() ) {
	cmd.UnixIO("in", "out", "err")
	cmd.Warn("testing")
	os.Args[0] = "zux.test"
	fstest.Verb = testing.Verbose()
	os.RemoveAll(tdir)
	os.Remove(tdb)
	fstest.MkTree(t, tdir)
	fn := func() {
		os.RemoveAll(tdir)
		os.RemoveAll(tdb)
	}
	db := mkdb(t, tdir)
	return db, fn
}

func TestDbScan(t *testing.T) {
	db, fn := mktest(t, tdir)
	defer fn()

	all := map[string]bool{}
	for _, f := range fstest.AllFiles {
		all[f] = true
	}
	fc := db.Files()
	for f := range fc {
		t.Logf("got %s\n", f.D.Fmt())
		if !all[f.D["path"]] {
			t.Fatalf("bad file");
		}
		delete(all, f.D["path"])
	}
	if len(all) > 0 {
		t.Fatalf("didn't get %s", all)
	}
}

func TestDBFile(t *testing.T) {
	db, fn := mktest(t, tdir)
	defer fn()
	if err := db.Save(tdb); err != nil {
		t.Fatalf("saving: %s", err)
	}
	ndb, err := LoadDB(tdb)
	if err != nil {
		t.Fatalf("reading: %s", err)
	}
	if testing.Verbose() {
		ndb.DumpTo(os.Stderr)
	}
	var b1, b2 bytes.Buffer

	db.DumpTo(&b1)
	ndb.DumpTo(&b2)
	if b1.String() != b2.String() {
		t.Fatalf("dbs do not match")
	}
}

var chg1 = []Chg {
	Chg{Type: Data, D: zx.Dir{"path": "/a/a1"}},
	Chg{Type: Meta, D: zx.Dir{"path": "/a/a2"}},
	Chg{Type: Del, D: zx.Dir{"path": "/a/b/c"}},
	Chg{Type: Add, D: zx.Dir{"path": "/a/n"}},
}

var chg2 = []Chg {
	Chg{Type: Data, D: zx.Dir{"path": "/1"}},
	Chg{Type: DirFile, D: zx.Dir{"path": "/2"}},
	Chg{Type: Del, D: zx.Dir{"path": "/a/b/c"}},
}

var chg3 = []Chg {
	Chg{Type: Data, D: zx.Dir{"path": "/1"}},
	Chg{Type: DirFile, D: zx.Dir{"path": "/2"}},
	Chg{Type: Data, D: zx.Dir{"path": "/a/a1"}},
	Chg{Type: Meta, D: zx.Dir{"path": "/a/a2"}},
	Chg{Type: Del, D: zx.Dir{"path": "/a/b/c"}},
	Chg{Type: Add, D: zx.Dir{"path": "/a/n"}},
}

func cmpChgs(t *testing.T, xc []Chg, gc []Chg) {
	for i := 0; i < len(xc) && i < len(gc); i++ {
		if xc[i].Type != gc[i].Type || xc[i].D["path"] != gc[i].D["path"] {
			t.Fatalf("bad chg %s", gc[i])
		}
	}
	if len(xc) != len(gc) {
		t.Fatalf("too many or too few changes")
	}
}

func TestCmp(t *testing.T) {
	db, fn := mktest(t, tdir)
	defer fn()
	fstest.MkChgs(t, tdir)
	ndb := mkdb(t, tdir)

	cc := ndb.ChangesFrom(db)
	chgs := []Chg{}
	for c := range cc {
		chgs = append(chgs, c)
		t.Logf("chg %s", c)
	}
	cmpChgs(t, chg1, chgs)
	fstest.MkChgs2(t, tdir)
	ndb2 := mkdb(t, tdir)
	chgs = []Chg{}
	cc = ndb2.ChangesFrom(ndb)
	for c := range cc {
		chgs = append(chgs, c)
		t.Logf("chg %s", c)
	}
	cmpChgs(t, chg2, chgs)
	t.Logf("all chgs\n")
	chgs = []Chg{}
	cc = ndb2.ChangesFrom(db)
	for c := range cc {
		chgs = append(chgs, c)
		t.Logf("chg %s", c)
	}
	cmpChgs(t, chg3, chgs)
}

func mkrtest(t *testing.T, rtdir string) (*DB, func() ) {
	cmd.UnixIO("in", "out", "err")
	cmd.Warn("testing")
	os.Args[0] = "zux.test"
	os.Mkdir(rtdir+"/p", 0755)
	fstest.Verb = testing.Verbose()
	fstest.MkTree(t, rtdir+"/p")
	os.Remove("/tmp/clive.9898")
	fs, err := zux.NewZX(rtdir)
	if err != nil {
		os.RemoveAll(rtdir)
		os.Remove("/tmp/clive.9898")
		t.Fatal(err)
	}
	srv, err := rzx.NewServer("unix!local!9898", auth.TLSserver)
	if err != nil {
		os.RemoveAll(rtdir)
		os.Remove("/tmp/clive.9898")
		t.Fatal(err)
	}
	if err := srv.Serve("main", fs); err != nil {
		os.RemoveAll(rtdir)
		os.Remove("/tmp/clive.9898")
		t.Fatal(err)
	}
	db := mkdb(t, "unix!local!9898!/p")
	fn := func() {
		db.Close()
		os.RemoveAll(rtdir)
		os.Remove("/tmp/clive.9898")
		srv.Close()
	}

	return db, fn
}

func TestRzxDbScan(t *testing.T) {
	db, fn := mkrtest(t, rtdir)
	defer fn()
	all := map[string]bool{}
	for _, f := range fstest.AllFiles {
		all[f] = true
	}
	fc := db.Files()
	for f := range fc {
		t.Logf("got %s\n", f.D.Fmt())
		if !all[f.D["path"]] {
			t.Fatalf("bad file");
		}
		delete(all, f.D["path"])
	}
	if len(all) > 0 {
		t.Fatalf("didn't get %s", all)
	}
}

func TestRzxCmp(t *testing.T) {
	db, fn := mkrtest(t, rtdir)
	defer fn()
	fstest.MkChgs(t, rtdir+"/p")
	ndb := mkdb(t, "unix!local!9898!/p")
	defer ndb.Close()
	cc := ndb.ChangesFrom(db)
	chgs := []Chg{}
	for c := range cc {
		chgs = append(chgs, c)
		t.Logf("chg %s", c)
	}
	cmpChgs(t, chg1, chgs)
	fstest.MkChgs2(t, rtdir+"/p")
	ndb2 := mkdb(t, "unix!local!9898!/p")
	defer ndb2.Close()
	chgs = []Chg{}
	cc = ndb2.ChangesFrom(ndb)
	for c := range cc {
		chgs = append(chgs, c)
		t.Logf("chg %s", c)
	}
	cmpChgs(t, chg2, chgs)
	t.Logf("all chgs\n")
	chgs = []Chg{}
	cc = ndb2.ChangesFrom(db)
	for c := range cc {
		chgs = append(chgs, c)
		t.Logf("chg %s", c)
	}
	cmpChgs(t, chg3, chgs)
}
