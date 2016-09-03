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
	os.Args[0] = "repl.test"
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

func chkFiles(t *testing.T, db *DB, files []string) {
	t.Logf("%s files:", db.Name)
	all := map[string]bool{}
	for _, f := range files {
		all[f] = true
	}
	fc := db.Files()
	for f := range fc {
		t.Logf("file %s\n", f.D.Fmt())
		if files != nil && !all[f.D["path"]] {
			t.Fatalf("bad file");
		}
		delete(all, f.D["path"])
	}
	if files != nil && len(all) > 0 {
		t.Fatalf("don't have %s", all)
	}
	t.Logf("%s files ok", db.Name)
}

func TestDbScan(t *testing.T) {
	db, fn := mktest(t, tdir)
	defer fn()
	chkFiles(t, db, fstest.AllFiles)
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
		if xc[i].At != Nowhere {
			if xc[i].At != gc[i].At {
				t.Fatalf("bad chg %s", gc[i])
			}
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
	os.Args[0] = "repl.test"
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
	chkFiles(t, db, fstest.AllFiles)
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

var schg = []Chg {
	Chg{Type: Data, D: zx.Dir{"path": "/1"}, At: Remote},
	Chg{Type: DirFile, D: zx.Dir{"path": "/2"}, At: Remote},
	Chg{Type: Data, D: zx.Dir{"path": "/a/a1"}, At: Local},
	Chg{Type: Meta, D: zx.Dir{"path": "/a/a2"}, At: Local},
	Chg{Type: Del, D: zx.Dir{"path": "/a/b/c"}, At: Local},
	Chg{Type: Add, D: zx.Dir{"path": "/a/n"}, At: Local},
}


func TestTreeChanges(t *testing.T) {
	db, fn := mkrtest(t, rtdir)
	defer fn()
	db.Close()
	db, fn2 := mktest(t, tdir)
	defer fn2()
	db.Close()

	tr, err := New("adb", tdir, "unix!local!9898!/p")
	if err != nil {
		t.Fatalf("tree %s", err)
	}
	tr.Debug = testing.Verbose()
	defer tr.Close()
	fstest.MkChgs(t, tdir)
	fstest.MkChgs2(t, rtdir+"/p")
	t.Logf("pull")
	cc, err := tr.PullChanges()
	if err != nil {
		t.Fatalf("pull %s", err)
	}
	for c := range cc {
		t.Logf("chg %s %s", c.At, c)
	}
	t.Logf("push")
	cc, err = tr.PushChanges()
	if err != nil {
		t.Fatalf("push %s", err)
	}
	for c := range cc {
		t.Logf("chg %s %s", c.At, c)
	}
	t.Logf("sync")
	cc, err = tr.Changes()
	if err != nil {
		t.Fatalf("sync %s", err)
	}
	chgs := []Chg{}
	for c := range cc {
		chgs = append(chgs, c)
		t.Logf("chg %s %s", c.At, c)
	}
	cmpChgs(t, schg, chgs)
}

func TestTreePull(t *testing.T) {
	db, fn := mkrtest(t, rtdir)
	defer fn()
	db.Close()
	db, fn2 := mktest(t, tdir)
	defer fn2()
	db.Close()

	tr, err := New("adb", tdir, "unix!local!9898!/p")
	if err != nil {
		t.Fatalf("tree %s", err)
	}
	tr.Debug = testing.Verbose()
	defer tr.Close()
	fstest.MkChgs(t, tdir)
	fstest.MkChgs2(t, rtdir+"/p")
	chkFiles(t, tr.Ldb, nil)
	t.Logf("pull")
	err = tr.Pull()
	if err != nil {
		t.Fatalf("pull %s", err)
	}
	chkFiles(t, tr.Ldb, nil)
	os.RemoveAll(tdir+".pull")
	os.Rename(tdir, tdir+".pull")
}

func TestTreePush(t *testing.T) {
	db, fn := mkrtest(t, rtdir)
	defer fn()
	db.Close()
	db, fn2 := mktest(t, tdir)
	defer fn2()
	db.Close()

	tr, err := New("adb", tdir, "unix!local!9898!/p")
	if err != nil {
		t.Fatalf("tree %s", err)
	}
	tr.Debug = testing.Verbose()
	tr.Rdb.Debug = testing.Verbose()
	defer tr.Close()
	fstest.MkChgs(t, tdir)
	fstest.MkChgs2(t, rtdir+"/p")
	chkFiles(t, tr.Rdb, nil)
	t.Logf("push")
	err = tr.Push()
	if err != nil {
		t.Fatalf("pull %s", err)
	}
	chkFiles(t, tr.Rdb, nil)
	os.RemoveAll(rtdir+".push")
	os.Rename(rtdir+"/p", rtdir+".push")
}

func TestTreeSync(t *testing.T) {
	db, fn := mkrtest(t, rtdir)
	defer fn()
	db.Close()
	db, fn2 := mktest(t, tdir)
	defer fn2()
	db.Close()

	tr, err := New("adb", tdir, "unix!local!9898!/p")
	if err != nil {
		t.Fatalf("tree %s", err)
	}
	tr.Debug = testing.Verbose()
	tr.Rdb.Debug = testing.Verbose()
	defer tr.Close()
	fstest.MkChgs(t, tdir)
	fstest.MkChgs2(t, rtdir+"/p")
	chkFiles(t, tr.Rdb, nil)
	t.Logf("sync")
	err = tr.Sync()
	if err != nil {
		t.Fatalf("sync %s", err)
	}
	chkFiles(t, tr.Rdb, nil)
	os.RemoveAll(tdir+".pull")
	os.Rename(tdir, tdir+".pull")
	os.RemoveAll(rtdir+".push")
	os.Rename(rtdir+"/p", rtdir+".push")
}
